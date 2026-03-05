package handler

import (
	"net/url"
	"strconv"
	"strings"

	"server-v2/internal/model"
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
	sourceId := c.Query("source")

	tid, err := strconv.Atoi(c.Query("tid"))
	if err == nil && tid > 0 {
		t = tid
	}

	year, _ := strconv.Atoi(c.Query("year"))
	area := c.Query("area")
	lang := c.Query("language")
	plot := c.Query("plot")
	sort := c.Query("sort")

	// 选中采集站时，优先直连该采集站返回原始数据
	if sourceId != "" {
		raw, err := service.ProvideSvc.GetVodDirectBySource(sourceId, ac, t, pg, wd, h_param, ids, year, area, lang, plot, sort)
		if err != nil {
			c.JSON(200, gin.H{"code": 0, "msg": "采集站直连失败: " + err.Error()})
			return
		}
		c.Data(200, "application/json; charset=utf-8", raw)
		return
	}

	classList, filters := service.ProvideSvc.GetClassList()
	if classList == nil {
		classList = []model.FilmClass{}
	}

	switch ac {
	case "list":
		page, pagecount, total, vodList := service.ProvideSvc.GetVodList(t, pg, wd, h_param, year, area, lang, plot, sort)
		if vodList == nil {
			vodList = []model.FilmList{}
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
				vodList = []model.FilmDetail{}
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
				detailList = []model.FilmDetail{}
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
			vodList = []model.FilmList{}
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

	sites := []gin.H{
		{
			"key":         "Bracket",
			"name":        "🌟 Bracket 私人影视库全量",
			"type":        1,
			"api":         apiPath,
			"searchable":  1,
			"quickSearch": 1,
			"filterable":  1,
		},
	}

	for _, source := range service.CollectSvc.GetFilmSourceList() {
		if !source.State {
			continue
		}
		sites = append(sites, gin.H{
			"key":         "source_" + source.Id,
			"name":        "📡 " + source.Name,
			"type":        1,
			"api":         apiPath + "?source=" + url.QueryEscape(source.Id),
			"searchable":  1,
			"quickSearch": 1,
			"filterable":  1,
		})
	}

	configJson := gin.H{
		"spider":    "",
		"wallpaper": "",
		"logo":      "",
		"sites":     sites,
	}

	c.JSON(200, configJson)
}
