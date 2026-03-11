package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"server/internal/config"
	"server/internal/infra/db"
	"server/internal/model"
	"server/internal/model/dto"
	"server/internal/repository"
	"server/internal/utils"
)

type ProvideService struct{}

var ProvideSvc = new(ProvideService)

// GetVodDirectBySource 获取指定采集站直连原始数据(MacCMS 兼容)
func (p *ProvideService) GetVodDirectBySource(sourceId, ac string, t int, pg int, wd string, h int, ids string, year int, area, lang, plot, sort string) ([]byte, error) {
	if sourceId == "" {
		return nil, errors.New("source is required")
	}
	s := repository.FindCollectSourceById(sourceId)
	if s == nil || !s.State {
		return nil, errors.New("collect source not found or disabled")
	}
	if s.ResultModel != model.JsonResult {
		return nil, errors.New("collect source is not json result")
	}

	r := utils.RequestInfo{Uri: s.Uri, Params: url.Values{}}
	if ac == "" {
		ac = "list"
	}
	r.Params.Set("ac", ac)
	if t > 0 {
		r.Params.Set("t", strconv.Itoa(t))
	}
	if pg > 0 {
		r.Params.Set("pg", strconv.Itoa(pg))
	}
	if wd != "" {
		r.Params.Set("wd", wd)
	}
	if h > 0 {
		r.Params.Set("h", strconv.Itoa(h))
	}
	if ids != "" {
		r.Params.Set("ids", ids)
	}
	if year > 0 {
		r.Params.Set("year", strconv.Itoa(year))
	}
	if area != "" {
		r.Params.Set("area", area)
	}
	if lang != "" {
		r.Params.Set("lang", lang)
		r.Params.Set("language", lang)
	}
	if plot != "" {
		r.Params.Set("plot", plot)
	}
	if sort != "" {
		r.Params.Set("sort", sort)
	}

	utils.ApiGet(&r)
	if len(r.Resp) > 0 {
		return r.Resp, nil
	}
	if r.Err != "" {
		return nil, errors.New(r.Err)
	}
	return nil, errors.New("empty response from collect source")
}

// GetClassList 获取格式化的分类列表和筛选条件
func (p *ProvideService) GetClassList() ([]model.FilmClass, map[string][]map[string]any) {
	// 1. 尝试从 Redis 获取缓存 (TVBox 配置缓存 5 分钟)
	cacheKey := config.TVBoxConfigCacheKey
	if data, err := db.Rdb.Get(db.Cxt, cacheKey).Result(); err == nil && data != "" {
		var res struct {
			ClassList []model.FilmClass
			Filters   map[string][]map[string]any
		}
		if json.Unmarshal([]byte(data), &res) == nil {
			return res.ClassList, res.Filters
		}
	}

	var classList []model.FilmClass
	filters := make(map[string][]map[string]any)

	tree := repository.GetActiveCategoryTree()
	for _, c := range tree.Children {
		if c.Show {
			classList = append(classList, model.FilmClass{
				ID:   c.Id,
				Name: c.Name,
			})

			searchTags := repository.GetSearchTag(model.SearchTagsVO{Pid: c.Id})
			// Initialize to empty slice to avoid "null" in JSON
			tvboxFilters := make([]map[string]any, 0)

			// Robustly get titles
			titles := make(map[string]string)
			if tIf, ok := searchTags["titles"]; ok {
				switch t := tIf.(type) {
				case map[string]any:
					for k, v := range t {
						if vStr, ok := v.(string); ok {
							titles[k] = vStr
						}
					}
				case map[string]string:
					titles = t
				}
			}

			// Robustly get sortList
			var sortList []string
			if sIf, ok := searchTags["sortList"]; ok {
				switch s := sIf.(type) {
				case []any:
					for _, v := range s {
						if vStr, ok := v.(string); ok {
							sortList = append(sortList, vStr)
						}
					}
				case []string:
					sortList = s
				}
			}

			// Robustly get tags
			var tags map[string]any
			if tMap, ok := searchTags["tags"].(map[string]any); ok {
				tags = tMap
			}

			for _, key := range sortList {
				name, ok := titles[key]
				if !ok {
					continue
				}

				var values []map[string]string
				tagDataIf := tags[key]
				if tagDataIf == nil {
					continue
				}

				switch td := tagDataIf.(type) {
				case []map[string]string:
					for _, item := range td {
						v := item["Value"]
						// TVBox tid filtering: if value is empty, it means "All", which maps to current type ID
						if key == "Category" && v == "" {
							v = strconv.FormatInt(c.Id, 10)
						}
						values = append(values, map[string]string{
							"n": item["Name"],
							"v": v,
						})
					}
				case []any:
					for _, item := range td {
						if m, ok := item.(map[string]any); ok {
							nStr, _ := m["Name"].(string)
							vStr, _ := m["Value"].(string)
							v := vStr
							if key == "Category" && v == "" {
								v = strconv.FormatInt(c.Id, 10)
							}
							values = append(values, map[string]string{
								"n": nStr,
								"v": v,
							})
						} else if m, ok := item.(map[string]string); ok {
							v := m["Value"]
							if key == "Category" && v == "" {
								v = strconv.FormatInt(c.Id, 10)
							}
							values = append(values, map[string]string{
								"n": m["Name"],
								"v": v,
							})
						}
					}
				}

				if len(values) > 0 {
					tvboxKey := strings.ToLower(key)
					if key == "Category" {
						tvboxKey = "tid"
					}

					tvboxFilters = append(tvboxFilters, map[string]any{
						"key":   tvboxKey,
						"name":  name,
						"value": values,
					})
				}
			}
			filters[strconv.FormatInt(c.Id, 10)] = tvboxFilters
		}
	}

	// 写入 Redis 缓存 (5 分钟)
	res := struct {
		ClassList []model.FilmClass
		Filters   map[string][]map[string]any
	}{classList, filters}
	if data, err := json.Marshal(res); err == nil {
		db.Rdb.Set(db.Cxt, cacheKey, string(data), time.Minute*5)
	}

	return classList, filters
}

// GetVodList 获取视频列表 (支持多维度筛选)
func (p *ProvideService) GetVodList(t int, pg int, wd string, h int, year int, area, lang, plot, sort string) (int, int, int, []model.FilmList) {
	page := dto.Page{PageSize: 20, Current: pg}
	if page.Current <= 0 {
		page.Current = 1
	}

	query := db.Mdb.Model(&model.SearchInfo{})

	var pid int64
	if t > 0 {
		if repository.IsRootCategory(int64(t)) {
			query = query.Where("pid = ?", t)
			pid = int64(t)
		} else {
			query = query.Where("cid = ?", t)
			pid = repository.GetRootId(int64(t))
		}
	}

	if wd != "" {
		query = query.Where("name LIKE ? OR sub_title LIKE ?", "%"+wd+"%", "%"+wd+"%")
	}

	if h > 0 {
		timeLimit := time.Now().Add(-time.Duration(h) * time.Hour).Unix()
		query = query.Where("update_stamp >= ?", timeLimit)
	}

	if year > 0 {
		query = query.Where("year = ?", year)
	}

	// 统一处理“其它”逻辑
	// 2. 处理规范化维度 (Area, Language, Plot) - 全部切换到 MovieTagRel 索引查询
	dims := map[string]string{
		"Area":     area,
		"Language": lang,
		"Plot":     plot,
	}

	for dimType, val := range dims {
		if val == "" || val == "全部" {
			continue
		}
		if val == "其它" && pid > 0 {
			topVals := repository.GetTopTagValues(pid, dimType)
			if len(topVals) > 0 {
				if dimType == "Plot" {
					for _, v := range topVals {
						query = query.Where("class_tag NOT LIKE ?", fmt.Sprintf("%%%s%%", v))
					}
				} else {
					k := strings.ToLower(dimType)
					query = query.Where(fmt.Sprintf("%s NOT IN ?", k), topVals)
				}
			}
		} else {
			if dimType == "Plot" {
				query = query.Where("class_tag LIKE ?", fmt.Sprintf("%%%s%%", val))
			} else {
				k := strings.ToLower(dimType)
				query = query.Where(fmt.Sprintf("%s = ?", k), val)
			}
		}
	}

	var count int64
	query.Count(&count)
	page.Total = int(count)
	page.PageCount = int((page.Total + page.PageSize - 1) / page.PageSize)

	orderBy := "update_stamp DESC"
	switch sort {
	case "hits":
		orderBy = "hits DESC"
	case "score":
		orderBy = "score DESC"
	case "release_stamp":
		orderBy = "release_stamp DESC"
	}

	var sl []model.SearchInfo
	query.Limit(page.PageSize).Offset((page.Current - 1) * page.PageSize).Order(orderBy).Find(&sl)

	var vodList []model.FilmList
	for _, s := range sl {
		vodList = append(vodList, model.FilmList{
			VodID:       s.Mid,
			VodName:     s.Name,
			TypeID:      s.Cid,
			TypeName:    s.CName,
			VodEn:       s.Initial,
			VodTime:     time.Unix(s.UpdateStamp, 0).Format("2006-01-02 15:04:05"),
			VodRemarks:  s.Remarks,
			VodPlayFrom: "bracket",
			VodPic:      s.Picture,
		})
	}

	return page.Current, page.PageCount, page.Total, vodList
}

// GetVodDetail 获取视频详情（带播放列表）
func (p *ProvideService) GetVodDetail(ids []string) []model.FilmDetail {
	var detailList []model.FilmDetail

	for _, idStr := range ids {
		idInt, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}
		var s model.SearchInfo
		if err := db.Mdb.Where("mid = ?", idStr).First(&s).Error; err != nil {
			continue
		}

		movieDetailVo := IndexSvc.GetFilmDetail(idInt)

		if movieDetailVo.Id == 0 && movieDetailVo.Name == "" {
			continue
		}

		var playFromList []string
		var playUrlList []string

		for _, source := range movieDetailVo.List {
			playFromList = append(playFromList, source.Name)

			var linkStrs []string
			for _, link := range source.LinkList {
				linkStrs = append(linkStrs, fmt.Sprintf("%s$%s", link.Episode, strings.ReplaceAll(link.Link, "$", "")))
			}
			playUrlList = append(playUrlList, strings.Join(linkStrs, "#"))
		}

		detail := model.FilmDetail{
			VodID:       s.Mid,
			TypeID:      s.Cid,
			TypeID1:     s.Pid,
			TypeName:    s.CName,
			VodName:     s.Name,
			VodEn:       s.Initial,
			VodTime:     time.Unix(s.UpdateStamp, 0).Format("2006-01-02 15:04:05"),
			VodRemarks:  s.Remarks,
			VodPlayFrom: strings.Join(playFromList, "$$$"),
			VodPlayURL:  strings.Join(playUrlList, "$$$"),
			VodPic:      movieDetailVo.Picture,
			VodSub:      movieDetailVo.SubTitle,
			VodClass:    movieDetailVo.ClassTag,
			VodActor:    movieDetailVo.Actor,
			VodDirector: movieDetailVo.Director,
			VodWriter:   movieDetailVo.Writer,
			VodBlurb:    movieDetailVo.Blurb,
			VodPubDate:  movieDetailVo.ReleaseDate,
			VodArea:     movieDetailVo.Area,
			VodLang:     movieDetailVo.Language,
			VodYear:     movieDetailVo.Year,
			VodState:    movieDetailVo.State,
			VodHits:     s.Hits,
			VodScore:    movieDetailVo.DbScore,
			VodContent:  movieDetailVo.Content,
		}
		detailList = append(detailList, detail)
	}

	return detailList
}
