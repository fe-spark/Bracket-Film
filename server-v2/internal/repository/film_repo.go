package repository

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"server-v2/config"
	"server-v2/internal/model"
	"server-v2/pkg/db"
	"server-v2/pkg/param"
	"server-v2/pkg/response"
	"server-v2/pkg/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ========= Table Initialization =========

func CreateSearchTagTable() {
	if !db.Mdb.Migrator().HasTable(&model.SearchTagItem{}) {
		_ = db.Mdb.AutoMigrate(&model.SearchTagItem{})
	}
}

func CreateSearchTable() {
	if ExistSearchTable() {
		var s model.SearchInfo
		dedup := fmt.Sprintf(
			`DELETE FROM %s WHERE id NOT IN (SELECT max_id FROM (SELECT MAX(id) AS max_id FROM %[1]s GROUP BY mid) AS t)`,
			s.TableName(),
		)
		if err := db.Mdb.Exec(dedup).Error; err != nil {
			log.Println("Dedup search_infos Failed: ", err)
		}
	}
	if err := db.Mdb.AutoMigrate(&model.SearchInfo{}); err != nil {
		log.Println("AutoMigrate SearchInfo Failed: ", err)
	}
}

func ExistSearchTable() bool {
	return db.Mdb.Migrator().HasTable(&model.SearchInfo{})
}

func CreateMovieDetailTable() {
	if !db.Mdb.Migrator().HasTable(&model.MovieDetailInfo{}) {
		_ = db.Mdb.AutoMigrate(&model.MovieDetailInfo{})
	}
}

func CreateMoviePlaylistTable() {
	if !db.Mdb.Migrator().HasTable(&model.MoviePlaylist{}) {
		_ = db.Mdb.AutoMigrate(&model.MoviePlaylist{})
	}
}

// ========= Upsert Logic =========

var upsertColumns = []string{
	"cid", "pid", "name", "sub_title", "c_name", "class_tag",
	"area", "language", "year", "initial", "score",
	"update_stamp", "hits", "state", "remarks", "release_stamp",
	"picture", "actor", "director", "blurb", "updated_at", "deleted_at",
}

func BatchSaveOrUpdate(list []model.SearchInfo) {
	if len(list) == 0 {
		return
	}
	if err := db.Mdb.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "mid"}},
		DoUpdates: clause.AssignmentColumns(upsertColumns),
	}).CreateInBatches(&list, 200).Error; err != nil {
		log.Printf("BatchSaveOrUpdate upsert 失败: %v\n", err)
		return
	}
	BatchHandleSearchTag(list...)
}

func SaveSearchInfo(s model.SearchInfo) error {
	isNew := !ExistSearchInfo(s.Mid)
	err := db.Mdb.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "mid"}},
		DoUpdates: clause.AssignmentColumns(upsertColumns),
	}).Create(&s).Error
	if err != nil {
		return err
	}
	if isNew {
		BatchHandleSearchTag(s)
	}
	return nil
}

func SaveDetails(list []model.MovieDetail) error {
	var infoList []model.SearchInfo
	for _, v := range list {
		infoList = append(infoList, ConvertSearchInfo(v))
	}
	BatchSaveOrUpdate(infoList)

	var details []model.MovieDetailInfo
	for _, v := range list {
		data, _ := json.Marshal(v)
		details = append(details, model.MovieDetailInfo{Mid: v.Id, Cid: v.Cid, Content: string(data)})
	}

	if len(details) > 0 {
		return db.Mdb.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "mid"}},
			DoUpdates: clause.AssignmentColumns([]string{"cid", "content", "updated_at"}),
		}).Create(&details).Error
	}
	return nil
}

func SaveDetail(detail model.MovieDetail) error {
	searchInfo := ConvertSearchInfo(detail)
	if err := SaveSearchInfo(searchInfo); err != nil {
		return err
	}
	data, _ := json.Marshal(detail)
	return db.Mdb.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "mid"}},
		DoUpdates: clause.AssignmentColumns([]string{"cid", "content", "updated_at"}),
	}).Create(&model.MovieDetailInfo{Mid: detail.Id, Cid: detail.Cid, Content: string(data)}).Error
}

func SaveSitePlayList(id string, list []model.MovieDetail) error {
	if len(list) <= 0 {
		return nil
	}
	var playlists []model.MoviePlaylist
	for _, d := range list {
		if len(d.PlayList) == 0 || strings.Contains(d.CName, "解说") {
			continue
		}
		data, _ := json.Marshal(d.PlayList)

		if d.DbId != 0 {
			playlists = append(playlists, model.MoviePlaylist{
				SourceId: id,
				MovieKey: utils.GenerateHashKey(d.DbId),
				Content:  string(data),
			})
		}
		playlists = append(playlists, model.MoviePlaylist{
			SourceId: id,
			MovieKey: utils.GenerateHashKey(d.Name),
			Content:  string(data),
		})
	}
	if len(playlists) > 0 {
		db.Mdb.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "source_id"}, {Name: "movie_key"}},
			DoUpdates: clause.AssignmentColumns([]string{"content", "updated_at"}),
		}).Create(&playlists)
	}
	return nil
}

// ========= Tag Operations =========

func BatchHandleSearchTag(infos ...model.SearchInfo) {
	for _, info := range infos {
		SaveSearchTag(info)
	}
}

func SaveSearchTag(search model.SearchInfo) {
	ensureStaticTagsForPid(search.Pid)
	for _, t := range GetChildrenTree(search.Pid) {
		db.Mdb.Clauses(clause.OnConflict{DoNothing: true}).Create(
			&model.SearchTagItem{Pid: search.Pid, TagType: "Category", Name: t.Name, Value: fmt.Sprint(t.Id), Score: 0})
	}
	HandleSearchTags(search.ClassTag, "Plot", search.Pid)
	HandleSearchTags(search.Area, "Area", search.Pid)
	HandleSearchTags(search.Language, "Language", search.Pid)
}

func ensureStaticTagsForPid(pid int64) {
	var count int64
	db.Mdb.Model(&model.SearchTagItem{}).Where("pid = ? AND tag_type = ?", pid, "Year").Count(&count)
	if count == 0 {
		currentYear := time.Now().Year()
		var items []model.SearchTagItem
		for i := 0; i < 12; i++ {
			y := fmt.Sprint(currentYear - i)
			items = append(items, model.SearchTagItem{Pid: pid, TagType: "Year", Name: y, Value: y, Score: int64(currentYear - i)})
		}
		db.Mdb.Clauses(clause.OnConflict{DoNothing: true}).Create(&items)
	}
	count = 0
	db.Mdb.Model(&model.SearchTagItem{}).Where("pid = ? AND tag_type = ?", pid, "Initial").Count(&count)
	if count == 0 {
		var items []model.SearchTagItem
		for i := 65; i <= 90; i++ {
			v := string(rune(i))
			items = append(items, model.SearchTagItem{Pid: pid, TagType: "Initial", Name: v, Value: v, Score: int64(90 - i)})
		}
		db.Mdb.Clauses(clause.OnConflict{DoNothing: true}).Create(&items)
	}
	count = 0
	db.Mdb.Model(&model.SearchTagItem{}).Where("pid = ? AND tag_type = ?", pid, "Sort").Count(&count)
	if count == 0 {
		items := []model.SearchTagItem{
			{Pid: pid, TagType: "Sort", Name: "时间排序", Value: "update_stamp", Score: 3},
			{Pid: pid, TagType: "Sort", Name: "人气排序", Value: "hits", Score: 2},
			{Pid: pid, TagType: "Sort", Name: "评分排序", Value: "score", Score: 1},
			{Pid: pid, TagType: "Sort", Name: "最新上映", Value: "release_stamp", Score: 0},
		}
		db.Mdb.Clauses(clause.OnConflict{DoNothing: true}).Create(&items)
	}
}

func HandleSearchTags(preTags string, tagType string, pid int64) {
	preTags = regexp.MustCompile(`[\s\n\r]+`).ReplaceAllString(preTags, "")
	if preTags == "" || preTags == "其它" {
		return
	}
	upsert := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" || v == "其它" {
			return
		}
		db.Mdb.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "pid"}, {Name: "tag_type"}, {Name: "value"}},
			DoUpdates: clause.Assignments(map[string]interface{}{"score": gorm.Expr("score + 1")}),
		}).Create(&model.SearchTagItem{Pid: pid, TagType: tagType, Name: v, Value: v, Score: 1})
	}
	var vals []string
	switch {
	case strings.Contains(preTags, "/"):
		vals = strings.Split(preTags, "/")
	case strings.Contains(preTags, ","):
		vals = strings.Split(preTags, ",")
	case strings.Contains(preTags, "，"):
		vals = strings.Split(preTags, "，")
	case strings.Contains(preTags, "、"):
		vals = strings.Split(preTags, "、")
	default:
		vals = []string{preTags}
	}
	for _, v := range vals {
		upsert(v)
	}
}

// ========= Queries =========

func ExistSearchInfo(mid int64) bool {
	var count int64
	db.Mdb.Model(&model.SearchInfo{}).Where("mid", mid).Count(&count)
	return count > 0
}

func ConvertSearchInfo(detail model.MovieDetail) model.SearchInfo {
	score, _ := strconv.ParseFloat(detail.DbScore, 64)
	stamp, _ := time.ParseInLocation(time.DateTime, detail.UpdateTime, time.Local)
	year, err := strconv.ParseInt(regexp.MustCompile(`[1-9][0-9]{3}`).FindString(detail.ReleaseDate), 10, 64)
	if err != nil {
		year = 0
	}
	return model.SearchInfo{
		Mid:          detail.Id,
		Cid:          detail.Cid,
		Pid:          detail.Pid,
		Name:         detail.Name,
		SubTitle:     detail.SubTitle,
		CName:        detail.CName,
		ClassTag:     detail.ClassTag,
		Area:         detail.Area,
		Language:     detail.Language,
		Year:         year,
		Initial:      detail.Initial,
		Score:        score,
		Hits:         detail.Hits,
		UpdateStamp:  stamp.Unix(),
		State:        detail.State,
		Remarks:      detail.Remarks,
		ReleaseStamp: detail.AddTime,
		Picture:      detail.Picture,
		Actor:        detail.Actor,
		Director:     detail.Director,
		Blurb:        detail.Blurb,
	}
}

// GetMovieListByPid 获取指定父类 ID 的影片基本信息
func GetMovieListByPid(pid int64, page *response.Page) []model.MovieBasicInfo {
	var count int64
	db.Mdb.Model(&model.SearchInfo{}).Where("pid = ?", pid).Count(&count)
	page.Total = int(count)
	page.PageCount = int((page.Total + page.PageSize - 1) / page.PageSize)

	var s []model.SearchInfo
	if err := db.Mdb.Limit(page.PageSize).Offset((page.Current-1)*page.PageSize).Where("pid = ?", pid).Order("update_stamp DESC").Find(&s).Error; err != nil {
		log.Printf("GetMovieListByPid Error: %v", err)
		return nil
	}

	var list []model.MovieBasicInfo
	for _, v := range s {
		list = append(list, model.MovieBasicInfo{
			Id: v.Mid, Cid: v.Cid, Pid: v.Pid, Name: v.Name, SubTitle: v.SubTitle,
			CName: v.CName, State: v.State, Picture: v.Picture, Actor: v.Actor,
			Director: v.Director, Blurb: v.Blurb, Remarks: v.Remarks,
			Area: v.Area, Year: fmt.Sprint(v.Year),
		})
	}
	return list
}

// GetMovieListByCid 获取指定子类 ID 的影片基本信息
func GetMovieListByCid(cid int64, page *response.Page) []model.MovieBasicInfo {
	var count int64
	db.Mdb.Model(&model.SearchInfo{}).Where("cid = ?", cid).Count(&count)
	page.Total = int(count)
	page.PageCount = int((page.Total + page.PageSize - 1) / page.PageSize)

	var s []model.SearchInfo
	if err := db.Mdb.Limit(page.PageSize).Offset((page.Current-1)*page.PageSize).Where("cid = ?", cid).Order("update_stamp DESC").Find(&s).Error; err != nil {
		log.Printf("GetMovieListByCid Error: %v", err)
		return nil
	}

	var list []model.MovieBasicInfo
	for _, v := range s {
		list = append(list, model.MovieBasicInfo{
			Id: v.Mid, Cid: v.Cid, Pid: v.Pid, Name: v.Name, SubTitle: v.SubTitle,
			CName: v.CName, State: v.State, Picture: v.Picture, Actor: v.Actor,
			Director: v.Director, Blurb: v.Blurb, Remarks: v.Remarks,
			Area: v.Area, Year: fmt.Sprint(v.Year),
		})
	}
	return list
}

func SearchFilmKeyword(keyword string, page *response.Page) []model.SearchInfo {
	var searchList []model.SearchInfo
	var count int64
	db.Mdb.Model(&model.SearchInfo{}).Where("name LIKE ?", fmt.Sprint(`%`, keyword, `%`)).Or("sub_title LIKE ?", fmt.Sprint(`%`, keyword, `%`)).Count(&count)
	page.Total = int(count)
	page.PageCount = int((page.Total + page.PageSize - 1) / page.PageSize)

	db.Mdb.Limit(page.PageSize).Offset((page.Current-1)*page.PageSize).
		Where("name LIKE ?", fmt.Sprintf(`%%%s%%`, keyword)).Or("sub_title LIKE ?", fmt.Sprintf(`%%%s%%`, keyword)).Order("year DESC, update_stamp DESC").Find(&searchList)

	return searchList
}

func GetRelateMovieBasicInfo(search model.SearchInfo, page *response.Page) []model.MovieBasicInfo {
	sql := ""
	name := regexp.MustCompile(`(第.{1,3}季.*)|([0-9]{1,3})|(剧场版)|(\s\S*$)|(之.*)|([\p{P}\p{S}].*)`).ReplaceAllString(search.Name, "")
	if len(name) == len(search.Name) && len(name) > 10 {
		name = name[:int(math.Ceil(float64(len(name))/5)*3)]
	}
	sql = fmt.Sprintf(`select * from %s where (name LIKE "%%%s%%" or sub_title LIKE "%%%[2]s%%") AND cid=%d AND search.deleted_at IS NULL union`, search.TableName(), name, search.Cid)

	sql = fmt.Sprintf(`%s (select * from %s where cid=%d AND `, sql, search.TableName(), search.Cid)
	search.ClassTag = strings.ReplaceAll(search.ClassTag, " ", "")

	if strings.Contains(search.ClassTag, ",") {
		s := "("
		for _, t := range strings.Split(search.ClassTag, ",") {
			s = fmt.Sprintf(`%s class_tag like "%%%s%%" OR`, s, t)
		}
		sql = fmt.Sprintf("%s %s)", sql, strings.TrimSuffix(s, "OR"))
	} else if strings.Contains(search.ClassTag, "/") {
		s := "("
		for _, t := range strings.Split(search.ClassTag, "/") {
			s = fmt.Sprintf(`%s class_tag like "%%%s%%" OR`, s, t)
		}
		sql = fmt.Sprintf("%s %s)", sql, strings.TrimSuffix(s, "OR"))
	} else {
		sql = fmt.Sprintf(`%s class_tag like "%%%s%%"`, sql, search.ClassTag)
	}

	sql = fmt.Sprintf("%s AND search.deleted_at IS NULL limit %d,%d)", sql, page.Current, page.PageSize)
	sql = fmt.Sprintf("(%s)  limit %d,%d", sql, page.Current, page.PageSize)

	var list []model.SearchInfo
	db.Mdb.Raw(sql).Scan(&list)

	var basicList []model.MovieBasicInfo
	for _, s := range list {
		basicList = append(basicList, GetBasicInfoByKey(s.Cid, s.Mid))
	}
	return basicList
}

// GetBasicInfoByKey 获取影片的基本信息
func GetBasicInfoByKey(cid int64, mid int64) model.MovieBasicInfo {
	var info model.MovieDetailInfo
	if err := db.Mdb.Where("mid = ?", mid).First(&info).Error; err == nil {
		var detail model.MovieDetail
		_ = json.Unmarshal([]byte(info.Content), &detail)
		return model.MovieBasicInfo{
			Id: detail.Id, Cid: detail.Cid, Pid: detail.Pid, Name: detail.Name,
			SubTitle: detail.SubTitle, CName: detail.CName, State: detail.State,
			Picture: detail.Picture, Actor: detail.Actor, Director: detail.Director,
			Blurb: detail.Blurb, Remarks: detail.Remarks, Area: detail.Area, Year: detail.Year,
		}
	}
	return model.MovieBasicInfo{}
}

// GetMovieDetail 获取影片详情信息
func GetMovieDetail(cid int64, mid int64) *model.MovieDetail {
	var movieDetailInfo model.MovieDetailInfo
	if err := db.Mdb.Where("mid = ?", mid).First(&movieDetailInfo).Error; err != nil {
		log.Printf("GetMovieDetail Error: %v", err)
		return nil
	}
	var detail model.MovieDetail
	if err := json.Unmarshal([]byte(movieDetailInfo.Content), &detail); err != nil {
		log.Printf("Unmarshal MovieDetail Error: %v", err)
		return nil
	}

	// 统一将 nil slice 初始化为空 slice，保证前端始终收到 [] 而非 null
	if detail.PlayFrom == nil {
		detail.PlayFrom = []string{}
	}
	if detail.PlayList == nil {
		detail.PlayList = [][]model.MovieUrlInfo{}
	} else {
		for i, inner := range detail.PlayList {
			if inner == nil {
				detail.PlayList[i] = []model.MovieUrlInfo{}
			}
		}
	}
	if detail.DownloadList == nil {
		detail.DownloadList = [][]model.MovieUrlInfo{}
	} else {
		for i, inner := range detail.DownloadList {
			if inner == nil {
				detail.DownloadList[i] = []model.MovieUrlInfo{}
			}
		}
	}
	return &detail
}

func GetMovieDetailByDBID(mid int64, name string) []model.MoviePlaySource {
	var mps []model.MoviePlaySource
	sources := GetCollectSourceList()
	for _, s := range sources {
		if s.Grade == model.SlaveCollect && s.State {
			var playlist model.MoviePlaylist
			key := utils.GenerateHashKey(mid)
			if mid == 0 {
				key = utils.GenerateHashKey(name)
			}
			if err := db.Mdb.Where("source_id = ? AND movie_key = ?", s.Id, key).First(&playlist).Error; err != nil && mid != 0 {
				db.Mdb.Where("source_id = ? AND movie_key = ?", s.Id, utils.GenerateHashKey(name)).First(&playlist)
			}
			if playlist.ID > 0 {
				var playLists [][]model.MovieUrlInfo
				if jsonErr := json.Unmarshal([]byte(playlist.Content), &playLists); jsonErr == nil {
					for _, pl := range playLists {
						if len(pl) > 0 {
							mps = append(mps, model.MoviePlaySource{SiteName: s.Name, PlayList: pl})
						}
					}
				}
			}
		}
	}
	return mps
}

func GetTagsByTitle(pid int64, t string) []string {
	if t == "Category" {
		var tags []string
		for _, c := range GetChildrenTree(pid) {
			if c.Show {
				tags = append(tags, fmt.Sprintf("%s:%d", c.Name, c.Id))
			}
		}
		return tags
	}
	limits := map[string]int{"Plot": 11, "Area": 12, "Language": 7}
	var items []model.SearchTagItem
	q := db.Mdb.Where("pid = ? AND tag_type = ?", pid, t).Order("score DESC")
	if limit, ok := limits[t]; ok {
		q = q.Limit(limit)
	}
	q.Find(&items)
	tags := make([]string, 0, len(items))
	for _, item := range items {
		tags = append(tags, fmt.Sprintf("%s:%s", item.Name, item.Value))
	}
	return tags
}

func HandleTagStr(title string, tags ...string) []map[string]string {
	r := make([]map[string]string, 0)
	if !strings.EqualFold(title, "Sort") {
		r = append(r, map[string]string{"Name": "全部", "Value": ""})
	}
	for _, t := range tags {
		if sl := strings.Split(t, ":"); len(sl) > 0 {
			r = append(r, map[string]string{"Name": sl[0], "Value": sl[1]})
		}
	}
	if !strings.EqualFold(title, "Sort") && !strings.EqualFold(title, "Year") && !strings.EqualFold(title, "Category") {
		r = append(r, map[string]string{"Name": "其它", "Value": "其它"})
	}
	return r
}

// GetSearchTag 获取搜索标签
func GetSearchTag(pid int64) map[string]interface{} {
	res := make(map[string]interface{})
	sortList := []string{"Category", "Plot", "Area", "Language", "Year", "Sort"}
	res["sortList"] = sortList
	res["titles"] = map[string]string{
		"Category": "类型",
		"Plot":     "剧情",
		"Area":     "地区",
		"Language": "语言",
		"Year":     "年份",
		"Sort":     "排序",
	}
	tagMap := make(map[string]interface{})
	for _, t := range sortList {
		tagMap[t] = HandleTagStr(t, GetTagsByTitle(pid, t)...)
	}
	res["tags"] = tagMap
	return res
}

func GetSearchOptions(pid int64) map[string]interface{} {
	tagMap := make(map[string]interface{})
	for _, t := range []string{"Plot", "Area", "Language", "Year"} {
		tagMap[t] = HandleTagStr(t, GetTagsByTitle(pid, t)...)
	}
	return tagMap
}

func GetSearchPage(s model.SearchVo) []model.SearchInfo {
	query := db.Mdb.Model(&model.SearchInfo{})
	if s.Name != "" {
		query = query.Where("name LIKE ?", fmt.Sprintf("%%%s%%", s.Name))
	}
	if s.Cid > 0 {
		query = query.Where("cid = ?", s.Cid)
	} else if s.Pid > 0 {
		query = query.Where("pid = ?", s.Pid)
	}
	if s.Plot != "" {
		query = query.Where("class_tag LIKE ?", fmt.Sprintf("%%%s%%", s.Plot))
	}
	if s.Area != "" {
		query = query.Where("area = ?", s.Area)
	}
	if s.Language != "" {
		query = query.Where("language = ?", s.Language)
	}
	if int(s.Year) > time.Now().Year()-12 {
		query = query.Where("year = ?", s.Year)
	}
	switch s.Remarks {
	case "完结":
		query = query.Where("remarks IN ?", []string{"完结", "HD"})
	case "":
	default:
		query = query.Not(map[string]interface{}{"remarks": []string{"完结", "HD"}})
	}
	if s.BeginTime > 0 {
		query = query.Where("update_stamp >= ? ", s.BeginTime)
	}
	if s.EndTime > 0 {
		query = query.Where("update_stamp <= ? ", s.EndTime)
	}

	response.GetPage(query, s.Paging)
	var sl []model.SearchInfo
	if err := query.Limit(s.Paging.PageSize).Offset((s.Paging.Current - 1) * s.Paging.PageSize).Find(&sl).Error; err != nil {
		log.Printf("GetSearchPage Error: %v", err)
		return nil
	}
	return sl
}

func GetSearchInfosByTags(st model.SearchTagsVO, page *response.Page) []model.SearchInfo {
	qw := db.Mdb.Model(&model.SearchInfo{})
	t := reflect.TypeOf(st)
	v := reflect.ValueOf(st)
	for i := 0; i < t.NumField(); i++ {
		value := v.Field(i).Interface()
		if !param.IsEmpty(value) {
			var ts []string
			if v, flag := value.(string); flag && strings.EqualFold(v, "其它") {
				for _, s := range GetTagsByTitle(st.Pid, t.Field(i).Name) {
					ts = append(ts, strings.Split(s, ":")[1])
				}
			}
			k := strings.ToLower(t.Field(i).Name)
			switch k {
			case "pid", "cid", "year":
				qw = qw.Where(fmt.Sprintf("%s = ?", k), value)
			case "area", "language":
				if strings.EqualFold(value.(string), "其它") {
					qw = qw.Where(fmt.Sprintf("%s NOT IN ?", k), ts)
					break
				}
				qw = qw.Where(fmt.Sprintf("%s = ?", k), value)
			case "plot":
				if strings.EqualFold(value.(string), "其它") {
					for _, t := range ts {
						qw = qw.Where("class_tag NOT LIKE ?", fmt.Sprintf("%%%v%%", t))
					}
					break
				}
				qw = qw.Where("class_tag LIKE ?", fmt.Sprintf("%%%v%%", value))
			case "sort":
				if strings.EqualFold(value.(string), "release_stamp") {
					qw.Order(fmt.Sprintf("year DESC ,%v DESC", value))
					break
				}
				qw.Order(fmt.Sprintf("%v DESC", value))
			default:
				break
			}
		}
	}

	response.GetPage(qw, page)
	var sl []model.SearchInfo
	if err := qw.Limit(page.PageSize).Offset((page.Current - 1) * page.PageSize).Find(&sl).Error; err != nil {
		log.Printf("GetSearchInfosByTags Error: %v", err)
		return nil
	}
	return sl
}

func GetSearchInfoById(id int64) *model.SearchInfo {
	s := model.SearchInfo{}
	if err := db.Mdb.Where("mid = ?", id).First(&s).Error; err != nil {
		log.Printf("GetSearchInfoById Error: %v", err)
		return nil
	}
	return &s
}

func DelFilmSearch(id int64) error {
	if err := db.Mdb.Where("mid = ?", id).Delete(&model.SearchInfo{}).Error; err != nil {
		log.Printf("DelFilmSearch Error: %v", err)
		return err
	}
	return nil
}

func ShieldFilmSearch(cid int64) error {
	if err := db.Mdb.Where("cid = ?", cid).Delete(&model.SearchInfo{}).Error; err != nil {
		log.Printf("ShieldFilmSearch Error: %v", err)
		return err
	}
	return nil
}

func RecoverFilmSearch(cid int64) error {
	if err := db.Mdb.Model(&model.SearchInfo{}).Unscoped().Where("cid = ?", cid).Update("deleted_at", nil).Error; err != nil {
		log.Printf("RecoverFilmSearch Error: %v", err)
		return err
	}
	return nil
}

// RedisOnlyFlush 仅清空 Redis 配置类缓存
func RedisOnlyFlush() {
	// 清理分类树与临时队列
	db.Rdb.Del(db.Cxt, config.CategoryTreeKey)
	db.Rdb.Del(db.Cxt, config.VirtualPictureKey)
}

// FilmZero 删除所有库存数据 (包含 MySQL 持久化表)
func FilmZero() {
	// 1. 清理 Redis (基础缓存)
	RedisOnlyFlush()

	// 2. 清理 MySQL (详情表与检索表)
	db.Mdb.Exec("TRUNCATE table movie_detail_infos")
	var s model.SearchInfo
	db.Mdb.Exec(fmt.Sprintf("TRUNCATE table %s", s.TableName()))
	// 3. 清理新引入的持久化表
	db.Mdb.Exec("TRUNCATE table movie_playlists")
	db.Mdb.Exec("TRUNCATE table category_persistents")
	db.Mdb.Exec("TRUNCATE table virtual_picture_queues")
	db.Mdb.Exec("TRUNCATE table search_tag_items")

	// 4. 同步清理采集失败记录，确保彻底清空
	TruncateRecordTable()
}

// MasterFilmZero 仅清理主站相关数据 (search_infos / movie_detail_infos / category)
// 保留附属站 movie_playlists 数据，用于主站切换时防止附属站数据丢失
func MasterFilmZero() {
	var s model.SearchInfo
	db.Mdb.Exec(fmt.Sprintf("TRUNCATE table %s", s.TableName()))
	db.Mdb.Exec("TRUNCATE table movie_detail_infos")
	db.Mdb.Exec("TRUNCATE table category_persistents")
	db.Mdb.Exec("TRUNCATE table virtual_picture_queues")
	db.Mdb.Exec("TRUNCATE table search_tag_items")
}

// CleanOrphanPlaylists 清理 movie_playlists 中与 search_infos 不匹配的孤儿记录
// 仅当 search_infos 存在数据时执行，避免主站清空后误删全部播放列表
func CleanOrphanPlaylists() int64 {
	// 1. 取出所有主站影片名称
	var names []string
	db.Mdb.Model(&model.SearchInfo{}).Pluck("name", &names)
	if len(names) == 0 {
		log.Println("[CleanOrphan] search_infos 为空，跳过孤儿清理")
		return 0
	}

	// 2. 生成有效 movie_key 集合（名称原文 + 去除"第一季"后缀的变体）
	re := regexp.MustCompile(`第一季$`)
	validKeys := make(map[string]struct{}, len(names)*2)
	for _, name := range names {
		validKeys[utils.GenerateHashKey(name)] = struct{}{}
		if trimmed := re.ReplaceAllString(name, ""); trimmed != name {
			validKeys[utils.GenerateHashKey(trimmed)] = struct{}{}
		}
	}

	// 3. 取出 movie_playlists 中所有 movie_key
	var allKeys []string
	db.Mdb.Model(&model.MoviePlaylist{}).Distinct().Pluck("movie_key", &allKeys)

	// 4. 找出孤儿 key（不在 validKeys 集合中）
	var orphanKeys []string
	for _, key := range allKeys {
		if _, ok := validKeys[key]; !ok {
			orphanKeys = append(orphanKeys, key)
		}
	}

	if len(orphanKeys) == 0 {
		log.Println("[CleanOrphan] movie_playlists 无孤儿记录")
		return 0
	}

	// 5. 批量删除孤儿记录
	result := db.Mdb.Where("movie_key IN ?", orphanKeys).Delete(&model.MoviePlaylist{})
	log.Printf("[CleanOrphan] 已清理 %d 条孤儿 movie_playlists 记录\n", result.RowsAffected)
	return result.RowsAffected
}

// GetHotMovieByPid 获取当前级分类下的热门影片
func GetHotMovieByPid(pid int64, page *response.Page) []model.SearchInfo {
	var s []model.SearchInfo
	t := time.Now().AddDate(0, -1, 0).Unix()
	if err := db.Mdb.Limit(page.PageSize).Offset((page.Current-1)*page.PageSize).Where("pid=? AND update_stamp > ?", pid, t).Order(" year DESC, hits DESC").Find(&s).Error; err != nil {
		log.Printf("GetHotMovieByPid Error: %v", err)
		return nil
	}
	return s
}

// GetHotMovieByCid 获取当前分类下的热门影片
func GetHotMovieByCid(cid int64, page *response.Page) []model.SearchInfo {
	var s []model.SearchInfo
	t := time.Now().AddDate(0, -1, 0).Unix()
	if err := db.Mdb.Limit(page.PageSize).Offset((page.Current-1)*page.PageSize).Where("cid=? AND update_stamp > ?", cid, t).Order(" year DESC, hits DESC").Find(&s).Error; err != nil {
		log.Printf("GetHotMovieByCid Error: %v", err)
		return nil
	}
	return s
}

// GetMultiplePlay 通过影片名 hash 值匹配播放源
func GetMultiplePlay(siteId, key string) []model.MovieUrlInfo {
	var playlist model.MoviePlaylist
	var playList []model.MovieUrlInfo
	if err := db.Mdb.Where("source_id = ? AND movie_key = ?", siteId, key).First(&playlist).Error; err == nil {
		var allPlayList [][]model.MovieUrlInfo
		if err := json.Unmarshal([]byte(playlist.Content), &allPlayList); err == nil && len(allPlayList) > 0 && len(allPlayList[0]) > 0 {
			playList = allPlayList[0]
		}
	}
	return playList
}

// GetBasicInfoBySearchInfos 通过 searchInfo 获取影片的基本信息
func GetBasicInfoBySearchInfos(infos ...model.SearchInfo) []model.MovieBasicInfo {
	var list []model.MovieBasicInfo
	for _, s := range infos {
		list = append(list, model.MovieBasicInfo{
			Id:       s.Mid,
			Cid:      s.Cid,
			Pid:      s.Pid,
			Name:     s.Name,
			SubTitle: s.SubTitle,
			CName:    s.CName,
			State:    s.State,
			Picture:  s.Picture,
			Actor:    s.Actor,
			Director: s.Director,
			Blurb:    s.Blurb,
			Remarks:  s.Remarks,
			Area:     s.Area,
			Year:     fmt.Sprint(s.Year),
		})
	}
	return list
}

// GetMovieListBySort 通过排序类型返回对应的影片基本信息
func GetMovieListBySort(t int, pid int64, page *response.Page) []model.MovieBasicInfo {
	var sl []model.SearchInfo
	qw := db.Mdb.Model(&model.SearchInfo{}).Where("pid", pid).Limit(page.PageSize).Offset((page.Current - 1) * page.PageSize)
	switch t {
	case 0:
		qw.Order("release_stamp DESC")
	case 1:
		qw.Order("hits DESC")
	case 2:
		qw.Order("update_stamp DESC")
	}
	if err := qw.Find(&sl).Error; err != nil {
		log.Printf("GetMovieListBySort Error: %v", err)
		return nil
	}
	return GetBasicInfoBySearchInfos(sl...)
}
