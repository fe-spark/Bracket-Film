package controller

import (
	"strconv"
	"strings"

	"server/logic"
	"server/model/collect"

	"github.com/gin-gonic/gin"
)

// HandleProvide 提供给外界采集的 MacCMS 兼容接口
func HandleProvide(c *gin.Context) {
	ac := c.Query("ac")
	t, _ := strconv.Atoi(c.DefaultQuery("t", "0"))
	pg, _ := strconv.Atoi(c.DefaultQuery("pg", "1"))
	wd := c.Query("wd")
	h, _ := strconv.Atoi(c.DefaultQuery("h", "0"))
	ids := c.Query("ids")

	tid, err := strconv.Atoi(c.Query("tid"))
	if err == nil && tid > 0 {
		t = tid
	}

	// 提取 TVBox 筛选扩展参数
	year, _ := strconv.Atoi(c.Query("year"))
	area := c.Query("area")
	lang := c.Query("language")
	plot := c.Query("plot")
	sort := c.Query("sort")

	classList, filters := logic.PL.GetClassList()
	if classList == nil {
		classList = []collect.FilmClass{}
	}

	// 处理不同类型的请求
	switch ac {
	case "list":
		// 返回简单的视频列表和分类
		page, pagecount, total, vodList := logic.PL.GetVodList(t, pg, wd, h, year, area, lang, plot, sort)
		if vodList == nil {
			vodList = []collect.FilmList{}
		}
		c.JSON(200, gin.H{
			"code":      1,
			"msg":       "数据列表",
			"page":      page,
			"pagecount": pagecount,
			"limit":     "20",
			"total":     total,
			"list":      vodList,
			"class":     classList,
			"filters":   filters,
		})
	case "videolist", "detail":
		// 返回详细的视频信息（包含播放地址）
		var idsArr []string
		if ids != "" {
			idsArr = strings.Split(ids, ",")
			vodList := logic.PL.GetVodDetail(idsArr)
			if vodList == nil {
				vodList = []collect.FilmDetail{}
			}
			c.JSON(200, gin.H{
				"code":      1,
				"msg":       "数据列表",
				"page":      1,
				"pagecount": 1,
				"limit":     "20",
				"total":     len(vodList),
				"list":      vodList,
				"class":     classList,
				"filters":   filters,
			})
		} else {
			// 如果没有传入ids，那么返回列表但是附带完整详情（有些客户端要求视频列表带详情）
			page, pagecount, total, vodListSimple := logic.PL.GetVodList(t, pg, wd, h, year, area, lang, plot, sort)

			// 取出 ids 去查详情
			var _idsArr []string
			for _, v := range vodListSimple {
				_idsArr = append(_idsArr, strconv.FormatInt(v.VodID, 10))
			}

			detailList := logic.PL.GetVodDetail(_idsArr)
			if detailList == nil {
				detailList = []collect.FilmDetail{}
			}

			c.JSON(200, gin.H{
				"code":      1,
				"msg":       "数据列表",
				"page":      page,
				"pagecount": pagecount,
				"limit":     "20",
				"total":     total,
				"list":      detailList,
				"class":     classList,
				"filters":   filters,
			})
		}

	default:
		// 默认返回基础分类和简单的视频列表
		page, pagecount, total, vodList := logic.PL.GetVodList(t, pg, wd, h, year, area, lang, plot, sort)
		if vodList == nil {
			vodList = []collect.FilmList{}
		}
		c.JSON(200, gin.H{
			"code":      1,
			"msg":       "数据列表",
			"page":      page,
			"pagecount": pagecount,
			"limit":     "20",
			"total":     total,
			"list":      vodList,
			"class":     classList,
			"filters":   filters,
		})
	}
}

// HandleProvideConfig 提供给 TVBox/影视仓 的一键网络配置 (config.json)
func HandleProvideConfig(c *gin.Context) {
	// 动态获取当前请求的主机地址(包含协议和端口)
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	host := c.Request.Host
	apiPath := scheme + "://" + host + "/provide/vod"

	configJson := gin.H{
		"spider":    "",
		"wallpaper": "",
		"logo":      "",
		"sites": []gin.H{
			{
				"key":         "Bracket",
				"name":        "🌟 Bracket 私人影视库全量",
				"type":        1,
				"api":         apiPath,
				"searchable":  1,
				"quickSearch": 1,
				"filterable":  1,
			},
		},
	}

	c.JSON(200, configJson)
}
