package core

import (
	"bufio"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strings"
)

var (
	// 正则 1：匹配 IP 登录，形如：122.97.10.15:54321 authenticated as user_1
	authRegex = regexp.MustCompile(`([0-9a-fA-F\.\:\[\]]+):(\d+)\s+authenticated as\s+([a-zA-Z0-9_\-]+)`)
	// 正则 2：匹配目标连接，形如：user_1 connected to github.com:443
	connectRegex = regexp.MustCompile(`([a-zA-Z0-9_\-]+)\s+connected to\s+([a-zA-Z0-9\.\-]+):(\d+)`)
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

// StartDaemon 启动日志监听守护进程
func StartDaemon() {
	service := getActiveService()
	fmt.Printf("[Daemon] Starting log parser for service: %s\n", service)

	go func() {
		for {
			cmd := exec.Command("journalctl", "-f", "-u", service, "-o", "cat")
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				fmt.Printf("[Daemon] Error creating stdout pipe: %v. Retrying in 5s...\n", err)
				continue
			}

			if err := cmd.Start(); err != nil {
				fmt.Printf("[Daemon] Error starting journalctl: %v. Retrying in 5s...\n", err)
				continue
			}

			scanner := bufio.NewScanner(stdout)
			mysql := GetMysql()

			for scanner.Scan() {
				line := scanner.Text()

				// 1. 匹配 IP 连入
				if authMatches := authRegex.FindStringSubmatch(line); len(authMatches) > 3 {
					ip := strings.Trim(authMatches[1], "[]")
					username := authMatches[3]
					
					// 仅保存有效的 IP
					if net.ParseIP(ip) != nil {
						if err := mysql.SaveUserIP(username, ip); err != nil {
							fmt.Printf("[Daemon] Error saving IP for %s: %v\n", username, err)
						} else {
							fmt.Printf("[Daemon] Saved IP: %s -> %s\n", username, ip)
						}
					}
				}

				// 2. 匹配访问的目标网站/域名
				if connMatches := connectRegex.FindStringSubmatch(line); len(connMatches) > 3 {
					username := connMatches[1]
					targetHost := connMatches[2]
					
					domain := cleanDomain(targetHost)
					if domain != "" {
						if err := mysql.SaveUserDomain(username, domain); err != nil {
							fmt.Printf("[Daemon] Error saving domain for %s: %v\n", username, err)
						} else {
							fmt.Printf("[Daemon] Saved domain visit: %s -> %s\n", username, domain)
						}
					}
				}
			}

			// 如果读取中断，等待 5 秒重新连接日志流
			cmd.Wait()
			fmt.Println("[Daemon] Journalctl stream closed. Reconnecting in 5s...")
			exec.Command("sleep", "5").Run()
		}
	}()
}
