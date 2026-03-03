package handler

import (
	"strconv"
	"strings"

	"server-v2/internal/model/collect"
	"server-v2/internal/service"

	"github.com/gin-gonic/gin"
)

type ProvideHandler struct{}

var ProvideHd = new(ProvideHandler)

// HandleProvide 提供给外界采集的 MacCMS 兼容接口
func (h *ProvideHandler) HandleProvide(c *gin.Context) {
	ac := c.Query("ac")
	t, _ := strconv.Atoi(c.DefaultQuery("t", "0"))
	pg, _ := strconv.Atoi(c.DefaultQuery("pg", "1"))
	wd := c.Query("wd")
	h_param, _ := strconv.Atoi(c.DefaultQuery("h", "0"))
	ids := c.Query("ids")

	tid, err := strconv.Atoi(c.Query("tid"))
	if err == nil && tid > 0 {
		t = tid
	}

	year, _ := strconv.Atoi(c.Query("year"))
	area := c.Query("area")
	lang := c.Query("language")
	plot := c.Query("plot")
	sort := c.Query("sort")

	classList, filters := service.ProvideSvc.GetClassList()
	if classList == nil {
		classList = []collect.FilmClass{}
	}

	switch ac {
	case "list":
		page, pagecount, total, vodList := service.ProvideSvc.GetVodList(t, pg, wd, h_param, year, area, lang, plot, sort)
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
		var idsArr []string
		if ids != "" {
			idsArr = strings.Split(ids, ",")
			vodList := service.ProvideSvc.GetVodDetail(idsArr)
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
			page, pagecount, total, vodListSimple := service.ProvideSvc.GetVodList(t, pg, wd, h_param, year, area, lang, plot, sort)
			var _idsArr []string
			for _, v := range vodListSimple {
				_idsArr = append(_idsArr, strconv.FormatInt(v.VodID, 10))
			}
			detailList := service.ProvideSvc.GetVodDetail(_idsArr)
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
		page, pagecount, total, vodList := service.ProvideSvc.GetVodList(t, pg, wd, h_param, year, area, lang, plot, sort)
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
func (h *ProvideHandler) HandleProvideConfig(c *gin.Context) {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	host := c.Request.Host
	apiPath := scheme + "://" + host + "/api/provide/vod"

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
