package core

import (
	"strings"
)

// DomainAuditInfo 审计展示所需的域名详细信息
type DomainAuditInfo struct {
	MappedDomain string `json:"mapped_domain"` // 映射合并后的主品牌名称（如 Bilibili）
	Category     string `json:"category"`      // 所属分类（如 社交与视频）
	IsStaticCDN  bool   `json:"is_static_cdn"`  // 是否是纯静态资源或广告
}

// 域名后缀到主品牌的映射规则
var domainToBrandRules = []struct {
	Suffix   string
	Brand    string
	Category string
}{
	// 🔞 成人内容
	{Suffix: "pornhub.com", Brand: "Pornhub", Category: "成人内容"},
	{Suffix: "phncdn.com", Brand: "Pornhub", Category: "成人内容"},
	{Suffix: "pornhubpremium.com", Brand: "Pornhub", Category: "成人内容"},
	{Suffix: "xvideos.com", Brand: "XVideos", Category: "成人内容"},
	{Suffix: "xv-ru.com", Brand: "XVideos", Category: "成人内容"},
	{Suffix: "xvideos-cdn.com", Brand: "XVideos", Category: "成人内容"},
	{Suffix: "xvv1deos.com", Brand: "XVideos", Category: "成人内容"},
	{Suffix: "spankbang.com", Brand: "SpankBang", Category: "成人内容"},
	{Suffix: "sbcdn.co", Brand: "SpankBang", Category: "成人内容"},
	{Suffix: "91porn.com", Brand: "91Porn", Category: "成人内容"},
	{Suffix: "91p12.com", Brand: "91Porn", Category: "成人内容"},
	{Suffix: "jable.tv", Brand: "Jable", Category: "成人内容"},
	{Suffix: "missav.com", Brand: "MissAV", Category: "成人内容"},
	{Suffix: "missav.ws", Brand: "MissAV", Category: "成人内容"},

	// 💬 社交与视频
	{Suffix: "youtube.com", Brand: "YouTube", Category: "社交与视频"},
	{Suffix: "googlevideo.com", Brand: "YouTube", Category: "社交与视频"},
	{Suffix: "ytimg.com", Brand: "YouTube", Category: "社交与视频"},
	{Suffix: "youtu.be", Brand: "YouTube", Category: "社交与视频"},
	{Suffix: "bilibili.com", Brand: "Bilibili", Category: "社交与视频"},
	{Suffix: "hdslb.com", Brand: "Bilibili", Category: "社交与视频"},
	{Suffix: "bilivideo.com", Brand: "Bilibili", Category: "社交与视频"},
	{Suffix: "bilivideo.cn", Brand: "Bilibili", Category: "社交与视频"},
	{Suffix: "b23.tv", Brand: "Bilibili", Category: "社交与视频"},
	{Suffix: "acgvideo.com", Brand: "Bilibili", Category: "社交与视频"},
	{Suffix: "acg.tv", Brand: "Bilibili", Category: "社交与视频"},
	{Suffix: "biliapi.net", Brand: "Bilibili", Category: "社交与视频"},
	{Suffix: "biliapi.com", Brand: "Bilibili", Category: "社交与视频"},
	{Suffix: "twitter.com", Brand: "X (Twitter)", Category: "社交与视频"},
	{Suffix: "x.com", Brand: "X (Twitter)", Category: "社交与视频"},
	{Suffix: "twimg.com", Brand: "X (Twitter)", Category: "社交与视频"},
	{Suffix: "tiktok.com", Brand: "TikTok / 抖音", Category: "社交与视频"},
	{Suffix: "byteoversea.com", Brand: "TikTok / 抖音", Category: "社交与视频"},
	{Suffix: "douyin.com", Brand: "TikTok / 抖音", Category: "社交与视频"},
	{Suffix: "amemv.com", Brand: "TikTok / 抖音", Category: "社交与视频"},
	{Suffix: "telegram.org", Brand: "Telegram", Category: "社交与视频"},
	{Suffix: "telegram.dog", Brand: "Telegram", Category: "社交与视频"},
	{Suffix: "tdesktop.com", Brand: "Telegram", Category: "社交与视频"},
	{Suffix: "facebook.com", Brand: "Facebook", Category: "社交与视频"},
	{Suffix: "fbcdn.net", Brand: "Facebook", Category: "社交与视频"},
	{Suffix: "instagram.com", Brand: "Instagram", Category: "社交与视频"},
	{Suffix: "cdninstagram.com", Brand: "Instagram", Category: "社交与视频"},
	{Suffix: "wechat.com", Brand: "微信", Category: "社交与视频"},
	{Suffix: "tenpay.com", Brand: "微信", Category: "社交与视频"},
	{Suffix: "qlogo.cn", Brand: "微信", Category: "社交与视频"},

	// 🛠️ 主流服务与工具
	{Suffix: "google.com", Brand: "Google", Category: "主流服务"},
	{Suffix: "gstatic.com", Brand: "Google", Category: "主流服务"},
	{Suffix: "googleapis.com", Brand: "Google", Category: "主流服务"},
	{Suffix: "gvt1.com", Brand: "Google", Category: "主流服务"},
	{Suffix: "baidu.com", Brand: "百度", Category: "主流服务"},
	{Suffix: "bdstatic.com", Brand: "百度", Category: "主流服务"},
	{Suffix: "bcebos.com", Brand: "百度", Category: "主流服务"},
	{Suffix: "github.com", Brand: "GitHub", Category: "主流服务"},
	{Suffix: "githubusercontent.com", Brand: "GitHub", Category: "主流服务"},
	{Suffix: "github.io", Brand: "GitHub", Category: "主流服务"},
	{Suffix: "microsoft.com", Brand: "微软/Bing", Category: "主流服务"},
	{Suffix: "live.com", Brand: "微软/Bing", Category: "主流服务"},
	{Suffix: "bing.com", Brand: "微软/Bing", Category: "主流服务"},
	{Suffix: "office.com", Brand: "微软/Bing", Category: "主流服务"},
	{Suffix: "apple.com", Brand: "Apple / iCloud", Category: "主流服务"},
	{Suffix: "icloud.com", Brand: "Apple / iCloud", Category: "主流服务"},
	{Suffix: "mzstatic.com", Brand: "Apple / iCloud", Category: "主流服务"},
}

// 纯静态 CDN/广告域名后缀，用于 hide_cdn 过滤
var staticCDNSuffixes = []string{
	"doubleclick.net", "google-analytics.com", "googlesyndication.com",
	"scorecardresearch.com", "cnzz.com", "51.la", "umeng.com",
	"cloudfront.net", "akamaized.net", "fastly.net", "edgecastcdn.net",
	"myqcloud.com", "aliyuncs.com", "dnsv1.com", "qbox.me", "qiniu.com",
	"alicdn.com", "bdstatic.com", "hdslb.com", "phncdn.com", "twimg.com",
	"fbcdn.net", "cdninstagram.com", "sbcdn.co", "xv-ru.com", "xvideos-cdn.com", "xvv1deos.com",
	"bilivideo.cn", "b23.tv", "acgvideo.com",
}

// GetDomainAuditInfo 获取域名的归类与品牌合并详情
func GetDomainAuditInfo(domain string) DomainAuditInfo {
	domain = strings.ToLower(strings.TrimSpace(domain))

	info := DomainAuditInfo{
		MappedDomain: domain,
		Category:     "其他",
		IsStaticCDN:  false,
	}

	// 1. 匹配映射规则
	for _, rule := range domainToBrandRules {
		if domain == rule.Suffix || strings.HasSuffix(domain, "."+rule.Suffix) {
			info.MappedDomain = rule.Brand
			info.Category = rule.Category
			break
		}
	}

	// 2. 检测是否为纯静态/广告域名
	for _, suffix := range staticCDNSuffixes {
		if domain == suffix || strings.HasSuffix(domain, "."+suffix) {
			info.IsStaticCDN = true
			break
		}
	}

	// 特殊兜底逻辑：如果映射到了主流服务/社交等，但是是公认的静态/视频流 CDN，标记为静态 CDN
	// （这样开启 hide_cdn 时可以只保留主要访问记录，而隐藏掉巨大的 googlevideo.com 流量）
	if strings.Contains(domain, "googlevideo.com") ||
		strings.Contains(domain, "hdslb.com") ||
		strings.Contains(domain, "bilivideo.com") ||
		strings.Contains(domain, "bilivideo.cn") ||
		strings.Contains(domain, "acgvideo.com") ||
		strings.Contains(domain, "phncdn.com") ||
		strings.Contains(domain, "xvideos-cdn.com") ||
		strings.Contains(domain, "xvv1deos.com") ||
		(strings.HasSuffix(domain, ".instagram.com") && domain != "instagram.com" && domain != "www.instagram.com") {
		info.IsStaticCDN = true
	}

	return info
}
