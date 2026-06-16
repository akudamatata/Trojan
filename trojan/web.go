package trojan

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"trojan/core"
	"trojan/util"
)

// WebMenu web管理菜单
func WebMenu() {
	fmt.Println()
	menu := []string{"重置web管理员密码", "修改显示的域名(非申请证书)", "修改web服务启动端口"}
	switch util.LoopInput("请选择: ", menu, true) {
	case 1:
		ResetAdminPass()
	case 2:
		SetDomain("")
	case 3:
		SetWebPortMenu()
	}
}

// ResetAdminPass 重置管理员密码
func ResetAdminPass() {
	inputPass := util.Input("请输入admin用户密码: ", "")
	if inputPass == "" {
		fmt.Println("撤销更改!")
	} else {
		encryPass := sha256.Sum224([]byte(inputPass))
		err := core.SetValue("admin_pass", fmt.Sprintf("%x", encryPass))
		if err == nil {
			fmt.Println(util.Green("重置admin密码成功!"))
		} else {
			fmt.Println(err)
		}
	}
}

// SetDomain 设置显示的域名
func SetDomain(domain string) {
	if domain == "" {
		domain = util.Input("请输入要显示的域名地址: ", "")
	}
	if domain == "" {
		fmt.Println("撤销更改!")
	} else {
		core.WriteDomain(domain)
		Restart()
		fmt.Println("修改domain成功!")
	}
}

// GetDomainAndPort 获取域名和端口
func GetDomainAndPort() (string, int) {
	config := core.GetConfig()
	return config.SSl.Sni, config.LocalPort
}

// GetWebPort 获取当前web服务在systemd中配置的启动端口
func GetWebPort() int {
	filePath := "/etc/systemd/system/trojan-web.service"
	if !util.IsExists(filePath) {
		return 80 // 默认端口
	}
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		return 80
	}
	content := string(contentBytes)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "ExecStart=") {
			parts := strings.Fields(line)
			for j := 0; j < len(parts)-1; j++ {
				if parts[j] == "-p" {
					port, err := strconv.Atoi(parts[j+1])
					if err == nil {
						return port
					}
				}
			}
		}
	}
	return 80
}

// SetWebPort 修改systemd中trojan-web的启动端口并重启服务
func SetWebPort(port int) error {
	filePath := "/etc/systemd/system/trojan-web.service"
	if !util.IsExists(filePath) {
		return fmt.Errorf("systemd service file not found")
	}
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	content := string(contentBytes)
	lines := strings.Split(content, "\n")
	found := false
	oldPort := 8888
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "ExecStart=") {
			parts := strings.Fields(line)
			newParts := []string{}
			hasPort := false
			for j := 0; j < len(parts); j++ {
				if parts[j] == "-p" {
					if j+1 < len(parts) {
						if op, err := strconv.Atoi(parts[j+1]); err == nil {
							oldPort = op
						}
					}
					newParts = append(newParts, "-p", strconv.Itoa(port))
					j++ // 跳过旧端口值
					hasPort = true
				} else {
					newParts = append(newParts, parts[j])
				}
			}
			if !hasPort {
				newParts = append(newParts, "-p", strconv.Itoa(port))
			}
			lines[i] = strings.Join(newParts, " ")
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("ExecStart line not found in service file")
	}
	newContent := strings.Join(lines, "\n")
	err = os.WriteFile(filePath, []byte(newContent), 0644)
	if err != nil {
		return err
	}

	// 自动更新 Nginx 配置文件中的面板反代端口并重新加载 Nginx
	updateNginxPort(oldPort, port)

	// 运行 systemctl daemon-reload 使修改生效
	util.ExecCommand("systemctl daemon-reload")

	// 异步重启 trojan-web，确保接口/CLI程序有时间返回响应
	go func() {
		time.Sleep(1 * time.Second)
		util.SystemctlRestart("trojan-web")
	}()

	return nil
}

// updateNginxPort 自动查找并替换 Nginx 配置中的面板反代端口
func updateNginxPort(oldPort, newPort int) {
	nginxConfPath := "/etc/nginx/conf.d/trojan.conf"
	if !util.IsExists(nginxConfPath) {
		return
	}
	contentBytes, err := os.ReadFile(nginxConfPath)
	if err != nil {
		return
	}
	content := string(contentBytes)

	oldStr := fmt.Sprintf("127.0.0.1:%d", oldPort)
	newStr := fmt.Sprintf("127.0.0.1:%d", newPort)

	if strings.Contains(content, oldStr) {
		newContent := strings.ReplaceAll(content, oldStr, newStr)
		err = os.WriteFile(nginxConfPath, []byte(newContent), 0644)
		if err == nil {
			// 测试并重载 nginx
			err = util.ExecCommand("nginx -t")
			if err == nil {
				_ = util.ExecCommand("systemctl reload nginx")
			}
		}
	}
}

// SetWebPortMenu 命令行菜单设置端口函数
func SetWebPortMenu() {
	currentPort := GetWebPort()
	fmt.Printf("当前web服务端口为: %s\n", util.Green(strconv.Itoa(currentPort)))
	inputPortStr := util.Input("请输入新的web服务启动端口 (1-65535): ", "")
	if inputPortStr == "" {
		fmt.Println("撤销更改!")
		return
	}
	port, err := strconv.Atoi(inputPortStr)
	if err != nil || port <= 0 || port > 65535 {
		fmt.Println(util.Red("输入端口有误, 必须是 1-65535 之间的整数!"))
		return
	}
	err = SetWebPort(port)
	if err != nil {
		fmt.Println(util.Red(fmt.Sprintf("修改端口失败: %v", err)))
	} else {
		fmt.Println(util.Green("修改端口成功, 正在重启trojan-web服务..."))
	}
}
