package trojan

import (
	"fmt"
	"os"
	"strconv"
	"github.com/tidwall/sjson"
	"trojan/core"
	"trojan/util"
)

// ConfigureNginx 自动配置 Nginx 分流
func ConfigureNginx(panelDomain, fakeDomain string) {
	fmt.Println()
	fmt.Println(util.Cyan("开始配置 Nginx 分流..."))

	// 1. 自动安装 Nginx
	fmt.Println("正在检查并安装 Nginx...")
	util.InstallPack("nginx")

	// 2. 引导用户输入影视站的实际映射端口
	var fakePort int
	for {
		portStr := util.Input("请输入您的 Docker 影视站在本地映射的端口 (默认 8080): ", "8080")
		p, err := strconv.Atoi(portStr)
		if err != nil || p <= 0 || p > 65535 {
			fmt.Println(util.Red("输入端口有误，必须是 1-65535 之间的整数，请重新输入!"))
			continue
		}
		fakePort = p
		break
	}

	// 3. 自动将 trojan-web 的端口修改为 8888
	fmt.Println("正在将 trojan-web 管理面板端口修改为 8888...")
	err := SetWebPort(8888)
	if err != nil {
		fmt.Printf(util.Red("修改面板端口失败: %v\n"), err)
	} else {
		fmt.Println(util.Green("修改面板端口成功，trojan-web 已重新绑定至本地 8888 端口。"))
	}

	// 4. 将 Trojan 的回落端口 remote_port 设为 80
	fmt.Println("正在配置 Trojan 服务端的流量回落端口为 80...")
	data := core.Load("")
	if data != nil {
		data, _ = sjson.SetBytes(data, "remote_port", 80)
		if core.Save(data, "") {
			fmt.Println(util.Green("成功更新 Trojan 回落端口至 80。"))
		} else {
			fmt.Println(util.Red("保存 Trojan 配置文件失败。"))
		}
	} else {
		fmt.Println(util.Red("加载 Trojan 配置文件失败。"))
	}

	// 5. 消除系统自带的默认 80 端口配置冲突
	if util.IsExists("/etc/nginx/sites-enabled/default") {
		_ = os.Remove("/etc/nginx/sites-enabled/default")
		fmt.Println("已移除 Debian/Ubuntu 默认的 Nginx 站点配置。")
	}
	if util.IsExists("/etc/nginx/conf.d/default.conf") {
		_ = os.Rename("/etc/nginx/conf.d/default.conf", "/etc/nginx/conf.d/default.conf.bak")
		fmt.Println("已禁用 CentOS 默认 of Nginx 站点配置。")
	}

	// 6. 生成 Nginx 配置文件 /etc/nginx/conf.d/trojan.conf
	nginxConfig := fmt.Sprintf(`# 影视站配置 (a.com) 与 Trojan 本地回落分流
server {
    listen 80;
    listen 127.0.0.1:80;
    server_name %s;

    location / {
        proxy_pass http://127.0.0.1:%d;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /trojan/ {
        proxy_pass http://127.0.0.1:8888;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

# Trojan 管理面板配置 (b.com)
server {
    listen 127.0.0.1:80;
    server_name %s;

    location / {
        proxy_pass http://127.0.0.1:8888;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

# 外网强制重定向 (b.com HTTP -> HTTPS)
server {
    listen 80;
    server_name %s;
    return 301 https://$host$request_uri;
}
`, fakeDomain, fakePort, panelDomain, panelDomain)

	nginxConfPath := "/etc/nginx/conf.d/trojan.conf"
	err = os.WriteFile(nginxConfPath, []byte(nginxConfig), 0644)
	if err != nil {
		fmt.Printf(util.Red("写入 Nginx 配置文件失败: %v\n"), err)
		return
	}
	fmt.Println(util.Green("已成功生成 Nginx 配置文件: ") + nginxConfPath)

	// 7. 启用并重启 Nginx 服务
	fmt.Println("正在启动并重载 Nginx 服务...")
	util.ExecCommand("systemctl enable nginx")
	util.ExecCommand("systemctl restart nginx")
	fmt.Println(util.Green("Nginx 分流配置全部完成！"))
}
