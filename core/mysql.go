package core

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"io"
	"log"
	"time"

	"strconv"
	"strings"

	// mysql sql驱动
	_ "github.com/go-sql-driver/mysql"
)

// Mysql 结构体
type Mysql struct {
	Enabled    bool   `json:"enabled"`
	ServerAddr string `json:"server_addr"`
	ServerPort int    `json:"server_port"`
	Database   string `json:"database"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Cafile     string `json:"cafile"`
}

// User 用户表记录结构体
type User struct {
	ID          uint
	Username    string
	Password    string
	EncryptPass string
	Quota       int64
	Download    uint64
	Upload      uint64
	UseDays     uint
	ExpiryDate  string
}

// PageQuery 分页查询的结构体
type PageQuery struct {
	PageNum  int
	CurPage  int
	Total    int
	PageSize int
	DataList []*User
}

// CreateTableSql 创表sql
var CreateTableSql = `
CREATE TABLE IF NOT EXISTS users (
    id INT UNSIGNED NOT NULL AUTO_INCREMENT,
    username VARCHAR(64) NOT NULL,
    password CHAR(56) NOT NULL,
    passwordShow VARCHAR(255) NOT NULL,
    quota BIGINT NOT NULL DEFAULT 0,
    download BIGINT UNSIGNED NOT NULL DEFAULT 0,
    upload BIGINT UNSIGNED NOT NULL DEFAULT 0,
    useDays int(10) DEFAULT 0,
    expiryDate char(10) DEFAULT '',
    PRIMARY KEY (id),
    INDEX (password)
) DEFAULT CHARSET=utf8mb4;
`

var CreateUserIpsTableSql = `
CREATE TABLE IF NOT EXISTS user_ips (
    username VARCHAR(64) NOT NULL,
    client_ip VARCHAR(45) NOT NULL,
    last_connected_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    country VARCHAR(64) NOT NULL DEFAULT '',
    region VARCHAR(64) NOT NULL DEFAULT '',
    city VARCHAR(64) NOT NULL DEFAULT '',
    isp VARCHAR(128) NOT NULL DEFAULT '',
    PRIMARY KEY (username, client_ip)
) DEFAULT CHARSET=utf8mb4;
`

// AlterUserIpsGeoSql 为已存在的 user_ips 表追加 GeoIP 缓存列（不存在则添加，已存在则忽略报错）
var AlterUserIpsGeoSql = []string{
	"ALTER TABLE user_ips ADD COLUMN country VARCHAR(64) NOT NULL DEFAULT ''",
	"ALTER TABLE user_ips ADD COLUMN region VARCHAR(64) NOT NULL DEFAULT ''",
	"ALTER TABLE user_ips ADD COLUMN city VARCHAR(64) NOT NULL DEFAULT ''",
	"ALTER TABLE user_ips ADD COLUMN isp VARCHAR(128) NOT NULL DEFAULT ''",
}

var CreateUserDomainsTableSql = `
CREATE TABLE IF NOT EXISTS user_domains (
    username VARCHAR(64) NOT NULL,
    domain VARCHAR(255) NOT NULL,
    visit_count INT NOT NULL DEFAULT 1,
    last_visited_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (username, domain)
) DEFAULT CHARSET=utf8mb4;
`

var CreateUserTrafficDailyTableSql = `
CREATE TABLE IF NOT EXISTS user_traffic_daily (
    username VARCHAR(64) NOT NULL,
    date DATE NOT NULL,
    upload BIGINT UNSIGNED NOT NULL DEFAULT 0,
    download BIGINT UNSIGNED NOT NULL DEFAULT 0,
    PRIMARY KEY (username, date)
) DEFAULT CHARSET=utf8mb4;
`

var CreateUserSubLogsTableSql = `
CREATE TABLE IF NOT EXISTS user_sub_logs (
    id INT UNSIGNED NOT NULL AUTO_INCREMENT,
    username VARCHAR(64) NOT NULL,
    client_ip VARCHAR(45) NOT NULL,
    user_agent VARCHAR(255) NOT NULL DEFAULT '',
    country VARCHAR(64) NOT NULL DEFAULT '',
    region VARCHAR(64) NOT NULL DEFAULT '',
    city VARCHAR(64) NOT NULL DEFAULT '',
    isp VARCHAR(128) NOT NULL DEFAULT '',
    accessed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX (username),
    INDEX (accessed_at)
) DEFAULT CHARSET=utf8mb4;
`

// GetDB 获取mysql数据库连接
func (mysql *Mysql) GetDB() *sql.DB {
	// 屏蔽mysql驱动包 of 日志输出
	mysqlDriver.SetLogger(log.New(io.Discard, "", 0))
	conn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", mysql.Username, mysql.Password, mysql.ServerAddr, mysql.ServerPort, mysql.Database)
	db, err := sql.Open("mysql", conn)
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}
	return db
}

// CreateTable 不存在trojan user表则自动创建
func (mysql *Mysql) CreateTable() {
	db := mysql.GetDB()
	if db == nil {
		return
	}
	defer db.Close()
	if _, err := db.Exec(CreateTableSql); err != nil {
		fmt.Println("CreateTableSql error:", err)
	}
	if _, err := db.Exec(CreateUserIpsTableSql); err != nil {
		fmt.Println("CreateUserIpsTableSql error:", err)
	}
	// 对已存在的旧表，尝试追加 GeoIP 缓存列（列已存在时 MySQL 会报 Duplicate column 错误，直接忽略）
	for _, alterSql := range AlterUserIpsGeoSql {
		db.Exec(alterSql)
	}
	if _, err := db.Exec(CreateUserDomainsTableSql); err != nil {
		fmt.Println("CreateUserDomainsTableSql error:", err)
	}
	if _, err := db.Exec(CreateUserTrafficDailyTableSql); err != nil {
		fmt.Println("CreateUserTrafficDailyTableSql error:", err)
	}
	if _, err := db.Exec(CreateUserSubLogsTableSql); err != nil {
		fmt.Println("CreateUserSubLogsTableSql error:", err)
	}
}

func queryUserList(db *sql.DB, sql string) ([]*User, error) {
	var (
		username    string
		encryptPass string
		passShow    string
		download    uint64
		upload      uint64
		quota       int64
		id          uint
		useDays     uint
		expiryDate  string
	)
	var userList []*User
	rows, err := db.Query(sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(&id, &username, &encryptPass, &passShow, &quota, &download, &upload, &useDays, &expiryDate); err != nil {
			return nil, err
		}
		userList = append(userList, &User{
			ID:          id,
			Username:    username,
			Password:    passShow,
			EncryptPass: encryptPass,
			Download:    download,
			Upload:      upload,
			Quota:       quota,
			UseDays:     useDays,
			ExpiryDate:  expiryDate,
		})
	}
	return userList, nil
}

func queryUser(db *sql.DB, sql string) (*User, error) {
	var (
		username    string
		encryptPass string
		passShow    string
		download    uint64
		upload      uint64
		quota       int64
		id          uint
		useDays     uint
		expiryDate  string
	)
	row := db.QueryRow(sql)
	if err := row.Scan(&id, &username, &encryptPass, &passShow, &quota, &download, &upload, &useDays, &expiryDate); err != nil {
		return nil, err
	}
	return &User{ID: id, Username: username, Password: passShow, EncryptPass: encryptPass, Download: download, Upload: upload, Quota: quota, UseDays: useDays, ExpiryDate: expiryDate}, nil
}

// CreateUser 创建Trojan用户
func (mysql *Mysql) CreateUser(username string, base64Pass string, originPass string) error {
	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	encryPass := sha256.Sum224([]byte(originPass))
	if _, err := db.Exec(fmt.Sprintf("INSERT INTO users(username, password, passwordShow, quota) VALUES ('%s', '%x', '%s', -1);", username, encryPass, base64Pass)); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

// UpdateUser 更新Trojan用户名和密码
func (mysql *Mysql) UpdateUser(id uint, username string, base64Pass string, originPass string) error {
	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	encryPass := sha256.Sum224([]byte(originPass))
	if _, err := db.Exec(fmt.Sprintf("UPDATE users SET username='%s', password='%x', passwordShow='%s' WHERE id=%d;", username, encryPass, base64Pass, id)); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

// DeleteUser 删除用户
func (mysql *Mysql) DeleteUser(id uint) error {
	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	if userList, err := mysql.GetData(strconv.Itoa(int(id))); err != nil {
		return err
	} else if userList != nil && len(userList) == 0 {
		return fmt.Errorf("不存在id为%d的用户", id)
	}
	if _, err := db.Exec(fmt.Sprintf("DELETE FROM users WHERE id=%d;", id)); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

// MonthlyResetData 设置了过期时间的用户，每月定时清空使用流量
func (mysql *Mysql) MonthlyResetData() error {
	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	userList, err := queryUserList(db, "SELECT * FROM users WHERE useDays != 0 AND quota != 0")
	if err != nil {
		return err
	}
	for _, user := range userList {
		if _, err := db.Exec(fmt.Sprintf("UPDATE users SET download=0, upload=0 WHERE id=%d;", user.ID)); err != nil {
			return err
		}
	}
	return nil
}

// DailyCheckExpire 检查是否有过期，过期了设置流量上限为0
func (mysql *Mysql) DailyCheckExpire() (bool, error) {
	needRestart := false
	now := time.Now()
	utc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return false, err
	}
	addDay, _ := time.ParseDuration("-24h")
	yesterdayStr := now.Add(addDay).In(utc).Format("2006-01-02")
	yesterday, _ := time.Parse("2006-01-02", yesterdayStr)
	db := mysql.GetDB()
	if db == nil {
		return false, errors.New("can't connect mysql")
	}
	defer db.Close()
	userList, err := queryUserList(db, "SELECT * FROM users WHERE quota != 0")
	if err != nil {
		return false, err
	}
	for _, user := range userList {
		if expireDate, err := time.Parse("2006-01-02", user.ExpiryDate); err == nil {
			if yesterday.Sub(expireDate).Seconds() >= 0 {
				if _, err := db.Exec(fmt.Sprintf("UPDATE users SET quota=0 WHERE id=%d;", user.ID)); err != nil {
					return false, err
				}
				if !needRestart {
					needRestart = true
				}
			}
		}
	}
	return needRestart, nil
}

// CancelExpire 取消过期时间
func (mysql *Mysql) CancelExpire(id uint) error {
	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	if _, err := db.Exec(fmt.Sprintf("UPDATE users SET useDays=0, expiryDate='' WHERE id=%d;", id)); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

// SetExpire 设置过期时间
func (mysql *Mysql) SetExpire(id uint, useDays uint) error {
	now := time.Now()
	utc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		fmt.Println(err)
		return err
	}
	addDay, _ := time.ParseDuration(strconv.Itoa(int(24*useDays)) + "h")
	expiryDate := now.Add(addDay).In(utc).Format("2006-01-02")

	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	if _, err := db.Exec(fmt.Sprintf("UPDATE users SET useDays=%d, expiryDate='%s' WHERE id=%d;", useDays, expiryDate, id)); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

// SetQuota 限制流量
func (mysql *Mysql) SetQuota(id uint, quota int) error {
	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	if _, err := db.Exec(fmt.Sprintf("UPDATE users SET quota=%d WHERE id=%d;", quota, id)); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

// CleanData 清空流量统计
func (mysql *Mysql) CleanData(id uint) error {
	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	if _, err := db.Exec(fmt.Sprintf("UPDATE users SET download=0, upload=0 WHERE id=%d;", id)); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

// CleanDataByName 清空指定用户名流量统计数据
func (mysql *Mysql) CleanDataByName(usernames []string) error {
	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	runSql := "UPDATE users SET download=0, upload=0 WHERE BINARY username in ("
	for i, name := range usernames {
		runSql = runSql + "'" + name + "'"
		if i == len(usernames)-1 {
			runSql = runSql + ")"
		} else {
			runSql = runSql + ","
		}
	}
	if _, err := db.Exec(runSql); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

// GetUserByName 通过用户名来获取用户
func (mysql *Mysql) GetUserByName(name string) *User {
	db := mysql.GetDB()
	if db == nil {
		return nil
	}
	defer db.Close()
	user, err := queryUser(db, fmt.Sprintf("SELECT * FROM users WHERE BINARY username='%s'", name))
	if err != nil {
		return nil
	}
	return user
}

// GetUserByPass 通过密码来获取用户
func (mysql *Mysql) GetUserByPass(pass string) *User {
	db := mysql.GetDB()
	if db == nil {
		return nil
	}
	defer db.Close()
	user, err := queryUser(db, fmt.Sprintf("SELECT * FROM users WHERE BINARY passwordShow='%s'", pass))
	if err != nil {
		return nil
	}
	return user
}

// PageList 通过分页获取用户记录
func (mysql *Mysql) PageList(curPage int, pageSize int) (*PageQuery, error) {
	var (
		total int
	)

	db := mysql.GetDB()
	if db == nil {
		return nil, errors.New("连接mysql失败")
	}
	defer db.Close()
	offset := (curPage - 1) * pageSize
	querySQL := fmt.Sprintf("SELECT * FROM users LIMIT %d, %d", offset, pageSize)
	userList, err := queryUserList(db, querySQL)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	db.QueryRow("SELECT COUNT(id) FROM users").Scan(&total)
	return &PageQuery{
		CurPage:  curPage,
		PageSize: pageSize,
		Total:    total,
		DataList: userList,
		PageNum:  (total + pageSize - 1) / pageSize,
	}, nil
}

// GetData 获取用户记录
func (mysql *Mysql) GetData(ids ...string) ([]*User, error) {
	querySQL := "SELECT * FROM users"
	db := mysql.GetDB()
	if db == nil {
		return nil, errors.New("连接mysql失败")
	}
	defer db.Close()
	if len(ids) > 0 {
		querySQL = querySQL + " WHERE id in (" + strings.Join(ids, ",") + ")"
	}
	userList, err := queryUserList(db, querySQL)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return userList, nil
}

// GetTotalUsedTraffic 获取所有用户的已用总流量 (upload + download)
func (mysql *Mysql) GetTotalUsedTraffic() (uint64, error) {
	db := mysql.GetDB()
	if db == nil {
		return 0, errors.New("can't connect mysql")
	}
	defer db.Close()
	var total sql.NullInt64
	err := db.QueryRow("SELECT SUM(upload + download) FROM users").Scan(&total)
	if err != nil {
		return 0, err
	}
	if total.Valid {
		return uint64(total.Int64), nil
	}
	return 0, nil
}

// GetTop10Users 获取流量排名前10的用户列表
func (mysql *Mysql) GetTop10Users() ([]*User, error) {
	db := mysql.GetDB()
	if db == nil {
		return nil, errors.New("can't connect mysql")
	}
	defer db.Close()
	querySQL := "SELECT id, username, password, passwordShow, quota, download, upload, useDays, expiryDate FROM users ORDER BY (upload + download) DESC LIMIT 10"
	userList, err := queryUserList(db, querySQL)
	if err != nil {
		return nil, err
	}
	return userList, nil
}

// UserIPInfo 结构体，用于返回用户 IP 连接信息（含缓存的 GeoIP 数据）
type UserIPInfo struct {
	IP      string `json:"ip"`
	Country string `json:"country"`
	Region  string `json:"region"`
	City    string `json:"city"`
	ISP     string `json:"isp"`
}

// UserDomain 结构体，用于返回域名的访问数据
type UserDomain struct {
	Domain     string `json:"domain"`
	VisitCount int    `json:"visit_count"`
}

// SaveUserIP 保存或更新用户的连入 IP (只保留最近30天)
func (mysql *Mysql) SaveUserIP(username string, clientIP string) error {
	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	_, err := db.Exec(`
		INSERT INTO user_ips (username, client_ip, last_connected_at) 
		VALUES (?, ?, NOW()) 
		ON DUPLICATE KEY UPDATE last_connected_at = NOW()
	`, username, clientIP)
	return err
}

// SaveUserDomain 增加或更新用户域名的访问统计
func (mysql *Mysql) SaveUserDomain(username string, domain string) error {
	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	_, err := db.Exec(`
		INSERT INTO user_domains (username, domain, visit_count, last_visited_at) 
		VALUES (?, ?, 1, NOW()) 
		ON DUPLICATE KEY UPDATE visit_count = visit_count + 1, last_visited_at = NOW()
	`, username, domain)
	return err
}

// SaveUserDomainBatch 批量累加用户域名的访问统计（由 Daemon 定时刷新调用）
func (mysql *Mysql) SaveUserDomainBatch(username string, domain string, count int) error {
	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	_, err := db.Exec(`
		INSERT INTO user_domains (username, domain, visit_count, last_visited_at) 
		VALUES (?, ?, ?, NOW()) 
		ON DUPLICATE KEY UPDATE visit_count = visit_count + ?, last_visited_at = NOW()
	`, username, domain, count, count)
	return err
}

// GetUserIPs 获取用户最近一个月内连入过的 IP 列表（含缓存的 GeoIP 数据）
func (mysql *Mysql) GetUserIPs(username string) ([]UserIPInfo, error) {
	db := mysql.GetDB()
	if db == nil {
		return nil, errors.New("can't connect mysql")
	}
	defer db.Close()
	rows, err := db.Query(`
		SELECT client_ip, country, region, city, isp FROM user_ips 
		WHERE username = ? AND last_connected_at >= NOW() - INTERVAL 30 DAY 
		ORDER BY last_connected_at DESC
	`, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ips []UserIPInfo
	for rows.Next() {
		var info UserIPInfo
		if err := rows.Scan(&info.IP, &info.Country, &info.Region, &info.City, &info.ISP); err == nil {
			ips = append(ips, info)
		}
	}
	return ips, nil
}

// UpdateIPGeo 将前端查询到的 GeoIP 结果回写缓存到数据库
func (mysql *Mysql) UpdateIPGeo(username, clientIP, country, region, city, isp string) error {
	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	_, err := db.Exec(`
		UPDATE user_ips SET country = ?, region = ?, city = ?, isp = ? 
		WHERE username = ? AND client_ip = ?
	`, country, region, city, isp, username, clientIP)
	return err
}

// GetOnlineUsernames 根据当前活跃的客户端 IP 列表，反查数据库得到在线的用户名
func (mysql *Mysql) GetOnlineUsernames(activeIPs []string) []string {
	if len(activeIPs) == 0 {
		return []string{}
	}
	db := mysql.GetDB()
	if db == nil {
		return []string{}
	}
	defer db.Close()

	// 构建 IN (?, ?, ...) 参数
	placeholders := make([]string, len(activeIPs))
	args := make([]interface{}, len(activeIPs))
	for i, ip := range activeIPs {
		placeholders[i] = "?"
		args[i] = ip
	}
	query := fmt.Sprintf("SELECT DISTINCT username FROM user_ips WHERE client_ip IN (%s)", strings.Join(placeholders, ","))
	rows, err := db.Query(query, args...)
	if err != nil {
		return []string{}
	}
	defer rows.Close()

	var usernames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			usernames = append(usernames, name)
		}
	}
	return usernames
}

// GetUserTopDomains 获取用户最常访问的前 10 个域名
func (mysql *Mysql) GetUserTopDomains(username string, limit int) ([]UserDomain, error) {
	db := mysql.GetDB()
	if db == nil {
		return nil, errors.New("can't connect mysql")
	}
	defer db.Close()
	rows, err := db.Query(`
		SELECT domain, visit_count FROM user_domains 
		WHERE username = ? AND last_visited_at >= NOW() - INTERVAL 30 DAY 
		ORDER BY visit_count DESC LIMIT ?
	`, username, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []UserDomain
	for rows.Next() {
		var ud UserDomain
		if err := rows.Scan(&ud.Domain, &ud.VisitCount); err == nil {
			domains = append(domains, ud)
		}
	}
	return domains, nil
}

// CleanOldUserLogs 清理 30 天以前的旧连接与访问数据
func (mysql *Mysql) CleanOldUserLogs() error {
	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	_, errIP := db.Exec("DELETE FROM user_ips WHERE last_connected_at < NOW() - INTERVAL 30 DAY")
	_, errDomain := db.Exec("DELETE FROM user_domains WHERE last_visited_at < NOW() - INTERVAL 30 DAY")
	if errIP != nil {
		return errIP
	}
	return errDomain
}

// UserTrafficDaily 结构体，用于返回用户每日流量记录
type UserTrafficDaily struct {
	Date     string `json:"date"`
	Upload   uint64 `json:"upload"`
	Download uint64 `json:"download"`
}

// UserSubLog 结构体，用于返回用户订阅日志
type UserSubLog struct {
	ID         uint   `json:"id"`
	ClientIP   string `json:"client_ip"`
	UserAgent  string `json:"user_agent"`
	Country    string `json:"country"`
	Region     string `json:"region"`
	City       string `json:"city"`
	ISP        string `json:"isp"`
	AccessedAt string `json:"accessed_at"`
}

// SaveUserTrafficDaily 保存或累加每日流量
func (mysql *Mysql) SaveUserTrafficDaily(username string, date string, upload uint64, download uint64) error {
	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	_, err := db.Exec(`
		INSERT INTO user_traffic_daily (username, date, upload, download) 
		VALUES (?, ?, ?, ?) 
		ON DUPLICATE KEY UPDATE upload = upload + ?, download = download + ?
	`, username, date, upload, download, upload, download)
	return err
}

// GetUserTrafficHistory 获取用户最近 limit 天的流量记录
func (mysql *Mysql) GetUserTrafficHistory(username string, limit int) ([]UserTrafficDaily, error) {
	db := mysql.GetDB()
	if db == nil {
		return nil, errors.New("can't connect mysql")
	}
	defer db.Close()
	rows, err := db.Query(`
		SELECT DATE_FORMAT(date, '%m-%d') as fmt_date, upload, download FROM user_traffic_daily 
		WHERE username = ? 
		ORDER BY date DESC LIMIT ?
	`, username, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []UserTrafficDaily
	for rows.Next() {
		var item UserTrafficDaily
		if err := rows.Scan(&item.Date, &item.Upload, &item.Download); err == nil {
			list = append(list, item)
		}
	}
	// 反转列表，使之按日期升序排列，方便图表展示
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return list, nil
}

// SaveUserSubLog 保存订阅拉取日志
func (mysql *Mysql) SaveUserSubLog(username, clientIP, userAgent, country, region, city, isp string) error {
	db := mysql.GetDB()
	if db == nil {
		return errors.New("can't connect mysql")
	}
	defer db.Close()
	_, err := db.Exec(`
		INSERT INTO user_sub_logs (username, client_ip, user_agent, country, region, city, isp, accessed_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, NOW())
	`, username, clientIP, userAgent, country, region, city, isp)
	return err
}

// GetUserSubLogs 获取订阅链接访问日志
func (mysql *Mysql) GetUserSubLogs(username string, limit int) ([]UserSubLog, error) {
	db := mysql.GetDB()
	if db == nil {
		return nil, errors.New("can't connect mysql")
	}
	defer db.Close()
	rows, err := db.Query(`
		SELECT id, client_ip, user_agent, country, region, city, isp, 
		       DATE_FORMAT(accessed_at, '%Y-%m-%d %H:%i:%s') as fmt_time 
		FROM user_sub_logs 
		WHERE username = ? 
		ORDER BY accessed_at DESC LIMIT ?
	`, username, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []UserSubLog
	for rows.Next() {
		var item UserSubLog
		if err := rows.Scan(&item.ID, &item.ClientIP, &item.UserAgent, &item.Country, &item.Region, &item.City, &item.ISP, &item.AccessedAt); err == nil {
			list = append(list, item)
		}
	}
	return list, nil
}
