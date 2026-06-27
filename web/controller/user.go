package controller

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"trojan/core"
	"trojan/trojan"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// UserList 获取用户列表
func UserList(requestUser string) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	mysql := core.GetMysql()
	userList, err := mysql.GetData()
	if err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	if requestUser != "admin" {
		findUser := false
		for _, user := range userList {
			if user.Username == requestUser {
				userList = []*core.User{user}
				findUser = true
				break
			}
		}
		if !findUser {
			userList = []*core.User{}
		}
	}
	domain, port := trojan.GetDomainAndPort()
	camoDomain, _ := core.GetValue("camouflage_domain")
	if camoDomain != "" {
		domain = camoDomain
	}

	// 获取当前在线用户名列表
	activeIPs := getActiveClientIPs()
	onlineUsers := mysql.GetOnlineUsernames(activeIPs)

	responseBody.Data = map[string]interface{}{
		"domain":      domain,
		"port":        port,
		"userList":    userList,
		"onlineUsers": onlineUsers,
	}
	return &responseBody
}

// PageUserList 分页查询获取用户列表
func PageUserList(curPage int, pageSize int) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	mysql := core.GetMysql()
	pageData, err := mysql.PageList(curPage, pageSize)
	if err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	domain, port := trojan.GetDomainAndPort()
	camoDomain, _ := core.GetValue("camouflage_domain")
	if camoDomain != "" {
		domain = camoDomain
	}
	responseBody.Data = map[string]interface{}{
		"domain":   domain,
		"port":     port,
		"pageData": pageData,
	}
	return &responseBody
}

// CreateUser 创建用户
func CreateUser(username string, password string) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	if username == "admin" {
		responseBody.Msg = "不能创建用户名为admin的用户!"
		return &responseBody
	}
	mysql := core.GetMysql()
	if user := mysql.GetUserByName(username); user != nil {
		responseBody.Msg = "已存在用户名为: " + username + " 的用户!"
		return &responseBody
	}
	pass, err := base64.StdEncoding.DecodeString(password)
	if err != nil {
		responseBody.Msg = "Base64解码失败: " + err.Error()
		return &responseBody
	}
	if user := mysql.GetUserByPass(password); user != nil {
		responseBody.Msg = "已存在密码为: " + string(pass) + " 的用户!"
		return &responseBody
	}
	if err := mysql.CreateUser(username, password, string(pass)); err != nil {
		responseBody.Msg = err.Error()
	}
	return &responseBody
}

// UpdateUser 更新用户
func UpdateUser(id uint, username string, password string) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	if username == "admin" {
		responseBody.Msg = "不能更改用户名为admin的用户!"
		return &responseBody
	}
	mysql := core.GetMysql()
	userList, err := mysql.GetData(strconv.Itoa(int(id)))
	if err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	if userList[0].Username != username {
		if user := mysql.GetUserByName(username); user != nil {
			responseBody.Msg = "已存在用户名为: " + username + " 的用户!"
			return &responseBody
		}
	}
	pass, err := base64.StdEncoding.DecodeString(password)
	if err != nil {
		responseBody.Msg = "Base64解码失败: " + err.Error()
		return &responseBody
	}
	if userList[0].Password != password {
		if user := mysql.GetUserByPass(password); user != nil {
			responseBody.Msg = "已存在密码为: " + string(pass) + " 的用户!"
			return &responseBody
		}
	}
	if err := mysql.UpdateUser(id, username, password, string(pass)); err != nil {
		responseBody.Msg = err.Error()
	}
	return &responseBody
}

// DelUser 删除用户
func DelUser(id uint) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	mysql := core.GetMysql()
	if err := mysql.DeleteUser(id); err != nil {
		responseBody.Msg = err.Error()
	} else {
		trojan.Restart()
	}
	return &responseBody
}

// SetExpire 设置用户过期
func SetExpire(id uint, useDays uint) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	mysql := core.GetMysql()
	if err := mysql.SetExpire(id, useDays); err != nil {
		responseBody.Msg = err.Error()
	}
	return &responseBody
}

// CancelExpire 取消设置用户过期
func CancelExpire(id uint) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	mysql := core.GetMysql()
	if err := mysql.CancelExpire(id); err != nil {
		responseBody.Msg = err.Error()
	}
	return &responseBody
}

// ClashSubInfo 获取clash订阅/通用订阅信息
func ClashSubInfo(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.String(200, "token is null")
		return
	}
	decodeByte, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		c.String(200, "token is error")
		return
	}
	if !gjson.GetBytes(decodeByte, "user").Exists() || !gjson.GetBytes(decodeByte, "pass").Exists() {
		c.String(200, "token is error")
		return
	}
	username := gjson.GetBytes(decodeByte, "user").String()
	password := gjson.GetBytes(decodeByte, "pass").String()

	mysql := core.GetMysql()
	user := mysql.GetUserByName(username)
	if user != nil {
		pass, _ := base64.StdEncoding.DecodeString(user.Password)
		if password == string(pass) {
			// 记录订阅拉取日志
			clientIP := c.ClientIP()
			userAgent := c.Request.UserAgent()
			go func() {
				country, region, city, isp := queryGeoIP(clientIP)
				mysql.SaveUserSubLog(username, clientIP, userAgent, country, region, city, isp)
			}()
			var wsData, wsHost string
			userInfo := fmt.Sprintf("upload=%d, download=%d", user.Upload, user.Download)
			if user.Quota != -1 {
				userInfo = fmt.Sprintf("%s, total=%d", userInfo, user.Quota)
			}
			if user.ExpiryDate != "" {
				utc, _ := time.LoadLocation("Asia/Shanghai")
				t, _ := time.ParseInLocation("2006-01-02", user.ExpiryDate, utc)
				userInfo = fmt.Sprintf("%s, expire=%d", userInfo, t.Unix())
			}
			c.Header("content-disposition", fmt.Sprintf("attachment; filename=%s", user.Username))
			c.Header("subscription-userinfo", userInfo)

			domain, port := trojan.GetDomainAndPort()
			camoDomain, _ := core.GetValue("camouflage_domain")
			targetDomain := domain
			if camoDomain != "" {
				targetDomain = camoDomain
			}
			
			configData := string(core.Load(""))
			wsEnabled := gjson.Get(configData, "websocket").Exists() && gjson.Get(configData, "websocket.enabled").Bool()
			var wsPath, wsHostHeader string
			if wsEnabled {
				wsPath = gjson.Get(configData, "websocket.path").String()
				if gjson.Get(configData, "websocket.host").Exists() {
					wsHostHeader = gjson.Get(configData, "websocket.host").String()
				}
			}

			flag := c.Query("flag")
			if flag == "shadowrocket" || flag == "rocket" || flag == "universal" || flag == "base64" {
				val := url.Values{}
				if camoDomain == "" {
					val.Set("sni", domain)
				}
				if wsEnabled {
					val.Set("type", "ws")
					if wsPath != "" {
						val.Set("path", wsPath)
					}
					if wsHostHeader != "" {
						val.Set("host", wsHostHeader)
					}
				}
				nodeName := fmt.Sprintf("%s:%d", targetDomain, port)
				uri := fmt.Sprintf("trojan://%s@%s:%d?%s#%s", 
					url.PathEscape(password), targetDomain, port, val.Encode(), url.PathEscape(nodeName))
				
				result := base64.StdEncoding.EncodeToString([]byte(uri + "\n"))
				c.String(200, result)
				return
			}

			name := fmt.Sprintf("%s:%d", targetDomain, port)
			if wsEnabled {
				if wsHostHeader != "" {
					wsHost = fmt.Sprintf(", headers: {Host: %s}", wsHostHeader)
				}
				wsOpt := fmt.Sprintf("{path: %s%s}", wsPath, wsHost)
				wsData = fmt.Sprintf(", network: ws, udp: true, ws-opts: %s", wsOpt)
			}
			sniStr := ""
			if camoDomain == "" {
				sniStr = fmt.Sprintf(", sni: %s", domain)
			}
			proxyData := fmt.Sprintf("  - {name: %s, server: %s, port: %d, type: trojan, password: %s%s%s}",
				name, targetDomain, port, password, sniStr, wsData)
			result := fmt.Sprintf(`proxies:
%s

proxy-groups:
  - name: PROXY
    type: select
    proxies:
      - %s

%s
`, proxyData, name, clashRules())
			c.String(200, result)
			return
		}
	}
	c.String(200, "token is error")
}

// UserIPStatus 结构体，用于返回用户的 IP 连接状态（含缓存的 GeoIP 信息）
type UserIPStatus struct {
	IP       string `json:"ip"`
	IsActive bool   `json:"is_active"`
	Country  string `json:"country"`
	Region   string `json:"region"`
	City     string `json:"city"`
	ISP      string `json:"isp"`
	IsBanned bool   `json:"is_banned"`
}

// getActiveClientIPs 从系统网络套接字中获取当前活跃连接的所有 IP
func getActiveClientIPs() []string {
	_, port := trojan.GetDomainAndPort()
	cmdStr := fmt.Sprintf("ss -t -H -a sport = :%d", port)
	out, err := exec.Command("bash", "-c", cmdStr).CombinedOutput()
	if err != nil {
		return []string{}
	}

	ipMap := make(map[string]bool)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 5 {
			peer := fields[4]
			if idx := strings.LastIndex(peer, ":"); idx != -1 {
				ip := peer[:idx]
				ip = strings.Trim(ip, "[]")
				if parsedIP := net.ParseIP(ip); parsedIP != nil {
					if ipv4 := parsedIP.To4(); ipv4 != nil {
						ipMap[ipv4.String()] = true
					} else {
						ipMap[parsedIP.String()] = true
					}
				}
			}
		}
	}

	var ips []string
	for ip := range ipMap {
		ips = append(ips, ip)
	}
	return ips
}

// UserDetail 获取用户详情（包括最近IP和Top10网站）
func UserDetail(username string) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)

	mysql := core.GetMysql()
	user := mysql.GetUserByName(username)
	if user == nil {
		responseBody.Msg = "用户不存在"
		return &responseBody
	}

	ipInfos, err := mysql.GetUserIPs(username)
	if err != nil {
		ipInfos = []core.UserIPInfo{}
	}

	activeIPs := getActiveClientIPs()
	activeIPMap := make(map[string]bool)
	for _, ip := range activeIPs {
		activeIPMap[ip] = true
	}

	bannedIPMap := make(map[string]bool)
	db := mysql.GetDB()
	if db != nil {
		rows, err := db.Query("SELECT ip FROM ip_blacklist WHERE expire_at IS NULL OR expire_at > NOW()")
		if err == nil {
			for rows.Next() {
				var ip string
				if err := rows.Scan(&ip); err == nil {
					bannedIPMap[ip] = true
				}
			}
			rows.Close()
		}
		db.Close()
	}

	var ipStatusList []UserIPStatus
	for _, info := range ipInfos {
		ipStatusList = append(ipStatusList, UserIPStatus{
			IP:       info.IP,
			IsActive: activeIPMap[info.IP],
			Country:  info.Country,
			Region:   info.Region,
			City:     info.City,
			ISP:      info.ISP,
			IsBanned: bannedIPMap[info.IP],
		})
	}

	domains, err := mysql.GetUserTopDomains(username, 10)
	if err != nil {
		domains = []core.UserDomain{}
	}

	domain, port := trojan.GetDomainAndPort()
	camoDomain, _ := core.GetValue("camouflage_domain")
	if camoDomain != "" {
		domain = camoDomain
	}

	responseBody.Data = map[string]interface{}{
		"id":         user.ID,
		"username":   user.Username,
		"password":   user.Password, // 返回 Base64 编码的明文密码以生成分享订阅链接
		"download":   user.Download,
		"upload":     user.Upload,
		"quota":      user.Quota,
		"useDays":    user.UseDays,
		"expiryDate": user.ExpiryDate,
		"ips":        ipStatusList,
		"domains":    domains,
		"domain":     domain,
		"port":       port,
	}
	return &responseBody
}

// SaveIPGeo 前端查询到 GeoIP 后回写缓存到数据库
func SaveIPGeo(username, ip, country, region, city, isp string) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	mysql := core.GetMysql()
	if err := mysql.UpdateIPGeo(username, ip, country, region, city, isp); err != nil {
		responseBody.Msg = err.Error()
	}
	return &responseBody
}

// queryGeoIP 从 Go 后台发起 GeoIP 查询
func queryGeoIP(ip string) (country, region, city, isp string) {
	country, region, city, isp = "unknown", "", "", "unknown"
	if ip == "127.0.0.1" || ip == "::1" || strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") {
		return "局域网", "", "", "Local Network"
	}
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://ip-api.com/json/%s?lang=zh-CN", ip))
	if err != nil {
		resp, err = client.Get(fmt.Sprintf("https://ipapi.co/%s/json/", ip))
		if err != nil {
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var res map[string]interface{}
		json.Unmarshal(body, &res)
		if res != nil {
			if c, ok := res["country_name"].(string); ok { country = c }
			if r, ok := res["region"].(string); ok { region = r }
			if ci, ok := res["city"].(string); ok { city = ci }
			if o, ok := res["org"].(string); ok { isp = o }
		}
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var res map[string]interface{}
	json.Unmarshal(body, &res)
	if res != nil && res["status"] == "success" {
		if c, ok := res["country"].(string); ok { country = c }
		if r, ok := res["regionName"].(string); ok { region = r }
		if ci, ok := res["city"].(string); ok { city = ci }
		if i, ok := res["isp"].(string); ok { isp = i }
	}
	return
}

// getActiveSocketsWithPort 获取所有当前活跃的远程 socket 连接（包含 IP 与端口）
func getActiveSocketsWithPort() []string {
	_, port := trojan.GetDomainAndPort()
	cmdStr := fmt.Sprintf("ss -t -H -a sport = :%d", port)
	out, err := exec.Command("bash", "-c", cmdStr).CombinedOutput()
	if err != nil {
		return []string{}
	}

	var ipPorts []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 5 {
			peer := fields[4]
			if idx := strings.LastIndex(peer, ":"); idx != -1 {
				ip := peer[:idx]
				ip = strings.Trim(ip, "[]")
				portStr := peer[idx+1:]
				if parsedIP := net.ParseIP(ip); parsedIP != nil {
					normIP := parsedIP.String()
					if ipv4 := parsedIP.To4(); ipv4 != nil {
						normIP = ipv4.String()
					}
					ipPorts = append(ipPorts, normIP+":"+portStr)
				}
			}
		}
	}
	return ipPorts
}

// GetActiveConnections 获取用户的实时活跃连接列表
func GetActiveConnections(username string) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)

	// 1. 获取所有当前活跃的远程 socket 连接
	activeIPPorts := getActiveSocketsWithPort()

	// 2. 清理内存中已经关闭的会话，并保证当前活跃会话在内存 map 中
	core.ActiveConnsMu.Lock()
	activeMap := make(map[string]bool)
	for _, ipPort := range activeIPPorts {
		activeMap[ipPort] = true
	}

	// 删除 ActiveConns 中已关闭的会话
	for ipPort := range core.ActiveConns {
		if !activeMap[ipPort] {
			delete(core.ActiveConns, ipPort)
		}
	}

	// 补充没有记录在 ActiveConns 里的活跃连接
	mysql := core.GetMysql()
	for _, ipPort := range activeIPPorts {
		if _, ok := core.ActiveConns[ipPort]; !ok {
			parts := strings.SplitN(ipPort, ":", 2)
			if len(parts) == 2 {
				// 反查用户名
				connUser := ""
				rows, err := mysql.GetDB().Query("SELECT username FROM user_ips WHERE client_ip = ? LIMIT 1", parts[0])
				if err == nil {
					if rows.Next() {
						rows.Scan(&connUser)
					}
					rows.Close()
				}
				if connUser == "" {
					connUser = "unknown"
				}

				core.ActiveConns[ipPort] = &core.ActiveConnection{
					Username:    connUser,
					ClientIP:    parts[0],
					ClientPort:  parts[1],
					TargetHost:  "Direct Tunnel",
					ConnectedAt: time.Now().Add(-5 * time.Second),
				}
			}
		}
	}

	// 3. 过滤出当前用户的会话
	var userConns []core.ActiveConnection
	for _, conn := range core.ActiveConns {
		if conn.Username == username {
			userConns = append(userConns, *conn)
		}
	}
	core.ActiveConnsMu.Unlock()

	// 保护返回值不为 nil
	if userConns == nil {
		userConns = []core.ActiveConnection{}
	}

	responseBody.Data = userConns
	return &responseBody
}

// KillActiveConnection 强行切断用户的某个活跃会话
func KillActiveConnection(clientIP string, clientPort string) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)

	if clientIP == "" || clientPort == "" {
		responseBody.Msg = "IP or Port is null"
		return &responseBody
	}

	// 1. 尝试使用 ss -K 直接断开（如果内核支持，这是最快的方式）
	cmdStr := fmt.Sprintf("ss -K state established dst %s dport = :%s", clientIP, clientPort)
	exec.Command("bash", "-c", cmdStr).Run()

	// 2. 对于不支持 CONFIG_INET_DIAG_DESTROY 的内核，使用 iptables + conntrack 强制断流
	inputRule := fmt.Sprintf("/usr/sbin/iptables -I INPUT -p tcp -s %s --sport %s -j REJECT --reject-with tcp-reset", clientIP, clientPort)
	outputRule := fmt.Sprintf("/usr/sbin/iptables -I OUTPUT -p tcp -d %s --dport %s -j REJECT --reject-with tcp-reset", clientIP, clientPort)
	conntrackCmd := fmt.Sprintf("/usr/sbin/conntrack -D -p tcp -s %s --sport %s", clientIP, clientPort)

	exec.Command("bash", "-c", inputRule).Run()
	exec.Command("bash", "-c", outputRule).Run()
	exec.Command("bash", "-c", conntrackCmd).Run()

	// 2秒后在后台自动清理 iptables 规则，避免影响新连接
	go func() {
		time.Sleep(2 * time.Second)
		cleanInput := fmt.Sprintf("/usr/sbin/iptables -D INPUT -p tcp -s %s --sport %s -j REJECT --reject-with tcp-reset", clientIP, clientPort)
		cleanOutput := fmt.Sprintf("/usr/sbin/iptables -D OUTPUT -p tcp -d %s --dport %s -j REJECT --reject-with tcp-reset", clientIP, clientPort)
		exec.Command("bash", "-c", cleanInput).Run()
		exec.Command("bash", "-c", cleanOutput).Run()
	}()

	// 成功后，从内存 map 中删除
	core.ActiveConnsMu.Lock()
	delete(core.ActiveConns, clientIP+":"+clientPort)
	core.ActiveConnsMu.Unlock()

	return &responseBody
}

// KillConnectionsByIP 强行切断某个 IP 的所有活跃会话（无需端口，直接枚举 ss 当前所有连接）
func KillConnectionsByIP(clientIP string) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)

	if clientIP == "" {
		responseBody.Msg = "IP is null"
		return &responseBody
	}

	// 1. 通过 ss 枚举该 IP 的所有活跃端口
	_, trojanPort := trojan.GetDomainAndPort()
	cmdStr := fmt.Sprintf("ss -t -H -a sport = :%d dst %s", trojanPort, clientIP)
	out, _ := exec.Command("bash", "-c", cmdStr).CombinedOutput()

	killedCount := 0
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 5 {
			continue
		}
		peer := fields[4]
		idx := strings.LastIndex(peer, ":")
		if idx == -1 {
			continue
		}
		portStr := peer[idx+1:]
		if portStr == "" || portStr == "*" {
			continue
		}

		// ss -K 断开单个连接（带端口）
		ssKill := fmt.Sprintf("ss -K state established dst %s dport = :%s", clientIP, portStr)
		exec.Command("bash", "-c", ssKill).Run()

		// iptables 方式兜底
		inputRule := fmt.Sprintf("/usr/sbin/iptables -I INPUT -p tcp -s %s --sport %s -j REJECT --reject-with tcp-reset", clientIP, portStr)
		outputRule := fmt.Sprintf("/usr/sbin/iptables -I OUTPUT -p tcp -d %s --dport %s -j REJECT --reject-with tcp-reset", clientIP, portStr)
		conntrackCmd := fmt.Sprintf("/usr/sbin/conntrack -D -p tcp -s %s --sport %s", clientIP, portStr)
		exec.Command("bash", "-c", inputRule).Run()
		exec.Command("bash", "-c", outputRule).Run()
		exec.Command("bash", "-c", conntrackCmd).Run()

		// 2 秒后清理 iptables 规则
		pStr := portStr
		go func() {
			time.Sleep(2 * time.Second)
			cleanIn := fmt.Sprintf("/usr/sbin/iptables -D INPUT -p tcp -s %s --sport %s -j REJECT --reject-with tcp-reset", clientIP, pStr)
			cleanOut := fmt.Sprintf("/usr/sbin/iptables -D OUTPUT -p tcp -d %s --dport %s -j REJECT --reject-with tcp-reset", clientIP, pStr)
			exec.Command("bash", "-c", cleanIn).Run()
			exec.Command("bash", "-c", cleanOut).Run()
		}()

		// 从内存 map 中删除
		ipPort := clientIP + ":" + portStr
		core.ActiveConnsMu.Lock()
		delete(core.ActiveConns, ipPort)
		core.ActiveConnsMu.Unlock()

		killedCount++
	}

	// 如果 ss 没找到端口（持久连接但无子隧道），fallback：直接按 IP 拒绝所有流量
	if killedCount == 0 {
		inputRule := fmt.Sprintf("/usr/sbin/iptables -I INPUT -p tcp -s %s -j REJECT --reject-with tcp-reset", clientIP)
		outputRule := fmt.Sprintf("/usr/sbin/iptables -I OUTPUT -p tcp -d %s -j REJECT --reject-with tcp-reset", clientIP)
		conntrackCmd := fmt.Sprintf("/usr/sbin/conntrack -D -p tcp --src %s", clientIP)
		exec.Command("bash", "-c", inputRule).Run()
		exec.Command("bash", "-c", outputRule).Run()
		exec.Command("bash", "-c", conntrackCmd).Run()

		ip := clientIP
		go func() {
			time.Sleep(3 * time.Second)
			cleanIn := fmt.Sprintf("/usr/sbin/iptables -D INPUT -p tcp -s %s -j REJECT --reject-with tcp-reset", ip)
			cleanOut := fmt.Sprintf("/usr/sbin/iptables -D OUTPUT -p tcp -d %s -j REJECT --reject-with tcp-reset", ip)
			exec.Command("bash", "-c", cleanIn).Run()
			exec.Command("bash", "-c", cleanOut).Run()
		}()
		killedCount = 1 // 表示已执行断流操作
	}

	responseBody.Data = map[string]interface{}{"killed": killedCount}
	return &responseBody
}

// GetUserTrafficHistory 获取用户最近 30 天的每日流量数据
func GetUserTrafficHistory(username string) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)

	mysql := core.GetMysql()
	list, err := mysql.GetUserTrafficHistory(username, 30)
	if err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}

	if list == nil {
		list = []core.UserTrafficDaily{}
	}

	responseBody.Data = list
	return &responseBody
}

// GetUserSubLogs 获取订阅审计日志和常用客户端占比
func GetUserSubLogs(username string) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)

	mysql := core.GetMysql()
	logs, err := mysql.GetUserSubLogs(username, 15)
	if err != nil {
		logs = []core.UserSubLog{}
	}

	allLogs, err := mysql.GetUserSubLogs(username, 100)
	clientStats := make(map[string]int)
	if err == nil {
		for _, logItem := range allLogs {
			ua := strings.ToLower(logItem.UserAgent)
			clientType := "Other"
			if strings.Contains(ua, "clash") {
				clientType = "Clash"
			} else if strings.Contains(ua, "shadowrocket") || strings.Contains(ua, "rocket") {
				clientType = "Shadowrocket"
			} else if strings.Contains(ua, "sing-box") {
				clientType = "Sing-Box"
			} else if strings.Contains(ua, "surge") {
				clientType = "Surge"
			} else if strings.Contains(ua, "quantumult") {
				clientType = "Quantumult X"
			} else if strings.Contains(ua, "mozilla") || strings.Contains(ua, "chrome") || strings.Contains(ua, "safari") {
				clientType = "Browser"
			}
			clientStats[clientType]++
		}
	}

	var suspectAlert bool
	var suspectCount int
	db := mysql.GetDB()
	if db != nil {
		err := db.QueryRow(`
			SELECT COUNT(DISTINCT client_ip) FROM user_sub_logs 
			WHERE username = ? AND accessed_at >= NOW() - INTERVAL 1 DAY
		`, username).Scan(&suspectCount)
		if err == nil && suspectCount >= 3 {
			suspectAlert = true
		}
		db.Close()
	}

	if logs == nil {
		logs = []core.UserSubLog{}
	}

	responseBody.Data = map[string]interface{}{
		"logs":         logs,
		"clientStats":  clientStats,
		"suspectAlert": suspectAlert,
		"suspectCount": suspectCount,
	}
	return &responseBody
}

// GetUserDomainStats 获取域名偏好分类与合规评分
func GetUserDomainStats(username string) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)

	mysql := core.GetMysql()
	domains, err := mysql.GetUserTopDomains(username, 100)
	if err != nil {
		domains = []core.UserDomain{}
	}

	categories := map[string]int{
		"Video":   0,
		"Social":  0,
		"Tech":    0,
		"Abuse":   0,
		"Search":  0,
	}

	videoKeywords := []string{"youtube", "netflix", "twitch", "bilibili", "tiktok", "vimeo", "hulu", "disney"}
	socialKeywords := []string{"telegram", "tg.me", "twitter", "t.co", "facebook", "instagram", "whatsapp", "discord"}
	techKeywords := []string{"github", "githubusercontent", "stackoverflow", "npmjs", "docker", "python", "golang", "microsoft", "google", "gpt", "openai"}
	abuseKeywords := []string{"torrent", "tracker", "peer", "bittorrent", "utorrent", "opentracker", "announce", "magnet", "xunlei", "thunder", "qbittorrent"}

	score := 100
	abuseCount := 0

	for _, d := range domains {
		dName := strings.ToLower(d.Domain)
		isMatched := false

		for _, kw := range abuseKeywords {
			if strings.Contains(dName, kw) {
				categories["Abuse"] += d.VisitCount
				abuseCount += d.VisitCount
				isMatched = true
				break
			}
		}

		if isMatched {
			continue
		}

		for _, kw := range videoKeywords {
			if strings.Contains(dName, kw) {
				categories["Video"] += d.VisitCount
				isMatched = true
				break
			}
		}
		if isMatched {
			continue
		}

		for _, kw := range socialKeywords {
			if strings.Contains(dName, kw) {
				categories["Social"] += d.VisitCount
				isMatched = true
				break
			}
		}
		if isMatched {
			continue
		}

		for _, kw := range techKeywords {
			if strings.Contains(dName, kw) {
				categories["Tech"] += d.VisitCount
				isMatched = true
				break
			}
		}
		if isMatched {
			continue
		}

		categories["Search"] += d.VisitCount
	}

	if abuseCount > 0 {
		score = 100 - (abuseCount * 5)
		if score < 10 {
			score = 10
		}
	}

	responseBody.Data = map[string]interface{}{
		"categories": categories,
		"score":      score,
	}
	return &responseBody
}

// IPBlacklistItem represents an item in the IP blacklist
type IPBlacklistItem struct {
	ID        uint    `json:"id"`
	IP        string  `json:"ip"`
	BanType   string  `json:"ban_type"`
	CreatedAt string  `json:"created_at"`
	ExpireAt  *string `json:"expire_at"`
}

// GetIPBlacklist returns all blacklisted IPs
func GetIPBlacklist() *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)

	mysql := core.GetMysql()
	db := mysql.GetDB()
	if db == nil {
		responseBody.Msg = "db error"
		return &responseBody
	}
	defer db.Close()

	rows, err := db.Query("SELECT id, ip, ban_type, DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:%s'), DATE_FORMAT(expire_at, '%Y-%m-%d %H:%i:%s') FROM ip_blacklist ORDER BY id DESC")
	if err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	defer rows.Close()

	list := []IPBlacklistItem{}
	for rows.Next() {
		var item IPBlacklistItem
		var createdAt string
		var expireAt sql.NullString

		err := rows.Scan(&item.ID, &item.IP, &item.BanType, &createdAt, &expireAt)
		if err == nil {
			item.CreatedAt = createdAt
			if expireAt.Valid {
				item.ExpireAt = &expireAt.String
			}
			list = append(list, item)
		} else {
			fmt.Println("[Firewall] GetIPBlacklist scan error:", err)
		}
	}

	responseBody.Data = list
	return &responseBody
}

// BanIP bans a client IP for a specified duration
func BanIP(ip, duration string) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)

	if ip == "" {
		responseBody.Msg = "IP cannot be empty"
		return &responseBody
	}

	mysql := core.GetMysql()
	db := mysql.GetDB()
	if db == nil {
		responseBody.Msg = "db error"
		return &responseBody
	}
	defer db.Close()

	var expireAt *time.Time
	now := time.Now()
	switch duration {
	case "day":
		t := now.AddDate(0, 0, 1)
		expireAt = &t
	case "week":
		t := now.AddDate(0, 0, 7)
		expireAt = &t
	case "month":
		t := now.AddDate(0, 1, 0)
		expireAt = &t
	case "permanent":
		expireAt = nil
	default:
		responseBody.Msg = "invalid duration"
		return &responseBody
	}

	// 1. Write to DB (insert or update)
	var err error
	if expireAt != nil {
		_, err = db.Exec("INSERT INTO ip_blacklist (ip, ban_type, expire_at) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE ban_type = VALUES(ban_type), expire_at = VALUES(expire_at)", ip, duration, *expireAt)
	} else {
		_, err = db.Exec("INSERT INTO ip_blacklist (ip, ban_type, expire_at) VALUES (?, ?, NULL) ON DUPLICATE KEY UPDATE ban_type = VALUES(ban_type), expire_at = NULL", ip, duration)
	}

	if err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}

	// 2. Call firewall block. Unblock first to clean up duplicate rules.
	core.UnblockIP(ip)
	if err := core.BlockIP(ip); err != nil {
		responseBody.Msg = "Failed to apply firewall rule: " + err.Error()
		return &responseBody
	}

	// 3. Force kill active connections from this IP immediately
	KillConnectionsByIP(ip)

	return &responseBody
}

// UnbanIP unbans a client IP
func UnbanIP(ip string) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)

	if ip == "" {
		responseBody.Msg = "IP cannot be empty"
		return &responseBody
	}

	mysql := core.GetMysql()
	db := mysql.GetDB()
	if db == nil {
		responseBody.Msg = "db error"
		return &responseBody
	}
	defer db.Close()

	// 1. Delete from DB
	_, err := db.Exec("DELETE FROM ip_blacklist WHERE ip = ?", ip)
	if err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}

	// 2. Call firewall unblock
	if err := core.UnblockIP(ip); err != nil {
		responseBody.Msg = "Failed to remove firewall rule: " + err.Error()
		return &responseBody
	}

	return &responseBody
}

// UserAuditRecord 行为审计单条记录
type UserAuditRecord struct {
	Username       string `json:"username"`
	Domain         string `json:"domain"`
	MappedDomain   string `json:"mapped_domain"`
	Category       string `json:"category"`
	VisitCount     int    `json:"visit_count"`
	LastVisitedAt  string `json:"last_visited_at"`
	Date           string `json:"date"`
}

// GetUserAuditList 获取行为审计列表
func GetUserAuditList(usernameParam, domainParam, categoryParam string, hideCDN bool, dateStart, dateEnd string, page, limit int) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)

	mysql := core.GetMysql()
	db := mysql.GetDB()
	if db == nil {
		responseBody.Msg = "db error"
		return &responseBody
	}
	defer db.Close()

	// 拼装 SQL 查询条件
	whereClauses := []string{"1=1"}
	var args []interface{}

	if usernameParam != "" {
		whereClauses = append(whereClauses, "username LIKE ?")
		args = append(args, "%"+usernameParam+"%")
	}
	if domainParam != "" {
		whereClauses = append(whereClauses, "domain LIKE ?")
		args = append(args, "%"+domainParam+"%")
	}
	if dateStart != "" {
		whereClauses = append(whereClauses, "date >= ?")
		args = append(args, dateStart)
	}
	if dateEnd != "" {
		whereClauses = append(whereClauses, "date <= ?")
		args = append(args, dateEnd)
	}

	// 1. 查询所有符合基础条件的数据并在内存中清洗过滤（因为分类和静态 CDN 状态是动态判定的）
	querySql := "SELECT username, domain, date, visit_count, last_visited_at FROM user_domains_daily WHERE " + 
		strings.Join(whereClauses, " AND ") + " ORDER BY last_visited_at DESC LIMIT 5000"
	
	rows, err := db.Query(querySql, args...)
	if err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	defer rows.Close()

	var allRecords []UserAuditRecord
	for rows.Next() {
		var r UserAuditRecord
		var lastVisited string
		if err := rows.Scan(&r.Username, &r.Domain, &r.Date, &r.VisitCount, &lastVisited); err == nil {
			r.Username = sanitizeString(r.Username)
			r.Domain = sanitizeString(r.Domain)
			r.LastVisitedAt = lastVisited
			
			// 动态映射和分类
			info := core.GetDomainAuditInfo(r.Domain)
			r.MappedDomain = info.MappedDomain
			r.Category = info.Category

			// 过滤静态 CDN
			if hideCDN && info.IsStaticCDN {
				continue
			}

			// 过滤类别
			if categoryParam != "" && r.Category != categoryParam {
				continue
			}

			allRecords = append(allRecords, r)
		}
	}

	// 2. 分页处理
	total := len(allRecords)
	startIndex := (page - 1) * limit
	if startIndex < 0 {
		startIndex = 0
	}
	endIndex := startIndex + limit
	if endIndex > total {
		endIndex = total
	}

	var pagedRecords []UserAuditRecord
	if startIndex < total {
		pagedRecords = allRecords[startIndex:endIndex]
	} else {
		pagedRecords = []UserAuditRecord{}
	}

	responseBody.Data = map[string]interface{}{
		"total":   total,
		"records": pagedRecords,
	}
	return &responseBody
}

// DomainUserRecord 某个域名的用户访问统计
type DomainUserRecord struct {
	Username       string `json:"username"`
	VisitCount     int    `json:"visit_count"`
	LastVisitedAt  string `json:"last_visited_at"`
}

// GetDomainUsersList 获取访问某个特定域名的用户排行
func GetDomainUsersList(domainQuery string, dateStart, dateEnd string) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)

	mysql := core.GetMysql()
	db := mysql.GetDB()
	if db == nil {
		responseBody.Msg = "db error"
		return &responseBody
	}
	defer db.Close()

	whereClauses := []string{"1=1"}
	var args []interface{}

	if dateStart != "" {
		whereClauses = append(whereClauses, "date >= ?")
		args = append(args, dateStart)
	}
	if dateEnd != "" {
		whereClauses = append(whereClauses, "date <= ?")
		args = append(args, dateEnd)
	}

	querySql := "SELECT username, domain, visit_count, last_visited_at FROM user_domains_daily WHERE " + 
		strings.Join(whereClauses, " AND ") + " ORDER BY last_visited_at DESC LIMIT 5000"
	
	rows, err := db.Query(querySql, args...)
	if err != nil {
		responseBody.Msg = err.Error()
		return &responseBody
	}
	defer rows.Close()

	// 聚合用户数据
	userMap := make(map[string]*DomainUserRecord)
	domainQueryLower := strings.ToLower(domainQuery)

	for rows.Next() {
		var username, domain string
		var count int
		var lastVisited string
		if err := rows.Scan(&username, &domain, &count, &lastVisited); err == nil {
			username = sanitizeString(username)
			domain = sanitizeString(domain)
			info := core.GetDomainAuditInfo(domain)
			
			// 匹配：当原始域名包含查询字符串，或者映射主品牌名称包含查询字符串时匹配
			match := strings.Contains(strings.ToLower(domain), domainQueryLower) || 
				strings.Contains(strings.ToLower(info.MappedDomain), domainQueryLower)
			
			if !match {
				continue
			}

			lastVisitedStr := lastVisited
			if record, exists := userMap[username]; exists {
				record.VisitCount += count
				if lastVisitedStr > record.LastVisitedAt {
					record.LastVisitedAt = lastVisitedStr
				}
			} else {
				userMap[username] = &DomainUserRecord{
					Username:      username,
					VisitCount:    count,
					LastVisitedAt: lastVisitedStr,
				}
			}
		}
	}

	var records []DomainUserRecord
	for _, r := range userMap {
		records = append(records, *r)
	}

	// 排序：按访问次数降序
	for i := 0; i < len(records); i++ {
		for j := i + 1; j < len(records); j++ {
			if records[i].VisitCount < records[j].VisitCount {
				records[i], records[j] = records[j], records[i]
			}
		}
	}

	responseBody.Data = records
	return &responseBody
}

// sanitizeString removes control characters that may break frontend rendering (e.g. Echarts eval)
func sanitizeString(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\ufffd", "?")
	return s
}
