package service

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"server-v2/internal/model"
	"server-v2/internal/model/collect"
	"server-v2/internal/repository"
	"server-v2/pkg/db"
	"server-v2/pkg/response"
)

type ProvideService struct{}

var ProvideSvc = new(ProvideService)

// GetClassList 获取格式化的分类列表和筛选条件
func (p *ProvideService) GetClassList() ([]collect.FilmClass, map[string][]map[string]interface{}) {
	var classList []collect.FilmClass
	filters := make(map[string][]map[string]interface{})

	tree := repository.GetCategoryTree()
	for _, c := range tree.Children {
		if c.Show {
			classList = append(classList, collect.FilmClass{
				TypeID:   c.Id,
				TypeName: c.Name,
			})

			searchTags := repository.GetSearchTag(c.Id)
			titles, _ := searchTags["titles"].(map[string]string)
			tags, _ := searchTags["tags"].(map[string]interface{})
			sortList, _ := searchTags["sortList"].([]string)

			var tvboxFilters []map[string]interface{}
			for _, key := range sortList {
				name, ok := titles[key]
				if !ok {
					continue
				}
				tagData, ok := tags[key].([]map[string]string)
				if !ok {
					continue
				}

				var values []map[string]string
				for _, t := range tagData {
					v := t["Value"]
					if key == "Category" && v == "" {
						v = strconv.FormatInt(c.Id, 10)
					}
					values = append(values, map[string]string{
						"n": t["Name"],
						"v": v,
					})
				}

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
			filters[strconv.FormatInt(c.Id, 10)] = tvboxFilters
		}
	}
	return classList, filters
}

// GetVodList 获取视频列表 (支持多维度筛选)
func (p *ProvideService) GetVodList(t int, pg int, wd string, h int, year int, area, lang, plot, sort string) (int, int, int, []collect.FilmList) {
	page := response.Page{PageSize: 20, Current: pg}
	if page.Current <= 0 {
		page.Current = 1
	}

	query := db.Mdb.Model(&model.SearchInfo{})

	if t > 0 {
		query = query.Where("cid = ? OR pid = ?", t, t)
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
	if area != "" && area != "全部" {
		query = query.Where("area = ?", area)
	}
	if lang != "" && lang != "全部" {
		query = query.Where("language = ?", lang)
	}
	if plot != "" && plot != "全部" {
		query = query.Where("class_tag LIKE ?", "%"+plot+"%")
	}

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

	var vodList []collect.FilmList
	for _, s := range sl {
		vodList = append(vodList, collect.FilmList{
			VodID:       s.Mid,
			VodName:     s.Name,
			TypeID:      s.Cid,
			TypeName:    s.CName,
			VodEn:       s.Initial,
			VodTime:     time.Unix(s.UpdateStamp, 0).Format("2006-01-02 15:04:05"),
			VodRemarks:  s.Remarks,
			VodPlayFrom: "bracket",
		})
	}

	return page.Current, page.PageCount, page.Total, vodList
}

// GetVodDetail 获取视频详情（带播放列表）
func (p *ProvideService) GetVodDetail(ids []string) []collect.FilmDetail {
	var detailList []collect.FilmDetail

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

		detail := collect.FilmDetail{
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
