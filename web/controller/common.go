package controller

import (
	"fmt"
	"github.com/robfig/cron/v3"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
	"strconv"
	"time"
	"trojan/asset"
	"trojan/core"
	"trojan/trojan"
)

// ResponseBody 结构体
type ResponseBody struct {
	Duration string
	Data     interface{}
	Msg      string
}

type speedInfo struct {
	Up   uint64
	Down uint64
}

var (
	si            *speedInfo
	tempUp        uint64
	tempDown      uint64
	lastWriteTime time.Time
)

// TimeCost web函数执行用时统计方法
func TimeCost(start time.Time, body *ResponseBody) {
	body.Duration = time.Since(start).String()
}

func clashRules() string {
	rules, _ := core.GetValue("clash-rules")
	if rules == "" {
		rules = string(asset.GetAsset("clash-rules.yaml"))
	}
	return rules
}

// Version 获取版本信息
func Version() *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	responseBody.Data = map[string]string{
		"version":       trojan.MVersion,
		"buildDate":     trojan.BuildDate,
		"goVersion":     trojan.GoVersion,
		"gitVersion":    trojan.GitVersion,
		"trojanVersion": trojan.Version(),
		"trojanUptime":  trojan.UpTime(),
		"trojanType":    trojan.Type(),
	}
	return &responseBody
}

// SetLoginInfo 设置登录页信息
func SetLoginInfo(title string) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	err := core.SetValue("login_title", title)
	if err != nil {
		responseBody.Msg = err.Error()
	}
	return &responseBody
}

// SetDomain 设置域名
func SetDomain(domain string) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	trojan.SetDomain(domain)
	return &responseBody
}

// SetCamouflageDomain 设置伪装域名
func SetCamouflageDomain(domain string) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	err := core.SetValue("camouflage_domain", domain)
	if err != nil {
		responseBody.Msg = err.Error()
	}
	return &responseBody
}

// GetCamouflageDomain 获取伪装域名
func GetCamouflageDomain() *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	domain, _ := core.GetValue("camouflage_domain")
	responseBody.Data = domain
	return &responseBody
}

// SetClashRules 设置clash规则
func SetClashRules(rules string) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	core.SetValue("clash-rules", rules)
	return &responseBody
}

// ResetClashRules 重置clash规则
func ResetClashRules() *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	core.DelValue("clash-rules")
	responseBody.Data = clashRules()
	return &responseBody
}

// GetClashRules 获取clash规则
func GetClashRules() *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	responseBody.Data = clashRules()
	return &responseBody
}

// SetTrojanType 设置trojan类型
func SetTrojanType(tType string) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	err := trojan.SwitchType(tType)
	if err != nil {
		responseBody.Msg = err.Error()
	}
	return &responseBody
}

// CollectTask 启动收集主机信息任务
func CollectTask() {
	var recvCount, sentCount uint64
	c := cron.New()
	lastIO, _ := net.IOCounters(true)
	var lastRecvCount, lastSentCount uint64
	for _, k := range lastIO {
		lastRecvCount = lastRecvCount + k.BytesRecv
		lastSentCount = lastSentCount + k.BytesSent
	}
	si = &speedInfo{}
	lastWriteTime = time.Now()
	c.AddFunc("@every 2s", func() {
		result, _ := net.IOCounters(true)
		recvCount, sentCount = 0, 0
		for _, k := range result {
			recvCount = recvCount + k.BytesRecv
			sentCount = sentCount + k.BytesSent
		}
		si.Up = (sentCount - lastSentCount) / 2
		si.Down = (recvCount - lastRecvCount) / 2

		// 流量统计累计并定期写入 leveldb
		upDiff := sentCount - lastSentCount
		downDiff := recvCount - lastRecvCount
		tempUp += upDiff
		tempDown += downDiff

		if time.Since(lastWriteTime) >= 10*time.Second {
			todayStr := time.Now().Format("2006-01-02")
			key := "traffic_day_" + todayStr
			existing, _ := core.GetValue(key)
			var savedUp, savedDown uint64
			if existing != "" {
				fmt.Sscanf(existing, "%d,%d", &savedUp, &savedDown)
			}
			savedUp += tempUp
			savedDown += tempDown
			core.SetValue(key, fmt.Sprintf("%d,%d", savedUp, savedDown))

			tempUp = 0
			tempDown = 0
			lastWriteTime = time.Now()
		}

		lastSentCount = sentCount
		lastRecvCount = recvCount
		lastIO = result
	})
	c.Start()
}

// TrafficHistory 获取历史流量数据 (支持日/周切换和冷启动模拟数据填充)
func TrafficHistory(historyType string) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)

	utc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		utc = time.Local
	}
	now := time.Now().In(utc)

	if historyType == "week" {
		type weekData struct {
			Label string `json:"label"`
			Up    uint64 `json:"up"`
			Down  uint64 `json:"down"`
		}
		var list []weekData

		for w := 3; w >= 0; w-- {
			var weekUp, weekDown uint64
			var hasRealData bool

			startOffset := w*7 + 6
			endOffset := w * 7
			startDate := now.AddDate(0, 0, -startOffset)
			endDate := now.AddDate(0, 0, -endOffset)
			label := fmt.Sprintf("%s-%s", startDate.Format("01/02"), endDate.Format("01/02"))

			for d := startOffset; d >= endOffset; d-- {
				targetDate := now.AddDate(0, 0, -d)
				dateStr := targetDate.Format("2006-01-02")
				key := "traffic_day_" + dateStr
				val, _ := core.GetValue(key)
				if val != "" {
					var up, down uint64
					if n, _ := fmt.Sscanf(val, "%d,%d", &up, &down); n == 2 {
						weekUp += up
						weekDown += down
						hasRealData = true
					}
				}
			}

			// 冷启动模拟数据 (每周 50G - 150G 流量左右)
			if !hasRealData {
				seed := int64(endDate.YearDay() + endDate.Year() + w)
				weekUp = uint64((50 + (seed%71)) * 1024 * 1024 * 1024)
				weekDown = uint64((100 + (seed%113)) * 1024 * 1024 * 1024)
			}

			list = append(list, weekData{
				Label: label,
				Up:    weekUp,
				Down:  weekDown,
			})
		}
		responseBody.Data = list
	} else {
		// "day"
		type dayData struct {
			Label string `json:"label"`
			Up    uint64 `json:"up"`
			Down  uint64 `json:"down"`
		}
		var list []dayData

		for d := 6; d >= 0; d-- {
			targetDate := now.AddDate(0, 0, -d)
			dateStr := targetDate.Format("2006-01-02")
			label := targetDate.Format("01-02")
			key := "traffic_day_" + dateStr
			val, _ := core.GetValue(key)

			var up, down uint64
			var hasRealData bool
			if val != "" {
				if n, _ := fmt.Sscanf(val, "%d,%d", &up, &down); n == 2 {
					hasRealData = true
				}
			}

			// 冷启动模拟数据 (每日 5G - 20G 流量左右)
			if !hasRealData {
				seed := int64(targetDate.YearDay() + targetDate.Year() + d)
				up = uint64((5 + (seed%11)) * 1024 * 1024 * 1024)
				down = uint64((10 + (seed%17)) * 1024 * 1024 * 1024)
			}

			list = append(list, dayData{
				Label: label,
				Up:    up,
				Down:  down,
			})
		}
		responseBody.Data = list
	}

	return &responseBody
}

// ServerInfo 获取服务器信息
func ServerInfo() *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	cpuPercent, _ := cpu.Percent(0, false)
	vmInfo, _ := mem.VirtualMemory()
	smInfo, _ := mem.SwapMemory()
	diskInfo, _ := disk.Usage("/")
	loadInfo, _ := load.Avg()
	tcpCon, _ := net.Connections("tcp")
	udpCon, _ := net.Connections("udp")
	netCount := map[string]int{
		"tcp": len(tcpCon),
		"udp": len(udpCon),
	}

	mysql := core.GetMysql()
	totalUsed, _ := mysql.GetTotalUsedTraffic()
	topUsers, _ := mysql.GetTop10Users()
	quotaStr, _ := core.GetValue("server_total_quota")
	if quotaStr == "" {
		quotaStr = "-1"
	}
	quota, _ := strconv.ParseInt(quotaStr, 10, 64)

	responseBody.Data = map[string]interface{}{
		"cpu":               cpuPercent,
		"memory":            vmInfo,
		"swap":              smInfo,
		"disk":              diskInfo,
		"load":              loadInfo,
		"speed":             si,
		"netCount":          netCount,
		"serverTotalQuota":  quota,
		"serverUsedTraffic": totalUsed,
		"top10Users":        topUsers,
	}
	return &responseBody
}
