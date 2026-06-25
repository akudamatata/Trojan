package core

import (
	"bufio"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	// 正则 1：匹配 IP 登录，形如：122.97.10.15:54321 authenticated as user_1
	authRegex = regexp.MustCompile(`([0-9a-fA-F\.\:\[\]]+):(\d+)\s+authenticated as\s+([a-zA-Z0-9_\-]+)`)
	// 正则 2：匹配目标连接，形如：user_1 connected to github.com:443
	connectRegex = regexp.MustCompile(`([a-zA-Z0-9_\-]+)\s+connected to\s+([a-zA-Z0-9\.\-]+):(\d+)`)
	// 正则 3：匹配 Trojan-Go 日志，形如：user 95128474c32898ab25e5e2a844b46552aa90da745ca8ce6f073956e8 from 116.115.19.244:13166 tunneling to mmbiz.qpic.cn:80
	tgRegex = regexp.MustCompile(`user\s+([0-9a-fA-F]{56})\s+from\s+([0-9a-fA-F\.\:\[\]]+):(\d+)\s+tunneling to\s+(\S+):(\d+)`)
)

// ipBuffer 和 domainBuffer 用于批量聚合写入，避免每条日志都触发一次 MySQL 连接
type domainKey struct {
	Username string
	Domain   string
}

type ActiveConnection struct {
	Username    string    `json:"username"`
	ClientIP    string    `json:"client_ip"`
	ClientPort  string    `json:"client_port"`
	TargetHost  string    `json:"target_host"`
	ConnectedAt time.Time `json:"connected_at"`
}

var (
	ipBufferMu sync.Mutex
	// ipBuffer: key = "username|ip", value = true (去重即可)
	ipBuffer = make(map[string]bool)

	domainBufferMu sync.Mutex
	// domainBuffer: key = domainKey, value = 本轮累计的访问次数
	domainBuffer = make(map[domainKey]int)

	ActiveConnsMu sync.RWMutex
	ActiveConns   = make(map[string]*ActiveConnection) // key = clientIP:clientPort
	lastAuthIP    = make(map[string]string)            // username -> clientIP:clientPort
)

// getActiveService 检测当前运行的是 trojan-go 还是 trojan
func getActiveService() string {
	err := exec.Command("systemctl", "is-active", "trojan-go").Run()
	if err == nil {
		return "trojan-go"
	}
	return "trojan"
}

// cleanDomain 清理并验证域名，过滤掉纯 IP 访问，移除 www. 前缀
func cleanDomain(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	// 去除中括号（如果是 IPv6 格式）
	host = strings.Trim(host, "[]")
	// 验证是否是纯 IP 地址
	if net.ParseIP(host) != nil {
		return ""
	}
	// 去掉 www. 前缀
	if strings.HasPrefix(host, "www.") {
		host = host[4:]
	}
	return host
}

// flushBuffers 将内存中聚合的 IP 和域名数据批量写入 MySQL，然后清空缓冲区
// 这样无论日志流量多高，MySQL 写入频率固定为每 flushInterval 一次
func flushBuffers() {
	mysql := GetMysql()

	// 1. 刷新 IP 缓冲区
	ipBufferMu.Lock()
	ipSnapshot := ipBuffer
	ipBuffer = make(map[string]bool)
	ipBufferMu.Unlock()

	for key := range ipSnapshot {
		parts := strings.SplitN(key, "|", 2)
		if len(parts) == 2 {
			if err := mysql.SaveUserIP(parts[0], parts[1]); err != nil {
				fmt.Printf("[Daemon] Error flushing IP %s: %v\n", key, err)
			}
		}
	}

	// 2. 刷新域名缓冲区
	domainBufferMu.Lock()
	domainSnapshot := domainBuffer
	domainBuffer = make(map[domainKey]int)
	domainBufferMu.Unlock()

	for dk, count := range domainSnapshot {
		if err := mysql.SaveUserDomainBatch(dk.Username, dk.Domain, count); err != nil {
			fmt.Printf("[Daemon] Error flushing domain %s->%s: %v\n", dk.Username, dk.Domain, err)
		}
	}

	if len(ipSnapshot) > 0 || len(domainSnapshot) > 0 {
		fmt.Printf("[Daemon] Flushed %d IPs, %d domains to DB\n", len(ipSnapshot), len(domainSnapshot))
	}
}

// StartDaemon 启动日志监听守护进程
func StartDaemon() {
	service := getActiveService()
	fmt.Printf("[Daemon] Starting log parser for service: %s\n", service)

	// 恢复已有的黑名单规则到防火墙，并启动定时清理过期黑名单
	SyncBlacklist()
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			CleanExpiredBlacklist()
		}
	}()

	// 启动定时刷新协程：每 30 秒将内存中的聚合数据批量写入 MySQL
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			flushBuffers()
		}
	}()

	// 启动日志读取协程
	go func() {
		for {
			cmd := exec.Command("journalctl", "-f", "-u", service, "-o", "cat")
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				fmt.Printf("[Daemon] Error creating stdout pipe: %v. Retrying in 5s...\n", err)
				time.Sleep(5 * time.Second)
				continue
			}

			if err := cmd.Start(); err != nil {
				fmt.Printf("[Daemon] Error starting journalctl: %v. Retrying in 5s...\n", err)
				time.Sleep(5 * time.Second)
				continue
			}

			scanner := bufio.NewScanner(stdout)

			for scanner.Scan() {
				line := scanner.Text()

				// 1. 匹配 IP 连入 → 写入内存缓冲区（去重）
				if authMatches := authRegex.FindStringSubmatch(line); len(authMatches) > 3 {
					ip := strings.Trim(authMatches[1], "[]")
					port := authMatches[2]
					username := authMatches[3]

					if net.ParseIP(ip) != nil {
						ipBufferMu.Lock()
						ipBuffer[username+"|"+ip] = true
						ipBufferMu.Unlock()

						ActiveConnsMu.Lock()
						lastAuthIP[username] = ip + ":" + port
						ActiveConnsMu.Unlock()
					}
				}

				// 2. 匹配访问的目标网站/域名 → 写入内存缓冲区（聚合计数）
				if connMatches := connectRegex.FindStringSubmatch(line); len(connMatches) > 3 {
					username := connMatches[1]
					targetHost := connMatches[2]
					targetPort := connMatches[3]

					domain := cleanDomain(targetHost)
					if domain != "" {
						domainBufferMu.Lock()
						domainBuffer[domainKey{Username: username, Domain: domain}]++
						domainBufferMu.Unlock()
					}

					ActiveConnsMu.Lock()
					if ipPort, ok := lastAuthIP[username]; ok {
						parts := strings.SplitN(ipPort, ":", 2)
						if len(parts) == 2 {
							ActiveConns[ipPort] = &ActiveConnection{
								Username:    username,
								ClientIP:    parts[0],
								ClientPort:  parts[1],
								TargetHost:  targetHost + ":" + targetPort,
								ConnectedAt: time.Now(),
							}
						}
					}
					ActiveConnsMu.Unlock()
				}

				// 3. 匹配 Trojan-Go 日志连入与访问目标 → 写入内存缓冲区
				if tgMatches := tgRegex.FindStringSubmatch(line); len(tgMatches) > 5 {
					hash := tgMatches[1]
					ip := strings.Trim(tgMatches[2], "[]")
					port := tgMatches[3]
					targetHost := tgMatches[4]
					targetPort := tgMatches[5]

					username := getUsernameByHash(hash)
					if username != "" {
						if net.ParseIP(ip) != nil {
							ipBufferMu.Lock()
							ipBuffer[username+"|"+ip] = true
							ipBufferMu.Unlock()
						}

						domain := cleanDomain(targetHost)
						if domain != "" {
							domainBufferMu.Lock()
							domainBuffer[domainKey{Username: username, Domain: domain}]++
							domainBufferMu.Unlock()
						}

						ActiveConnsMu.Lock()
						ipPort := ip + ":" + port
						ActiveConns[ipPort] = &ActiveConnection{
							Username:    username,
							ClientIP:    ip,
							ClientPort:  port,
							TargetHost:  targetHost + ":" + targetPort,
							ConnectedAt: time.Now(),
						}
						ActiveConnsMu.Unlock()
					}
				}
			}

			// 如果读取中断，刷新剩余缓冲后等待 5 秒重新连接日志流
			flushBuffers()
			cmd.Wait()
			fmt.Println("[Daemon] Journalctl stream closed. Reconnecting in 5s...")
			time.Sleep(5 * time.Second)
		}
	}()
}

var (
	userCacheMu sync.RWMutex
	userCache   = make(map[string]string) // hash -> username
)

// getUsernameByHash 根据密码 SHA224 密文哈希反查用户名（使用内存缓存防止频繁查询数据库）
func getUsernameByHash(hash string) string {
	userCacheMu.RLock()
	username, exists := userCache[hash]
	userCacheMu.RUnlock()
	if exists {
		return username
	}

	mysql := GetMysql()
	db := mysql.GetDB()
	if db == nil {
		return ""
	}
	defer db.Close()

	err := db.QueryRow("SELECT username FROM users WHERE password = ?", hash).Scan(&username)
	if err != nil {
		// 缓存空白结果，防止频繁的无效查询（缓存击穿保护）
		userCacheMu.Lock()
		userCache[hash] = ""
		userCacheMu.Unlock()
		return ""
	}

	userCacheMu.Lock()
	userCache[hash] = username
	userCacheMu.Unlock()
	return username
}
