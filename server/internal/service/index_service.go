package service

import (
	"regexp"
	"strings"

	"server/internal/model"
	"server/internal/model/dto"
	"server/internal/repository"
	"server/internal/utils"
)

type IndexService struct{}

var IndexSvc = new(IndexService)

// IndexPage 首页数据处理
func (i *IndexService) IndexPage() map[string]any {
	Info := make(map[string]any)
	tree := model.CategoryTree{Category: &model.Category{Id: 0, Name: "分类信息"}}
	sysTree := repository.GetActiveCategoryTree()
	for _, c := range sysTree.Children {
		if c.Show {
			tree.Children = append(tree.Children, c)
		}
	}
	Info["category"] = tree
	list := make([]map[string]any, 0)
	for _, c := range tree.Children {
		page := dto.Page{PageSize: 14, Current: 1}
		var movies []model.MovieBasicInfo
		var hotMovies []model.SearchInfo
		if c.Children != nil {
			movies = repository.GetMovieListByPid(c.Id, &page)
			hotMovies = repository.GetHotMovieByPid(c.Id, &page)
		} else {
			movies = repository.GetMovieListByCid(c.Id, &page)
			hotMovies = repository.GetHotMovieByCid(c.Id, &page)
		}
		if movies == nil {
			movies = make([]model.MovieBasicInfo, 0)
		}
		if hotMovies == nil {
			hotMovies = make([]model.SearchInfo, 0)
		}
		item := map[string]any{"nav": c, "movies": movies, "hot": hotMovies}
		list = append(list, item)
	}
	Info["content"] = list
	banners := repository.GetBanners()
	if banners == nil {
		banners = make(model.Banners, 0)
	}
	Info["banners"] = banners
	return Info
}

// GetFilmDetail 影片详情信息页面处理
func (i *IndexService) GetFilmDetail(id int) model.MovieDetailVo {
	search := repository.GetSearchInfoById(int64(id))
	if search == nil {
		return model.MovieDetailVo{List: make([]model.PlayLinkVo, 0)}
	}
	movieDetail := repository.GetMovieDetail(search.Cid, search.Mid)
	if movieDetail == nil {
		return model.MovieDetailVo{List: make([]model.PlayLinkVo, 0)}
	}
	res := model.MovieDetailVo{MovieDetail: *movieDetail}
	res.List = multipleSource(movieDetail)
	return res
}

// GetCategoryInfo 分类信息获取, 组装导航栏需要的信息
func (i *IndexService) GetCategoryInfo() map[string]any {
	nav := make(map[string]any)
	tree := repository.GetCategoryTree()
	for _, t := range tree.Children {
		switch t.Category.Name {
		case "动漫", "动漫片":
			nav["cartoon"] = t
		case "电影", "电影片":
			nav["film"] = t
		case "连续剧", "电视剧":
			nav["tv"] = t
		case "综艺", "综艺片":
			nav["variety"] = t
		}
	}
	return nav
}

// GetNavCategory 获取导航分类信息
func (i *IndexService) GetNavCategory() []*model.Category {
	tree := repository.GetCategoryTree()
	cl := make([]*model.Category, 0)
	for _, c := range tree.Children {
		if c.Show {
			cl = append(cl, c.Category)
		}
	}
	return cl
}

// SearchFilmInfo 获取关键字匹配的影片信息
func (i *IndexService) SearchFilmInfo(key string, page *dto.Page) []model.MovieBasicInfo {
	sl := repository.SearchFilmKeyword(key, page)
	var bl []model.MovieBasicInfo
	for _, s := range sl {
		bl = append(bl, repository.GetBasicInfoByKey(s.Cid, s.Mid))
	}
	return bl
}

// GetFilmCategory 根据Pid或Cid获取指定的分页数据
func (i *IndexService) GetFilmCategory(id int64, idType string, page *dto.Page) []model.MovieBasicInfo {
	var basicList []model.MovieBasicInfo
	switch idType {
	case "pid":
		basicList = repository.GetMovieListByPid(id, page)
	case "cid":
		basicList = repository.GetMovieListByCid(id, page)
	}
	return basicList
}

// GetPidCategory 获取pid对应的分类信息
func (i *IndexService) GetPidCategory(pid int64) *model.CategoryTree {
	tree := repository.GetCategoryTree()
	for _, t := range tree.Children {
		if t.Id == pid {
			return &model.CategoryTree{Category: t.Category, Children: t.Children}
		}
	}
	return nil
}

// RelateMovie 根据当前影片信息匹配相关的影片
func (i *IndexService) RelateMovie(detail model.MovieDetail, page *dto.Page) []model.MovieBasicInfo {
	// 关键修复：从数据库获取规范化后的 SearchInfo，而不是直接使用 detail 中不可信的 Cid/Pid
	search := repository.GetSearchInfoById(detail.Id)
	if search == nil {
		// 备选方案：如果 SearchInfo 暂无，则构造一个简易的
		search = &model.SearchInfo{
			Cid:      detail.Cid,
			Pid:      detail.Pid,
			Name:     detail.Name,
			ClassTag: detail.ClassTag,
			Area:     detail.Area,
			Language: detail.Language,
		}
	}
	return repository.GetRelateMovieBasicInfo(*search, page)
}

// SearchTags 整合对应分类的搜索tag
func (i *IndexService) SearchTags(st model.SearchTagsVO) map[string]any {
	return repository.GetSearchTag(st)
}

func multipleSource(detail *model.MovieDetail) []model.PlayLinkVo {
	master := repository.GetCollectSourceListByGrade(model.MasterCollect)
	if len(master) == 0 || len(detail.PlayList) == 0 {
		return make([]model.PlayLinkVo, 0)
	}
	firstList := detail.PlayList[0]
	if firstList == nil {
		firstList = []model.MovieUrlInfo{}
	}
	playList := []model.PlayLinkVo{{Id: master[0].Id, Name: master[0].Name, LinkList: firstList}}

	names := make(map[string]int)
	if detail.DbId > 0 {
		names[utils.GenerateHashKey(detail.DbId)] = 0
	}
	names[utils.GenerateHashKey(detail.Name)] = 0
	names[utils.GenerateHashKey(regexp.MustCompile(`第一季$`).ReplaceAllString(detail.Name, ""))] = 0

	if len(detail.SubTitle) > 0 && strings.Contains(detail.SubTitle, ",") {
		for v := range strings.SplitSeq(detail.SubTitle, ",") {
			names[utils.GenerateHashKey(v)] = 0
		}
	}
	if len(detail.SubTitle) > 0 && strings.Contains(detail.SubTitle, "/") {
		for v := range strings.SplitSeq(detail.SubTitle, "/") {
			names[utils.GenerateHashKey(v)] = 0
		}
	}
	sc := repository.GetCollectSourceListByGrade(model.SlaveCollect)
	for _, s := range sc {
		for k := range names {
			pl := repository.GetMultiplePlay(s.Id, k)
			if len(pl) > 0 {
				playList = append(playList, model.PlayLinkVo{Id: s.Id, Name: s.Name, LinkList: pl})
				break
			}
		}
	}

	return playList
}

// GetFilmsByTags 通过searchTag 返回满足条件的分页影片信息
func (i *IndexService) GetFilmsByTags(st model.SearchTagsVO, page *dto.Page) []model.MovieBasicInfo {
	sl := repository.GetSearchInfosByTags(st, page)
	return repository.GetBasicInfoBySearchInfos(sl...)
}

// GetFilmClassify 通过Pid返回当前所属分类下的首页展示数据
func (i *IndexService) GetFilmClassify(pid int64, page *dto.Page) map[string]any {
	res := make(map[string]any)
	res["news"] = repository.GetMovieListBySort(0, pid, page)
	res["top"] = repository.GetMovieListBySort(1, pid, page)
	res["recent"] = repository.GetMovieListBySort(2, pid, page)
	return res
}
