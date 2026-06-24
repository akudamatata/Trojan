package controller

import (
	"fmt"
	"github.com/robfig/cron/v3"
	"strconv"
	"time"
	"trojan/core"
	"trojan/trojan"
)

var c *cron.Cron

// SetData 设置流量限制
func SetData(id uint, quota int) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	mysql := core.GetMysql()
	if err := mysql.SetQuota(id, quota); err != nil {
		responseBody.Msg = err.Error()
	}
	return &responseBody
}

// CleanData 清空流量
func CleanData(id uint) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	mysql := core.GetMysql()
	if err := mysql.CleanData(id); err != nil {
		responseBody.Msg = err.Error()
	}
	return &responseBody
}

func monthlyResetJob() {
	mysql := core.GetMysql()
	if err := mysql.MonthlyResetData(); err != nil {
		fmt.Println("MonthlyResetError: " + err.Error())
	}
}

// GetResetDay 获取重置日
func GetResetDay() *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	dayStr, _ := core.GetValue("reset_day")
	day, _ := strconv.Atoi(dayStr)
	responseBody.Data = map[string]interface{}{
		"resetDay": day,
	}
	return &responseBody
}

// UpdateResetDay 更新重置流量日
func UpdateResetDay(day uint) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	if day > 31 || day < 0 {
		responseBody.Msg = fmt.Sprintf("%d为非正常日期", day)
		return &responseBody
	}
	dayStr, _ := core.GetValue("reset_day")
	oldDay, _ := strconv.Atoi(dayStr)
	if day == uint(oldDay) {
		return &responseBody
	}
	if len(c.Entries()) > 1 {
		c.Remove(c.Entries()[len(c.Entries())-1].ID)
	}
	if day != 0 {
		c.AddFunc(fmt.Sprintf("0 0 %d * *", day), func() {
			monthlyResetJob()
		})
	}
	core.SetValue("reset_day", strconv.Itoa(int(day)))
	return &responseBody
}

// ScheduleTask 定时任务
func ScheduleTask() {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	c = cron.New(cron.WithLocation(loc))
	c.AddFunc("@daily", func() {
		mysql := core.GetMysql()
		if needRestart, err := mysql.DailyCheckExpire(); err != nil {
			fmt.Println("DailyCheckError: " + err.Error())
		} else if needRestart {
			trojan.Restart()
		}
		if err := mysql.CleanOldUserLogs(); err != nil {
			fmt.Println("CleanOldUserLogs error: " + err.Error())
		}
	})

	// 每分钟计算并保存一次用户的每日流量消耗
	c.AddFunc("@every 1m", func() {
		aggregateUserDailyTraffic()
	})

	dayStr, _ := core.GetValue("reset_day")
	if dayStr == "" {
		dayStr = "1"
		core.SetValue("reset_day", dayStr)
	}
	day, _ := strconv.Atoi(dayStr)
	if day != 0 {
		c.AddFunc(fmt.Sprintf("0 0 %d * *", day), func() {
			monthlyResetJob()
		})
	}
	c.Start()
}

func aggregateUserDailyTraffic() {
	mysql := core.GetMysql()
	userList, err := mysql.GetData()
	if err != nil {
		return
	}

	todayStr := time.Now().Format("2006-01-02")

	for _, user := range userList {
		username := user.Username
		upKey := "last_traffic_up_" + username
		downKey := "last_traffic_down_" + username

		// 读取上一次记录的累积流量 (LevelDB)
		lastUpStr, _ := core.GetValue(upKey)
		lastDownStr, _ := core.GetValue(downKey)

		var lastUp, lastDown uint64
		if lastUpStr != "" {
			fmt.Sscanf(lastUpStr, "%d", &lastUp)
		} else {
			// 第一次跑或者冷启动，以当前数据库的值为 baseline
			core.SetValue(upKey, strconv.FormatUint(user.Upload, 10))
			lastUp = user.Upload
		}
		if lastDownStr != "" {
			fmt.Sscanf(lastDownStr, "%d", &lastDown)
		} else {
			core.SetValue(downKey, strconv.FormatUint(user.Download, 10))
			lastDown = user.Download
		}

		// 计算差值
		var deltaUp, deltaDown uint64
		if user.Upload >= lastUp {
			deltaUp = user.Upload - lastUp
		} else {
			// 说明流量被管理员手动重置了
			deltaUp = user.Upload
		}

		if user.Download >= lastDown {
			deltaDown = user.Download - lastDown
		} else {
			// 手动重置了
			deltaDown = user.Download
		}

		if deltaUp > 0 || deltaDown > 0 {
			// 保存差额到每日表
			err := mysql.SaveUserTrafficDaily(username, todayStr, deltaUp, deltaDown)
			if err == nil {
				// 更新 LevelDB 中的 baseline
				core.SetValue(upKey, strconv.FormatUint(user.Upload, 10))
				core.SetValue(downKey, strconv.FormatUint(user.Download, 10))
			}
		}
	}
}

// SetTotalQuota 设置服务器总流量限制
func SetTotalQuota(quota int64) *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	err := core.SetValue("server_total_quota", strconv.FormatInt(quota, 10))
	if err != nil {
		responseBody.Msg = err.Error()
	}
	return &responseBody
}

// GetTotalQuota 获取服务器总流量限制
func GetTotalQuota() *ResponseBody {
	responseBody := ResponseBody{Msg: "success"}
	defer TimeCost(time.Now(), &responseBody)
	quotaStr, _ := core.GetValue("server_total_quota")
	if quotaStr == "" {
		quotaStr = "-1"
	}
	quota, _ := strconv.ParseInt(quotaStr, 10, 64)
	responseBody.Data = map[string]interface{}{
		"totalQuota": quota,
	}
	return &responseBody
}
