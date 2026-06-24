package controller

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"net"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"trojan/core"
	"trojan/trojan"
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

	var ipStatusList []UserIPStatus
	for _, info := range ipInfos {
		ipStatusList = append(ipStatusList, UserIPStatus{
			IP:       info.IP,
			IsActive: activeIPMap[info.IP],
			Country:  info.Country,
			Region:   info.Region,
			City:     info.City,
			ISP:      info.ISP,
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

