package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
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

	type categoryResult struct {
		index   int
		item    model.FilmClass
		filters []map[string]any
	}

	resultChan := make(chan categoryResult, len(tree.Children))
	var wg sync.WaitGroup

	for i, c := range tree.Children {
		if !c.Show {
			continue
		}
		wg.Add(1)
		go func(index int, category *model.CategoryTree) {
			defer wg.Done()

			searchTags := repository.GetSearchTag(model.SearchTagsVO{Pid: category.Id})
			tvboxFilters := make([]map[string]any, 0)

			// Robustly get metadata from searchTags
			titles := make(map[string]string)
			if tIf, ok := searchTags["titles"]; ok {
				if tMap, ok := tIf.(map[string]any); ok {
					for k, v := range tMap {
						if vStr, ok := v.(string); ok {
							titles[k] = vStr
						}
					}
				} else if tStrMap, ok := tIf.(map[string]string); ok {
					titles = tStrMap
				}
			}

			var sortList []string
			if sIf, ok := searchTags["sortList"]; ok {
				if sArr, ok := sIf.([]any); ok {
					for _, v := range sArr {
						if vStr, ok := v.(string); ok {
							sortList = append(sortList, vStr)
						}
					}
				} else if sStrArr, ok := sIf.([]string); ok {
					sortList = sStrArr
				}
			}

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
						if key == "Category" && v == "" {
							v = strconv.FormatInt(category.Id, 10)
						}
						values = append(values, map[string]string{"n": item["Name"], "v": v})
					}
				case []any:
					for _, item := range td {
						if m, ok := item.(map[string]any); ok {
							nStr, _ := m["Name"].(string)
							vStr, _ := m["Value"].(string)
							v := vStr
							if key == "Category" && v == "" {
								v = strconv.FormatInt(category.Id, 10)
							}
							values = append(values, map[string]string{"n": nStr, "v": v})
						}
					}
				}

				if len(values) > 0 {
					tvboxKey := strings.ToLower(key)
					if key == "Category" {
						tvboxKey = "tid"
					}
					tvboxFilters = append(tvboxFilters, map[string]any{
						"key": tvboxKey, "name": name, "value": values,
					})
				}
			}

			resultChan <- categoryResult{
				index:   index,
				item:    model.FilmClass{ID: category.Id, Name: category.Name},
				filters: tvboxFilters,
			}
		}(i, c)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 收集并保持顺序 (或根据分类权重排序，这里尝试保持原有 tree.Children 顺序)
	type finalItem struct {
		index   int
		item    model.FilmClass
		filters []map[string]any
	}
	var finals []finalItem
	for res := range resultChan {
		finals = append(finals, finalItem{res.index, res.item, res.filters})
	}

	// 按原始索引排序
	sort.Slice(finals, func(i, j int) bool {
		return finals[i].index < finals[j].index
	})

	for _, f := range finals {
		classList = append(classList, f.item)
		filters[strconv.FormatInt(f.item.ID, 10)] = f.filters
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
func (p *ProvideService) GetVodList(t int, pg int, wd string, h int, year string, area, lang, plot, sort string, limit int) (int, int, int, []model.FilmList) {
	if limit <= 0 {
		limit = 20
	}
	if t <= 0 && wd == "" && h == 0 && year == "" && area == "" && lang == "" && plot == "" {
		return 1, 1, 0, []model.FilmList{}
	}
	// 1. 针对第一页的首页请求尝试 Redis 缓存 (依赖主动失效，TTL 设为 12 小时作为兜底)
	cacheKey := ""
	if pg <= 1 && wd == "" && h == 0 && year == "" && area == "" && lang == "" && plot == "" {
		cacheKey = fmt.Sprintf("%s:%d:S%s:L%d", config.TVBoxList, t, sort, limit)
		if data, err := db.Rdb.Get(db.Cxt, cacheKey).Result(); err == nil && data != "" {
			var res struct {
				Current   int
				PageCount int
				Total     int
				VodList   []model.FilmList
			}
			if json.Unmarshal([]byte(data), &res) == nil {
				return res.Current, res.PageCount, res.Total, res.VodList
			}
		}
	}

	page := dto.Page{PageSize: limit, Current: pg}
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

	if year != "" && year != "全部" {
		if year == model.TagOthersValue || year == "其他" || year == "其它" {
			if pid > 0 {
				topVals := repository.GetTopTagValues(pid, "Year")
				if len(topVals) > 0 {
					query = query.Where("year NOT IN ?", topVals)
				}
			}
		} else if y, err := strconv.Atoi(year); err == nil && y > 0 {
			query = query.Where("year = ?", y)
		}
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
		if (val == model.TagOthersValue || val == "其他" || val == "其它") && pid > 0 {
			topVals := repository.GetTopTagValues(pid, dimType)
			if len(topVals) > 0 {
				if dimType == "Plot" {
					maxPlotExcludes := 5
					if len(topVals) < maxPlotExcludes {
						maxPlotExcludes = len(topVals)
					}
					for i := 0; i < maxPlotExcludes; i++ {
						v := topVals[i]
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

	dto.GetPage(query, &page)

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

	// 2. 写入 Redis 缓存
	if cacheKey != "" {
		res := struct {
			Current   int
			PageCount int
			Total     int
			VodList   []model.FilmList
		}{page.Current, page.PageCount, page.Total, vodList}
		if data, err := json.Marshal(res); err == nil {
			db.Rdb.Set(db.Cxt, cacheKey, string(data), time.Hour*12)
		}
	}

	return page.Current, page.PageCount, page.Total, vodList
}

// GetVodDetail 获取视频详情（带播放列表）
func (p *ProvideService) GetVodDetail(ids []string) []model.FilmDetail {
	var detailList []model.FilmDetail
	siteBasic := repository.GetSiteBasic()
	useProxy := siteBasic.IsVideoProxy
	domain := strings.TrimRight(siteBasic.Domain, "/")

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
				playLink := link.Link
				if useProxy {
					playLink = wrapPlayLinkWithProxy(playLink, domain)
				}
				linkStrs = append(linkStrs, fmt.Sprintf("%s$%s", link.Episode, strings.ReplaceAll(playLink, "$", "")))
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

func wrapPlayLinkWithProxy(link, domain string) string {
	if link == "" {
		return link
	}
	if strings.HasPrefix(link, "/api/proxy/video?url=") {
		return domain + link
	}
	if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
		// 拼接域名 + 代理路径，并在末尾添加 &.m3u8 欺骗播放器
		return domain + "/api/proxy/video?url=" + url.QueryEscape(link) + "&.m3u8"
	}
	return link
}
