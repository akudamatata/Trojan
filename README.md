# 🚀 Trojan Multi-User Manager

<p align="center">
  <img src="https://img.shields.io/github/v/release/akudamatata/Trojan?style=flat-square&color=6366f1" alt="Release">
  <img src="https://img.shields.io/docker/pulls/jrohy/trojan?style=flat-square&color=10b981" alt="Docker Pulls">
  <a href="https://goreportcard.com/report/github.com/Jrohy/trojan">
    <img src="https://goreportcard.com/badge/github.com/Jrohy/trojan?style=flat-square" alt="Go Report Card">
  </a>
  <img src="https://img.shields.io/github/downloads/akudamatata/Trojan/total?style=flat-square&color=f59e0b" alt="Downloads">
  <img src="https://img.shields.io/badge/license-GPL%20V3-blue?style=flat-square" alt="License">
</p>

---

**Trojan 多用户管理系统** 是一个支持 **Web 网页管理面板** 与 **Linux 命令行终端 (CLI)** 双端运行的高性能 Trojan 部署平台。它不仅可以帮您快速安装管理多用户，还支持智能的 Nginx 流量分流，将普通访问伪装至伪装站，极大提升连接的隐蔽性和防阻断能力。

---

## ✨ 核心特性

- 🌐 **双端协同管理**：拥有现代精美的在线 Web 管理页面，同时也保留了高效的 Linux 命令行交互终端。
- 🛡️ **智能分流与伪装**：支持双域名证书申请，实现 Trojan 代理与 Nginx 伪装站（如 Docker 伪装站）的无感共存与完美分流。
- 🚀 **高度自动化运维**：
  - **热更证书**：内置 ACME **Webroot 验证模式**，申请和续签证书无需停用 Nginx，业务零中断。
  - **端口自适应**：在网页端或 CLI 修改面板服务端口时，程序会**自动更新 Nginx 反代配置并平滑重载**，无需手动修改配置文件。
- ⚡ **多内核自由切换**：支持原生 Trojan 与更高性能的 Trojan-Go 内核在线一键无缝切换。
- 📊 **多维度监控**：实时查看系统负载、CPU/内存/磁盘资源趋势，以及详细的流量限制、月度/日度流量消耗排行榜。
- 🔗 **便捷分享与订阅**：支持一键生成 `trojan://` 链接与二维码分享，内置快速导出为 Clash 订阅的功能。
- 📦 **云端自动打包**：集成 GitHub Actions，自动拉取编译前端静态资源，并将其以二进制格式打包嵌入 Go 后端中，即开即用。

---

## 🛠️ 安装部署

> [!IMPORTANT]
> 部署前，请确保您的域名解析已正确生效，且服务器的 80/443 端口未被其他非 Nginx 服务占用。

### 方式 A：命令行一键脚本安装（推荐）

通过以下一键命令可以完成面板的安装与后续更新：

```bash
# 安装 / 更新面板程序
source <(curl -sL https://raw.githubusercontent.com/akudamatata/Trojan/master/install.sh)

# 卸载面板程序
source <(curl -sL https://raw.githubusercontent.com/akudamatata/Trojan/master/install.sh) --remove
```
* 安装完成后，在终端直接输入 `trojan` 即可调出命令行管理菜单。
* 浏览器访问 `https://您的域名` 即可登录 Web 后台进行用户管理。

---

### 方式 B：基于 Docker 容器部署

#### 1. 部署数据库 (推荐使用 MariaDB)
```bash
docker run --name trojan-mariadb \
  --restart=always \
  -p 3306:3306 \
  -v /home/mariadb:/var/lib/mysql \
  -e MYSQL_ROOT_PASSWORD=trojan \
  -e MYSQL_ROOT_HOST=% \
  -e MYSQL_DATABASE=trojan \
  -d mariadb:10.2
```

#### 2. 初始化安装 Trojan
```bash
docker run -it -d --name trojan --net=host --restart=always --privileged akudamatata/trojan init
```
* 启动完毕后，进入容器运行初始化：`docker exec -it trojan bash`，在容器内键入 `trojan` 进行配置。
* **管理面板控制命令**：
  ```bash
  systemctl start trojan-web   # 启动网页端
  systemctl enable trojan-web  # 开启自启动
  ```

---

## 🧭 双域名分流与伪装站配置指引

此功能支持在单台服务器上：使用域名 `a.com` 访问普通的伪装站，而使用域名 `b.com` 作为你独享的代理连接服务器与面板后台管理地址。

### 第一步：避让宿主机 80 端口
如果你的伪装站是通过 Docker 容器运行的，为了将 80 端口释放给宿主机的 Nginx 分流接管，请将伪装站容器的端口映射修改为非 80 端口（例如 `8080`）：
```yaml
# docker-compose.yml 示例
services:
  web:
    image: movie-site:latest
    ports:
      - "8080:80" # 将宿主机端口由 80 映射修改为 8080
```
修改完成后重启容器：`docker compose down && docker compose up -d`。

### 第二步：一键配置 TLS 证书与分流
在服务器终端中直接运行：
```bash
trojan tls
```
1. 提示输入**管理面板域名**（如 `b.com`）；
2. 提示输入**伪装站域名**（如 `a.com`，若不需要可回车跳过）；
3. 面板程序会自动申请双域名 SSL 证书，并生成 Nginx 分流配置文件。
4. 部署完成后，外部所有的 80/443 请求都将由 Nginx 进行智能过滤和代理，分流瞬间生效。

---

## 💡 常见问题与排查

> [!WARNING]
> **CentOS 系统下开启 SELinux 导致 502 Bad Gateway 修复**
> 如果证书与分流配置完成后，访问域名返回 **502 Bad Gateway**，且在 Nginx 错误日志 `/var/log/nginx/error.log` 中看到 `Permission denied` 报错，这是因为 CentOS 的 SELinux 默认阻断了 Nginx 连接本地其他非标准端口（如 8888 或 8080）。

**解决方案：**
在服务器终端直接运行以下两条命令放行即可：
```bash
# 允许 Nginx 进行网络反代连接
setsebool -P httpd_can_network_connect 1

# 重启 Nginx 使设置生效
systemctl restart nginx
```

---

## ⌨️ 命令行指令一览

在控制台输入 `trojan` 后，可以使用以下子命令：

| 指令 | 功能描述 |
| :--- | :--- |
| `trojan web` | 启动 Web 面板服务 |
| `trojan tls` | 自动申请证书并配置 Nginx 分流 |
| `trojan add` | 在终端快速添加用户 |
| `trojan del` | 删除用户 |
| `trojan info` | 显示当前的全部用户信息与流量列表 |
| `trojan log` | 实时监视并查看 Trojan 核心日志 |
| `trojan start` / `stop` / `restart` | 启动、停止或重启 Trojan 核心服务 |
| `trojan status` | 查看 Trojan 核心服务当前的运行状态 |
| `trojan port` | 修改 Trojan 代理连接端口 |
| `trojan update` | 一键更新 Trojan 核心核心程序 |
| `trojan updateWeb` | 更新 Trojan Web 管理程序 |
| `trojan export [path]` / `import [path]` | 备份导出 / 恢复导入用户 SQL 数据库 |

---

## ❤️ 鸣谢

- 感谢原作者 [Jrohy](https://github.com/Jrohy) 开发出的经典面板底座。
- 感谢 [JetBrains](https://www.jetbrains.com/) 提供优秀的 GoLand 集成开发工具支持。

[![JetBrains logo](asset/jetbrains.svg)](https://jb.gg/OpenSource)
