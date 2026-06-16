# trojan
![](https://img.shields.io/github/v/release/Jrohy/trojan.svg) 
![](https://img.shields.io/docker/pulls/jrohy/trojan.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/Jrohy/trojan)](https://goreportcard.com/report/github.com/Jrohy/trojan)
[![Downloads](https://img.shields.io/github/downloads/Jrohy/trojan/total.svg)](https://img.shields.io/github/downloads/Jrohy/trojan/total.svg)
[![License](https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true)](https://www.gnu.org/licenses/gpl-3.0.en.html)


trojan多用户管理部署程序

## 功能
- 在线web页面和命令行两种方式管理trojan多用户
- 启动 / 停止 / 重启 trojan 服务端
- 支持流量统计和流量限制
- 命令行模式管理, 支持命令补全
- 集成acme.sh证书申请
- 生成客户端配置文件
- 在线实时查看trojan日志
- 在线trojan和trojan-go随时切换
- 支持trojan://分享链接和二维码分享(仅限web页面)
- 支持转化为clash订阅地址并导入到[clash_for_windows](https://github.com/Fndroid/clash_for_windows_pkg/releases)(仅限web页面)
- 支持双域名证书申请，实现 Trojan 与 Nginx 伪装站（如 Docker 影视站）完美分流
- 自动集成 GitHub Actions 云端构建，自动编译前端并打包嵌入后端发布二进制
- 限制用户使用期限

## 安装方式
*trojan使用请提前准备好服务器可用的域名*  

###  a. 一键脚本安装
```
#安装/更新
source <(curl -sL https://raw.githubusercontent.com/akudamatata/Trojan/master/install.sh)

#卸载
source <(curl -sL https://raw.githubusercontent.com/akudamatata/Trojan/master/install.sh) --remove

```
安装完后输入'trojan'可进入管理程序   
浏览器访问 https://域名 可在线web页面管理trojan用户  
前端页面源码地址: [trojan-web](https://github.com/Jrohy/trojan-web)

### b. docker运行
1. 安装mysql  

因为mariadb内存使用比mysql至少减少一半, 所以推荐使用mariadb数据库
```
docker run --name trojan-mariadb --restart=always -p 3306:3306 -v /home/mariadb:/var/lib/mysql -e MYSQL_ROOT_PASSWORD=trojan -e MYSQL_ROOT_HOST=% -e MYSQL_DATABASE=trojan -d mariadb:10.2
```
端口和root密码以及持久化目录都可以改成其他的

2. 安装trojan
```
docker run -it -d --name trojan --net=host --restart=always --privileged akudamatata/trojan init
```
运行完后进入容器 `docker exec -it trojan bash`, 然后输入'trojan'即可进行初始化安装   

启动web服务: `systemctl start trojan-web`   

设置自启动: `systemctl enable trojan-web`

更新管理程序: `source <(curl -sL https://raw.githubusercontent.com/akudamatata/Trojan/master/install.sh)`

## 🚀 双域名分流与伪装站（影视站）配置指引

此功能可在单台服务器上实现：用 `a.com` 访问公开的影视站（HTTP/HTTPS），用 `b.com` 作为您自己独享的代理连接地址和隐藏管理面板入口。

### 第一步：避让 80 端口（针对 Docker 影视站）
如果您的影视站是 Docker 容器运行，为了将主机的 80 端口释放给 Nginx 作为分流入口，请将您的 `docker-compose.yml` 中的端口映射改为非 80 端口（例如 `8080`）：
```yaml
ports:
  - '8080:3000' # 将宿主机 80 改为 8080 端口
```
修改后重启容器：`docker compose down && docker compose up -d`。

### 第二步：申请证书与 Nginx 自动分流
在服务器终端中运行：
```bash
trojan tls
```
1. 提示输入管理面板的域名（主域名）；
2. 提示输入伪装站（影视站）的域名（如不需要可直接回车跳过）；
3. 输入完后，程序将自动通过 `acme.sh` 申请双域名 TLS 证书，并自动安装配置 Nginx；
4. 配置完成后，Nginx 会自动接管 80 端口，并引导外部的流量分发。您直接在浏览器中刷新即可看到影视站与面板同时生效！

---

## 🛡️ CentOS 系统下开启 SELinux 导致 502 Bad Gateway 修复
如果配置完成后，访问您的域名时页面显示 **502 Bad Gateway**，而在 Nginx 的错误日志 `/var/log/nginx/error.log` 中看到了 `Permission denied` 报错，这是由于 CentOS 的 **SELinux 策略**默认禁止了 Nginx 连接本地其他非标端口。

### 解决方案：
在服务器终端中直接运行以下两条命令放行即可：
```bash
# 允许 Nginx (httpd) 进行内部网络反代连接
setsebool -P httpd_can_network_connect 1

# 重启 Nginx
systemctl restart nginx
```

---

## 运行截图
![avatar](asset/1.png)
![avatar](asset/2.png)

## 命令行
```
Usage:
  trojan [flags]
  trojan [command]

Available Commands:
  add           添加用户
  clean         清空指定用户流量
  completion    自动命令补全(支持bash和zsh)
  del           删除用户
  help          Help about any command
  info          用户信息列表
  log           查看trojan日志
  port          修改trojan端口
  restart       重启trojan
  start         启动trojan
  status        查看trojan状态
  stop          停止trojan
  tls           证书安装
  update        更新trojan
  updateWeb     更新trojan管理程序
  version       显示版本号
  import [path] 导入sql文件
  export [path] 导出sql文件
  web           以web方式启动

Flags:
  -h, --help   help for trojan
```

## 注意
安装完trojan后强烈建议开启BBR等加速: [one_click_script](https://github.com/jinwyp/one_click_script)  

## Thanks
感谢JetBrains提供的免费GoLand  
[![avatar](asset/jetbrains.svg)](https://jb.gg/OpenSource)
