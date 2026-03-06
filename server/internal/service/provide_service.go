package service

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"server/internal/infra/db"
	"server/internal/model"
	"server/internal/model/dto"
	"server/internal/repository"
	"server/internal/utils"

	"gorm.io/gorm"
)

type ProvideService struct{}

var ProvideSvc = new(ProvideService)

// GetVodDirectBySource 指定采集站直连获取原始数据（MacCMS 兼容）
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
func (p *ProvideService) GetClassList() ([]model.FilmClass, map[string][]map[string]interface{}) {
	var classList []model.FilmClass
	filters := make(map[string][]map[string]interface{})

	tree := repository.GetCategoryTree()
	for _, c := range tree.Children {
		if c.Show {
			classList = append(classList, model.FilmClass{
				ID:   c.Id,
				Name: c.Name,
			})

			searchTags := repository.GetSearchTag(c.Id)
			// Initialize to empty slice to avoid "null" in JSON
			tvboxFilters := make([]map[string]interface{}, 0)

			// Robustly get titles
			titles := make(map[string]string)
			if tIf, ok := searchTags["titles"]; ok {
				switch t := tIf.(type) {
				case map[string]interface{}:
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
				case []interface{}:
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
			var tags map[string]interface{}
			if tMap, ok := searchTags["tags"].(map[string]interface{}); ok {
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
				case []interface{}:
					for _, item := range td {
						if m, ok := item.(map[string]interface{}); ok {
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

					tvboxFilters = append(tvboxFilters, map[string]interface{}{
						"key":   tvboxKey,
						"name":  name,
						"value": values,
					})
				}
			}
			filters[strconv.FormatInt(c.Id, 10)] = tvboxFilters
		}
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
		query = query.Where("cid = ? OR pid = ?", t, t)
		pid = repository.GetParentId(int64(t))
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
	handleOther := func(curValue, tagType string, q *gorm.DB) *gorm.DB {
		if curValue == "" || curValue == "全部" {
			return q
		}
		if curValue == "其它" && pid > 0 {
			tags := repository.GetTagsByTitle(pid, tagType)
			var exclude []string
			for _, tg := range tags {
				if sl := strings.Split(tg, ":"); len(sl) > 1 {
					exclude = append(exclude, sl[1])
				}
			}
			if len(exclude) > 0 {
				col := strings.ToLower(tagType)
				if tagType == "Plot" {
					for _, ex := range exclude {
						q = q.Where("class_tag NOT LIKE ?", "%"+ex+"%")
					}
					return q
				}
				return q.Where(fmt.Sprintf("%s NOT IN ?", col), exclude)
			}
		}
		if tagType == "Plot" {
			return q.Where("class_tag LIKE ?", "%"+curValue+"%")
		}
		return q.Where(fmt.Sprintf("%s = ?", strings.ToLower(tagType)), curValue)
	}

	query = handleOther(area, "Area", query)
	query = handleOther(lang, "Language", query)
	query = handleOther(plot, "Plot", query)

	var count int64
	query.Count(&count)
	page.Total = int(count)
	page.PageCount = int((page.Total + page.PageSize - 1) / page.PageSize)

	orderBy := "update_stamp DESC"
	if sort != "" {
		switch sort {
		case "hits":
			orderBy = "hits DESC"
		case "score":
			orderBy = "score DESC"
		case "release_stamp":
			orderBy = "release_stamp DESC"
		}
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
