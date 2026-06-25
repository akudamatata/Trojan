package web

import (
	"embed"
	"fmt"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"io/fs"
	"net/http"
	"strconv"
	"trojan/core"
	"trojan/util"
	"trojan/web/controller"
)

//go:embed templates/*
var f embed.FS

func userRouter(router *gin.Engine) {
	user := router.Group("/trojan/user")
	{
		user.GET("", func(c *gin.Context) {
			requestUser := RequestUsername(c)
			c.JSON(200, controller.UserList(requestUser))
		})
		user.GET("/detail", func(c *gin.Context) {
			username := c.Query("username")
			c.JSON(200, controller.UserDetail(username))
		})
		user.POST("/geo", func(c *gin.Context) {
			username := c.PostForm("username")
			ip := c.PostForm("ip")
			country := c.PostForm("country")
			region := c.PostForm("region")
			city := c.PostForm("city")
			isp := c.PostForm("isp")
			c.JSON(200, controller.SaveIPGeo(username, ip, country, region, city, isp))
		})
		user.GET("/page", func(c *gin.Context) {
			curPageStr := c.DefaultQuery("curPage", "1")
			pageSizeStr := c.DefaultQuery("pageSize", "10")
			curPage, _ := strconv.Atoi(curPageStr)
			pageSize, _ := strconv.Atoi(pageSizeStr)
			c.JSON(200, controller.PageUserList(curPage, pageSize))
		})
		user.POST("", func(c *gin.Context) {
			username := c.PostForm("username")
			password := c.PostForm("password")
			c.JSON(200, controller.CreateUser(username, password))
		})
		user.POST("/update", func(c *gin.Context) {
			sid := c.PostForm("id")
			username := c.PostForm("username")
			password := c.PostForm("password")
			id, _ := strconv.Atoi(sid)
			c.JSON(200, controller.UpdateUser(uint(id), username, password))
		})
		user.POST("/expire", func(c *gin.Context) {
			sid := c.PostForm("id")
			sDays := c.PostForm("useDays")
			id, _ := strconv.Atoi(sid)
			useDays, _ := strconv.Atoi(sDays)
			c.JSON(200, controller.SetExpire(uint(id), uint(useDays)))
		})
		user.DELETE("/expire", func(c *gin.Context) {
			sid := c.Query("id")
			id, _ := strconv.Atoi(sid)
			c.JSON(200, controller.CancelExpire(uint(id)))
		})
		user.DELETE("", func(c *gin.Context) {
			stringId := c.Query("id")
			id, _ := strconv.Atoi(stringId)
			c.JSON(200, controller.DelUser(uint(id)))
		})
		user.GET("/active-connections", func(c *gin.Context) {
			username := c.Query("username")
			c.JSON(200, controller.GetActiveConnections(username))
		})
		user.POST("/active-connections/kill", func(c *gin.Context) {
			clientIP := c.PostForm("client_ip")
			clientPort := c.PostForm("client_port")
			c.JSON(200, controller.KillActiveConnection(clientIP, clientPort))
		})
		user.POST("/kill-by-ip", func(c *gin.Context) {
			clientIP := c.PostForm("client_ip")
			c.JSON(200, controller.KillConnectionsByIP(clientIP))
		})
		user.GET("/blacklist", func(c *gin.Context) {
			c.JSON(200, controller.GetIPBlacklist())
		})
		user.POST("/blacklist/ban", func(c *gin.Context) {
			ip := c.PostForm("ip")
			duration := c.PostForm("duration")
			c.JSON(200, controller.BanIP(ip, duration))
		})
		user.POST("/blacklist/unban", func(c *gin.Context) {
			ip := c.PostForm("ip")
			c.JSON(200, controller.UnbanIP(ip))
		})
		user.GET("/traffic-history", func(c *gin.Context) {
			username := c.Query("username")
			c.JSON(200, controller.GetUserTrafficHistory(username))
		})
		user.GET("/sub-logs", func(c *gin.Context) {
			username := c.Query("username")
			c.JSON(200, controller.GetUserSubLogs(username))
		})
		user.GET("/domain-stats", func(c *gin.Context) {
			username := c.Query("username")
			c.JSON(200, controller.GetUserDomainStats(username))
		})
	}
}

func trojanRouter(router *gin.Engine) {
	router.POST("/trojan/start", func(c *gin.Context) {
		c.JSON(200, controller.Start())
	})
	router.POST("/trojan/stop", func(c *gin.Context) {
		c.JSON(200, controller.Stop())
	})
	router.POST("/trojan/restart", func(c *gin.Context) {
		c.JSON(200, controller.Restart())
	})
	router.GET("/trojan/loglevel", func(c *gin.Context) {
		c.JSON(200, controller.GetLogLevel())
	})
	router.GET("/trojan/export", func(c *gin.Context) {
		result := controller.ExportCsv(c)
		if result != nil {
			c.JSON(200, result)
		}
	})
	router.POST("/trojan/import", func(c *gin.Context) {
		c.JSON(200, controller.ImportCsv(c))
	})
	router.POST("/trojan/update", func(c *gin.Context) {
		c.JSON(200, controller.Update())
	})
	router.POST("/trojan/switch", func(c *gin.Context) {
		tType := c.DefaultPostForm("type", "trojan")
		c.JSON(200, controller.SetTrojanType(tType))
	})
	router.POST("/trojan/loglevel", func(c *gin.Context) {
		slevel := c.DefaultPostForm("level", "1")
		level, _ := strconv.Atoi(slevel)
		c.JSON(200, controller.SetLogLevel(level))
	})
	router.POST("/trojan/domain", func(c *gin.Context) {
		c.JSON(200, controller.SetDomain(c.PostForm("domain")))
	})
	router.GET("/trojan/log", func(c *gin.Context) {
		controller.Log(c)
	})
	router.GET("/trojan/webport", func(c *gin.Context) {
		c.JSON(200, controller.GetWebPort())
	})
	router.POST("/trojan/webport", func(c *gin.Context) {
		c.JSON(200, controller.SetWebPort(c))
	})
}

func dataRouter(router *gin.Engine) {
	data := router.Group("/trojan/data")
	{
		data.POST("", func(c *gin.Context) {
			sID := c.PostForm("id")
			sQuota := c.PostForm("quota")
			id, _ := strconv.Atoi(sID)
			quota, _ := strconv.Atoi(sQuota)
			c.JSON(200, controller.SetData(uint(id), quota))
		})
		data.DELETE("", func(c *gin.Context) {
			sID := c.Query("id")
			id, _ := strconv.Atoi(sID)
			c.JSON(200, controller.CleanData(uint(id)))
		})
		data.POST("/resetDay", func(c *gin.Context) {
			dayStr := c.DefaultPostForm("day", "1")
			day, _ := strconv.Atoi(dayStr)
			c.JSON(200, controller.UpdateResetDay(uint(day)))
		})
		data.GET("/resetDay", func(c *gin.Context) {
			c.JSON(200, controller.GetResetDay())
		})
		data.POST("/totalQuota", func(c *gin.Context) {
			sQuota := c.PostForm("quota")
			quota, _ := strconv.ParseInt(sQuota, 10, 64)
			c.JSON(200, controller.SetTotalQuota(quota))
		})
		data.GET("/totalQuota", func(c *gin.Context) {
			c.JSON(200, controller.GetTotalQuota())
		})
	}
}

func commonRouter(router *gin.Engine) {
	common := router.Group("/common")
	{
		common.GET("/version", func(c *gin.Context) {
			c.JSON(200, controller.Version())
		})
		common.GET("/serverInfo", func(c *gin.Context) {
			c.JSON(200, controller.ServerInfo())
		})
		common.GET("/trafficHistory", func(c *gin.Context) {
			historyType := c.DefaultQuery("type", "day")
			c.JSON(200, controller.TrafficHistory(historyType))
		})
		common.GET("/clashRules", func(c *gin.Context) {
			c.JSON(200, controller.GetClashRules())
		})
		common.POST("/clashRules", func(c *gin.Context) {
			rules := c.PostForm("rules")
			c.JSON(200, controller.SetClashRules(rules))
		})
		common.DELETE("/clashRules", func(c *gin.Context) {
			c.JSON(200, controller.ResetClashRules())
		})
		common.POST("/loginInfo", func(c *gin.Context) {
			c.JSON(200, controller.SetLoginInfo(c.PostForm("title")))
		})
		common.GET("/camouflageDomain", func(c *gin.Context) {
			c.JSON(200, controller.GetCamouflageDomain())
		})
		common.POST("/camouflageDomain", func(c *gin.Context) {
			c.JSON(200, controller.SetCamouflageDomain(c.PostForm("domain")))
		})
		common.GET("/certInfo", func(c *gin.Context) {
			c.JSON(200, controller.GetCertInfo())
		})
		common.POST("/applyCert", func(c *gin.Context) {
			c.JSON(200, controller.ApplyCert())
		})
	}
}

func staticRouter(router *gin.Engine) {
	staticFs, _ := fs.Sub(f, "templates/static")
	router.StaticFS("/static", http.FS(staticFs))

	router.GET("/", func(c *gin.Context) {
		indexHTML, _ := f.ReadFile("templates/" + "index.html")
		c.Writer.Write(indexHTML)
	})
}

func noTokenRouter(router *gin.Engine) {
	router.GET("/trojan/user/subscribe", func(c *gin.Context) {
		controller.ClashSubInfo(c)
	})
}

// Start web启动入口
func Start(host string, port, timeout int, isSSL bool) {
	router := gin.Default()
	router.SetTrustedProxies(nil)
	router.Use(gzip.Gzip(gzip.DefaultCompression))
	staticRouter(router)
	noTokenRouter(router)
	router.Use(Auth(router, timeout).MiddlewareFunc())
	trojanRouter(router)
	userRouter(router)
	dataRouter(router)
	commonRouter(router)
	controller.ScheduleTask()
	controller.CollectTask()
	core.GetMysql().CreateTable()
	core.StartDaemon()
	util.OpenPort(port)
	if isSSL {
		config := core.GetConfig()
		ssl := &config.SSl
		router.RunTLS(fmt.Sprintf("%s:%d", host, port), ssl.Cert, ssl.Key)
	} else {
		router.Run(fmt.Sprintf("%s:%d", host, port))
	}
}
