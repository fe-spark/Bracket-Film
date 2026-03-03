package system

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

	"server/config"
	"server/plugin/common/param"
	"server/plugin/db"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SearchInfo 存储用于检索的信息
type SearchInfo struct {
	gorm.Model
	Mid          int64   `json:"mid" gorm:"uniqueIndex:idx_mid"` // 影片ID
	Cid          int64   `json:"cid"`                            // 分类ID
	Pid          int64   `json:"pid"`                            // 上级分类ID
	Name         string  `json:"name"`                           // 片名
	SubTitle     string  `json:"subTitle"`                       // 影片子标题
	CName        string  `json:"cName"`                          // 分类名称
	ClassTag     string  `json:"classTag"`                       // 类型标签
	Area         string  `json:"area"`                           // 地区
	Language     string  `json:"language"`                       // 语言
	Year         int64   `json:"year"`                           // 年份
	Initial      string  `json:"initial"`                        // 首字母
	Score        float64 `json:"score"`                          // 评分
	UpdateStamp  int64   `json:"updateStamp"`                    // 更新时间
	Hits         int64   `json:"hits"`                           // 热度排行
	State        string  `json:"state"`                          // 状态 正片|预告
	Remarks      string  `json:"remarks"`                        // 完结 | 更新至x集
	ReleaseStamp int64   `json:"releaseStamp"`                   // 上映时间 时间戳
	Picture      string  `json:"picture"`                        // 简介图片
	Actor        string  `json:"actor"`                          // 主演
	Director     string  `json:"director"`                       // 导演
	Blurb        string  `json:"blurb"`                          // 简介, 不完整
}

// Tag 影片分类标签结构体
type Tag struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

func (s *SearchInfo) TableName() string {
	return config.SearchTableName
}

// SearchTagItem 影片检索标签持久化模型 (MySQL)
// 取代原 Redis ZSet/Hash 存储，持久化影片分类筛选标签
type SearchTagItem struct {
	gorm.Model
	Pid     int64  `gorm:"uniqueIndex:uidx_search_tag;not null"`
	TagType string `gorm:"uniqueIndex:uidx_search_tag;size:32;not null"`  // Category/Plot/Area/Language/Year/Initial/Sort
	Name    string `gorm:"size:128;not null"`                             // 展示名称
	Value   string `gorm:"uniqueIndex:uidx_search_tag;size:128;not null"` // 筛选值
	Score   int64  `gorm:"default:0"`                                     // 热度权重，用于排序
}

// CreateSearchTagTable 创建检索标签持久化表
func CreateSearchTagTable() {
	if !db.Mdb.Migrator().HasTable(&SearchTagItem{}) {
		_ = db.Mdb.AutoMigrate(&SearchTagItem{})
	}
}

// RedisOnlyFlush 仅清空 Redis 缓存 (不影响 MySQL 持久化数据)
// SearchTag 已迁移至 MySQL，不再清理 Search:Pid* 键
func RedisOnlyFlush() {
	// 1. 清理基础缓存
	db.Rdb.Del(db.Cxt, db.Rdb.Keys(db.Cxt, "MovieBasicInfo:*").Val()...)
	db.Rdb.Del(db.Cxt, db.Rdb.Keys(db.Cxt, "MovieDetail:*").Val()...)
	db.Rdb.Del(db.Cxt, db.Rdb.Keys(db.Cxt, "MultipleSource:*").Val()...)

	// 2. 清理分类树与临时队列
	db.Rdb.Del(db.Cxt, config.CategoryTreeKey)
	db.Rdb.Del(db.Cxt, config.VirtualPictureKey)
}

// FilmZero 删除所有库存数据 (包含 MySQL 持久化表)
func FilmZero() {
	// 1. 清理 Redis (基础缓存)
	RedisOnlyFlush()

	// 2. 清理 MySQL (详情表与检索表)
	db.Mdb.Exec("TRUNCATE table movie_details")
	var s SearchInfo
	db.Mdb.Exec(fmt.Sprintf("TRUNCATE table %s", s.TableName()))
	// 3. 清理新引入的持久化表
	db.Mdb.Exec("TRUNCATE table movie_playlists")
	db.Mdb.Exec("TRUNCATE table category_persistents")
	db.Mdb.Exec("TRUNCATE table virtual_picture_queues")
}

/*
SearchKeyword 设置search关键字集合(影片分类检索类型数据)
	类型, 剧情 , 地区, 语言, 年份, 首字母, 排序
	1. 在影片详情保存到 MySQL 并可选缓存到 Redis 时将影片相关数据进行记录
	2. 通过分值对类型进行排序类型展示到页面
*/

// ensureStaticTagsForPid 确保静态标签 (Year/Initial/Sort) 已写入 MySQL
func ensureStaticTagsForPid(pid int64) {
	// Year: 近 12 年
	var yCount int64
	db.Mdb.Model(&SearchTagItem{}).Where("pid = ? AND tag_type = ?", pid, "Year").Count(&yCount)
	if yCount == 0 {
		currentYear := time.Now().Year()
		var items []SearchTagItem
		for i := 0; i < 12; i++ {
			y := fmt.Sprint(currentYear - i)
			items = append(items, SearchTagItem{Pid: pid, TagType: "Year", Name: y, Value: y, Score: int64(currentYear - i)})
		}
		db.Mdb.Clauses(clause.OnConflict{DoNothing: true}).Create(&items)
	}
	// Initial: A-Z
	var iCount int64
	db.Mdb.Model(&SearchTagItem{}).Where("pid = ? AND tag_type = ?", pid, "Initial").Count(&iCount)
	if iCount == 0 {
		var items []SearchTagItem
		for i := 65; i <= 90; i++ {
			v := string(rune(i))
			items = append(items, SearchTagItem{Pid: pid, TagType: "Initial", Name: v, Value: v, Score: int64(90 - i)})
		}
		db.Mdb.Clauses(clause.OnConflict{DoNothing: true}).Create(&items)
	}
	// Sort: 固定 4 个选项
	var sCount int64
	db.Mdb.Model(&SearchTagItem{}).Where("pid = ? AND tag_type = ?", pid, "Sort").Count(&sCount)
	if sCount == 0 {
		items := []SearchTagItem{
			{Pid: pid, TagType: "Sort", Name: "时间排序", Value: "update_stamp", Score: 3},
			{Pid: pid, TagType: "Sort", Name: "人气排序", Value: "hits", Score: 2},
			{Pid: pid, TagType: "Sort", Name: "评分排序", Value: "score", Score: 1},
			{Pid: pid, TagType: "Sort", Name: "最新上映", Value: "release_stamp", Score: 0},
		}
		db.Mdb.Clauses(clause.OnConflict{DoNothing: true}).Create(&items)
	}
}

// SaveSearchTag 保存影片检索标签到 MySQL
func SaveSearchTag(search SearchInfo) {
	// 确保静态标签已存在
	ensureStaticTagsForPid(search.Pid)
	// Category: 从分类树实时同步
	for _, t := range GetChildrenTree(search.Pid) {
		db.Mdb.Clauses(clause.OnConflict{DoNothing: true}).Create(
			&SearchTagItem{Pid: search.Pid, TagType: "Category", Name: t.Name, Value: fmt.Sprint(t.Id), Score: 0})
	}
	// Plot / Area / Language: 动态 upsert，score 累计热度
	HandleSearchTags(search.ClassTag, "Plot", search.Pid)
	HandleSearchTags(search.Area, "Area", search.Pid)
	HandleSearchTags(search.Language, "Language", search.Pid)
}

// HandleSearchTags 将 preTags 字符串中的各标签 upsert 到 MySQL (score+1)
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
		}).Create(&SearchTagItem{Pid: pid, TagType: tagType, Name: v, Value: v, Score: 1})
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

func BatchHandleSearchTag(infos ...SearchInfo) {
	for _, info := range infos {
		SaveSearchTag(info)
	}
}

// ================================= Spider 数据处理(mysql) =================================

// CreateSearchTable 创建存储检索信息的数据表
func CreateSearchTable() {
	// 如果不存在则创建表
	if !ExistSearchTable() {
		err := db.Mdb.AutoMigrate(&SearchInfo{})
		if err != nil {
			log.Println("Create Table SearchInfo Failed: ", err)
		}
	}
}

// ExistSearchTable 是否存在Search Table
func ExistSearchTable() bool {
	// 1. 判断表中是否存在当前表
	return db.Mdb.Migrator().HasTable(&SearchInfo{})
}

// AddSearchIndex search表中数据保存完毕后 将常用字段添加索引提高查询效率
func AddSearchIndex() {
	var s SearchInfo
	tableName := s.TableName()
	// 添加索引
	db.Mdb.Exec(fmt.Sprintf("CREATE UNIQUE INDEX idx_mid ON %s (mid)", tableName))
	db.Mdb.Exec(fmt.Sprintf("CREATE INDEX idx_time ON %s (update_stamp DESC)", tableName))
	db.Mdb.Exec(fmt.Sprintf("CREATE INDEX idx_hits ON %s (hits DESC)", tableName))
	db.Mdb.Exec(fmt.Sprintf("CREATE INDEX idx_score ON %s (score DESC)", tableName))
	db.Mdb.Exec(fmt.Sprintf("CREATE INDEX idx_release ON %s (release_stamp DESC)", tableName))
	db.Mdb.Exec(fmt.Sprintf("CREATE INDEX idx_year ON %s (year DESC)", tableName))
}

// upsertColumns 是 search_infos 中重复采集时需要覆盖更新的列（mid 是冲突键，不在此列表中）
var upsertColumns = []string{
	"cid", "pid", "name", "sub_title", "c_name", "class_tag",
	"area", "language", "year", "initial", "score",
	"update_stamp", "hits", "state", "remarks", "release_stamp",
	"picture", "actor", "director", "blurb", "updated_at",
}

// BatchSave 批量保存影片search信息（已统一为 upsert，保留函数签名兼容旧调用）
func BatchSave(list []SearchInfo) {
	BatchSaveOrUpdate(list)
}

// BatchSaveOrUpdate 批量 upsert 影片检索信息
// 使用 ON CONFLICT (mid) DO UPDATE 替代原有的「先 count 再 create/update」循环事务，
// 彻底消除：① Rollback-without-return 导致整批丢失；② 并发 TOCTOU 重复主键冲突。
func BatchSaveOrUpdate(list []SearchInfo) {
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
	// 插入/更新成功后将相应 tag 数据写入 MySQL
	BatchHandleSearchTag(list...)
}

// SaveSearchInfo 保存单条影片检索信息（upsert）
func SaveSearchInfo(s SearchInfo) error {
	isNew := !ExistSearchInfo(s.Mid)
	err := db.Mdb.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "mid"}},
		DoUpdates: clause.AssignmentColumns(upsertColumns),
	}).Create(&s).Error
	if err != nil {
		log.Printf("SaveSearchInfo upsert 失败 mid=%d: %v\n", s.Mid, err)
		return err
	}
	// 新增时才写 tag（更新时 tag 不做回收，由 BatchHandleSearchTag 增量维护）
	if isNew {
		BatchHandleSearchTag(s)
	}
	return nil
}

// ExistSearchInfo 通过Mid查询是否存在影片的检索信息
func ExistSearchInfo(mid int64) bool {
	var count int64
	db.Mdb.Model(&SearchInfo{}).Where("mid", mid).Count(&count)
	return count > 0
}

// TunCateSearchTable 截断SearchInfo数据表
func TunCateSearchTable() {
	var searchInfo SearchInfo
	err := db.Mdb.Exec(fmt.Sprintf("TRUNCATE TABLE %s", searchInfo.TableName())).Error
	if err != nil {
		log.Println("TRUNCATE TABLE Error: ", err)
	}
}

// ================================= API 数据接口信息处理 =================================

// GetMovieListByPid  通过Pid 分类ID 获取对应影片的数据信息
func GetMovieListByPid(pid int64, page *Page) []MovieBasicInfo {
	// 1. 优先尝试从 Redis 获取缓存列表
	cacheKey := fmt.Sprintf("Cache:List:Pid%d:Pg%d:Sz%d", pid, page.Current, page.PageSize)
	cacheData := db.Rdb.Get(db.Cxt, cacheKey).Val()
	if cacheData != "" {
		var list []MovieBasicInfo
		if err := json.Unmarshal([]byte(cacheData), &list); err == nil {
			return list
		}
	}

	// 2. Redis 未命中，查询 MySQL
	var count int64
	db.Mdb.Model(&SearchInfo{}).Where("pid", pid).Count(&count)
	page.Total = int(count)
	page.PageCount = int((page.Total + page.PageSize - 1) / page.PageSize)

	var s []SearchInfo
	if err := db.Mdb.Limit(page.PageSize).Offset((page.Current-1)*page.PageSize).Where("pid", pid).Order("update_stamp DESC").Find(&s).Error; err != nil {
		log.Println(err)
		return nil
	}

	var list []MovieBasicInfo
	for _, v := range s {
		list = append(list, MovieBasicInfo{
			Id: v.Mid, Cid: v.Cid, Pid: v.Pid, Name: v.Name, SubTitle: v.SubTitle,
			CName: v.CName, State: v.State, Picture: v.Picture, Actor: v.Actor,
			Director: v.Director, Blurb: v.Blurb, Remarks: v.Remarks,
			Area: v.Area, Year: fmt.Sprint(v.Year),
		})
	}

	// 3. 将结果回填 Redis 缓存 (短效)
	if len(list) > 0 {
		data, _ := json.Marshal(list)
		_ = db.Rdb.Set(db.Cxt, cacheKey, data, time.Minute*30).Err() // 列表页缓存 30 分钟即可
	}

	return list
}

// GetMovieListByCid 通过Cid查找对应的影片分页数据
func GetMovieListByCid(cid int64, page *Page) []MovieBasicInfo {
	// 1. 优先尝试从 Redis 获取缓存列表
	cacheKey := fmt.Sprintf("Cache:List:Cid%d:Pg%d:Sz%d", cid, page.Current, page.PageSize)
	cacheData := db.Rdb.Get(db.Cxt, cacheKey).Val()
	if cacheData != "" {
		var list []MovieBasicInfo
		if err := json.Unmarshal([]byte(cacheData), &list); err == nil {
			return list
		}
	}

	// 2. Redis 未命中，查询 MySQL
	var count int64
	db.Mdb.Model(&SearchInfo{}).Where("cid", cid).Count(&count)
	page.Total = int(count)
	page.PageCount = int((page.Total + page.PageSize - 1) / page.PageSize)

	var s []SearchInfo
	if err := db.Mdb.Limit(page.PageSize).Offset((page.Current-1)*page.PageSize).Where("cid", cid).Order("update_stamp DESC").Find(&s).Error; err != nil {
		log.Println(err)
		return nil
	}

	var list []MovieBasicInfo
	for _, v := range s {
		list = append(list, MovieBasicInfo{
			Id: v.Mid, Cid: v.Cid, Pid: v.Pid, Name: v.Name, SubTitle: v.SubTitle,
			CName: v.CName, State: v.State, Picture: v.Picture, Actor: v.Actor,
			Director: v.Director, Blurb: v.Blurb, Remarks: v.Remarks,
			Area: v.Area, Year: fmt.Sprint(v.Year),
		})
	}

	// 3. 将结果回填 Redis 缓存 (短效)
	if len(list) > 0 {
		data, _ := json.Marshal(list)
		_ = db.Rdb.Set(db.Cxt, cacheKey, data, time.Minute*30).Err()
	}

	return list
}

// GetHotMovieByPid  获取Pid指定类别的热门影片
func GetHotMovieByPid(pid int64, page *Page) []SearchInfo {
	// 返回分页参数
	// var count int64
	// db.Mdb.Model(&SearchInfo{}).Where("pid", pid).Count(&count)
	// page.Total = int(count)
	// page.PageCount = int((page.Total + page.PageSize - 1) / page.PageSize)
	// 进行具体的信息查询
	var s []SearchInfo
	// 当前时间偏移一个月
	t := time.Now().AddDate(0, -1, 0).Unix()
	if err := db.Mdb.Limit(page.PageSize).Offset((page.Current-1)*page.PageSize).Where("pid=? AND update_stamp > ?", pid, t).Order(" year DESC, hits DESC").Find(&s).Error; err != nil {
		log.Println(err)
		return nil
	}
	return s
}

// GetHotMovieByCid 获取当前分类下的热门影片
func GetHotMovieByCid(cid int64, page *Page) []SearchInfo {
	// 返回分页参数
	// var count int64
	// db.Mdb.Model(&SearchInfo{}).Where("pid", pid).Count(&count)
	// page.Total = int(count)
	// page.PageCount = int((page.Total + page.PageSize - 1) / page.PageSize)
	// 进行具体的信息查询
	var s []SearchInfo
	// 当前时间偏移一个月
	t := time.Now().AddDate(0, -1, 0).Unix()
	if err := db.Mdb.Limit(page.PageSize).Offset((page.Current-1)*page.PageSize).Where("cid=? AND update_stamp > ?", cid, t).Order(" year DESC, hits DESC").Find(&s).Error; err != nil {
		log.Println(err)
		return nil
	}
	return s
}

// SearchFilmKeyword 通过关键字搜索库存中满足条件的影片名
func SearchFilmKeyword(keyword string, page *Page) []SearchInfo {
	// 1. 优先尝试从 Redis 获取缓存
	cacheKey := fmt.Sprintf("Cache:Search:%s:Pg%d:Sz%d", keyword, page.Current, page.PageSize)
	cacheData := db.Rdb.Get(db.Cxt, cacheKey).Val()
	if cacheData != "" {
		var list []SearchInfo
		if err := json.Unmarshal([]byte(cacheData), &list); err == nil {
			return list
		}
	}

	var searchList []SearchInfo
	// 2. 先统计搜索满足条件的数据量
	var count int64
	db.Mdb.Model(&SearchInfo{}).Where("name LIKE ?", fmt.Sprint(`%`, keyword, `%`)).Or("sub_title LIKE ?", fmt.Sprint(`%`, keyword, `%`)).Count(&count)
	page.Total = int(count)
	page.PageCount = int((page.Total + page.PageSize - 1) / page.PageSize)

	// 3. 获取满足条件的数据
	db.Mdb.Limit(page.PageSize).Offset((page.Current-1)*page.PageSize).
		Where("name LIKE ?", fmt.Sprintf(`%%%s%%`, keyword)).Or("sub_title LIKE ?", fmt.Sprintf(`%%%s%%`, keyword)).Order("year DESC, update_stamp DESC").Find(&searchList)

	// 4. 将结果回填 Redis 缓存 (短效)
	if len(searchList) > 0 {
		data, _ := json.Marshal(searchList)
		_ = db.Rdb.Set(db.Cxt, cacheKey, data, time.Minute*10).Err() // 搜索缓存建议时间更短
	}

	return searchList
}

// GetRelateMovieBasicInfo GetRelateMovie 根据SearchInfo获取相关影片
func GetRelateMovieBasicInfo(search SearchInfo, page *Page) []MovieBasicInfo {
	/*
		根据当前影片信息匹配相关的影片
		1. 分类Cid,
		2. 如果影片名称含有第x季 则根据影片名进行模糊匹配
		3. class_tag 剧情内容匹配, 切分后使用 or 进行匹配
		4. area 地区
		5. 语言 Language
	*/
	// sql 拼接查询条件
	sql := ""

	// 优先进行名称相似匹配
	// search.Name = regexp.MustCompile("第.{1,3}季").ReplaceAllString(search.Name, "")
	name := regexp.MustCompile(`(第.{1,3}季.*)|([0-9]{1,3})|(剧场版)|(\s\S*$)|(之.*)|([\p{P}\p{S}].*)`).ReplaceAllString(search.Name, "")
	// 如果处理后的影片名称依旧没有改变 且具有一定长度 则截取部分内容作为搜索条件
	if len(name) == len(search.Name) && len(name) > 10 {
		// 中文字符需截取3的倍数,否则可能乱码
		name = name[:int(math.Ceil(float64(len(name))/5)*3)]
	}
	sql = fmt.Sprintf(`select * from %s where (name LIKE "%%%s%%" or sub_title LIKE "%%%[2]s%%") AND cid=%d AND search.deleted_at IS NULL union`, search.TableName(), name, search.Cid)
	// 执行后续匹配内容, 匹配结果过少,减少过滤条件
	// sql = fmt.Sprintf(`%s select * from %s where cid=%d AND area="%s" AND language="%s" AND`, sql, search.TableName(), search.Cid, search.Area, search.Language)

	// 添加其他相似匹配规则
	sql = fmt.Sprintf(`%s (select * from %s where cid=%d AND `, sql, search.TableName(), search.Cid)
	// 根据剧情标签查找相似影片, classTag 使用的分隔符为 , | /
	// 首先去除 classTag 中包含的所有空格
	search.ClassTag = strings.ReplaceAll(search.ClassTag, " ", "")
	// 如果 classTag 中包含分割符则进行拆分匹配
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
	// 除名称外的相似影片使用随机排序
	// sql = fmt.Sprintf("%s ORDER BY RAND() limit %d,%d)", sql, page.Current, page.PageSize)
	sql = fmt.Sprintf("%s AND search.deleted_at IS NULL limit %d,%d)", sql, page.Current, page.PageSize)
	// 条件拼接完成后加上limit参数
	sql = fmt.Sprintf("(%s)  limit %d,%d", sql, page.Current, page.PageSize)
	// 执行sql
	var list []SearchInfo
	db.Mdb.Raw(sql).Scan(&list)
	// 根据list 获取对应的BasicInfo
	var basicList []MovieBasicInfo
	for _, s := range list {
		// 通过key获取对应的影片基本数据
		basicList = append(basicList, GetBasicInfoByKey(fmt.Sprintf(config.MovieBasicInfoKey, s.Cid, s.Mid)))
	}

	return basicList
}

// GetMultiplePlay 通过影片名hash值匹配播放源 (MySQL 优先)
func GetMultiplePlay(siteId, key string) []MovieUrlInfo {
	// 1. 优先从 Redis 获取 (可选)
	cacheKey := fmt.Sprintf("Cache:Play:%s:%s", siteId, key)
	cacheData := db.Rdb.Get(db.Cxt, cacheKey).Val()
	if cacheData != "" {
		var playList []MovieUrlInfo
		if err := json.Unmarshal([]byte(cacheData), &playList); err == nil {
			return playList
		}
	}

	// 2. Redis 未命中，查询 MySQL
	var playlist MoviePlaylist
	var playList []MovieUrlInfo
	if err := db.Mdb.Where("source_id = ? AND movie_key = ?", siteId, key).First(&playlist).Error; err == nil {
		_ = json.Unmarshal([]byte(playlist.Content), &playList)
		// 3. 回填缓存
		_ = db.Rdb.Set(db.Cxt, cacheKey, playlist.Content, time.Minute*30).Err()
	}
	return playList
}

// GetSearchTag 通过影片分类 Pid 返回对应分类的 tag 信息 (纯 MySQL)
func GetSearchTag(pid int64) map[string]interface{} {
	res := make(map[string]interface{})
	sortList := []string{"Category", "Plot", "Area", "Language", "Year", "Sort"}
	res["sortList"] = sortList
	// titles 为固定映射，不再依赖 Redis
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

// GetTagsByTitle 从 MySQL 返回 Pid+TagType 对应的检索标签 ("Name:Value" 格式)
func GetTagsByTitle(pid int64, t string) []string {
	if t == "Category" {
		// Category 直接从分类树获取，保证实时性
		var tags []string
		for _, c := range GetChildrenTree(pid) {
			if c.Show {
				tags = append(tags, fmt.Sprintf("%s:%d", c.Name, c.Id))
			}
		}
		return tags
	}
	limits := map[string]int{"Plot": 11, "Area": 12, "Language": 7}
	var items []SearchTagItem
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

// HandleTagStr 处理tag数据格式
func HandleTagStr(title string, tags ...string) []map[string]string {
	r := make([]map[string]string, 0)
	if !strings.EqualFold(title, "Sort") {
		r = append(r, map[string]string{
			"Name":  "全部",
			"Value": "",
		})
	}
	for _, t := range tags {
		if sl := strings.Split(t, ":"); len(sl) > 0 {
			r = append(r, map[string]string{
				"Name":  sl[0],
				"Value": sl[1],
			})
		}
	}
	if !strings.EqualFold(title, "Sort") && !strings.EqualFold(title, "Year") && !strings.EqualFold(title, "Category") {
		r = append(r, map[string]string{
			"Name":  "其它",
			"Value": "其它",
		})
	}
	return r
}

// GetSearchInfosByTags 查询满足searchTag条件的影片分页数据
func GetSearchInfosByTags(st SearchTagsVO, page *Page) []SearchInfo {
	// 准备查询语句的条件
	qw := db.Mdb.Model(&SearchInfo{})
	// 通过searchTags的非空属性值, 拼接对应的查询条件
	t := reflect.TypeOf(st)
	v := reflect.ValueOf(st)
	for i := 0; i < t.NumField(); i++ {
		// 如果字段值不为空
		value := v.Field(i).Interface()
		if !param.IsEmpty(value) {
			// 如果value是 其它 则进行特殊处理
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

	// 返回分页参数
	GetPage(qw, page)
	// 查询具体的searchInfo 分页数据
	var sl []SearchInfo
	if err := qw.Limit(page.PageSize).Offset((page.Current - 1) * page.PageSize).Find(&sl).Error; err != nil {
		log.Println(err)
		return nil
	}
	return sl
}

// GetMovieListBySort 通过排序类型返回对应的影片基本信息
func GetMovieListBySort(t int, pid int64, page *Page) []MovieBasicInfo {
	var sl []SearchInfo
	qw := db.Mdb.Model(&SearchInfo{}).Where("pid", pid).Limit(page.PageSize).Offset((page.Current) - 10*page.PageSize)
	// 针对不同排序类型返回对应的分页数据
	switch t {
	case 0:
		// 最新上映 (上映时间)
		qw.Order("release_stamp DESC")
	case 1:
		// 排行榜 (暂定为热度排行)
		qw.Order("hits DESC")
	case 2:
		// 最近更新 (更新时间)
		qw.Order("update_stamp DESC")
	}
	if err := qw.Find(&sl).Error; err != nil {
		log.Println(err)
		return nil
	}
	return GetBasicInfoBySearchInfos(sl...)
}

// ================================= Manage 管理后台 =================================

// GetSearchPage 获取影片检索分页数据
func GetSearchPage(s SearchVo) []SearchInfo {
	// 构建 query查询条件
	query := db.Mdb.Model(&SearchInfo{})
	// 如果参数不为空则追加对应查询条件
	if s.Name != "" {
		query = query.Where("name LIKE ?", fmt.Sprintf("%%%s%%", s.Name))
	}
	// 分类ID为负数则默认不追加该条件
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

	// 返回分页参数
	GetPage(query, s.Paging)
	// 查询具体的数据
	var sl []SearchInfo
	if err := query.Limit(s.Paging.PageSize).Offset((s.Paging.Current - 1) * s.Paging.PageSize).Find(&sl).Error; err != nil {
		log.Println(err)
		return nil
	}
	return sl
}

// GetSearchOptions 获取影片分类检索的筛选标签 (纯 MySQL)
func GetSearchOptions(pid int64) map[string]interface{} {
	tagMap := make(map[string]interface{})
	for _, t := range []string{"Plot", "Area", "Language", "Year"} {
		tagMap[t] = HandleTagStr(t, GetTagsByTitle(pid, t)...)
	}
	return tagMap
}

// GetSearchInfoById 查询id对应的检索信息
func GetSearchInfoById(id int64) *SearchInfo {
	s := SearchInfo{}
	if err := db.Mdb.First(&s, id).Error; err != nil {
		log.Println(err)
		return nil
	}
	return &s
}

// DelFilmSearch 删除影片检索信息, (不影响后续更新, 逻辑删除)
func DelFilmSearch(id int64) error {
	// 通过检索id对影片检索信息进行删除
	if err := db.Mdb.Delete(&SearchInfo{}, id).Error; err != nil {
		log.Println(err)
		return err
	}
	return nil
}

// ShieldFilmSearch 删除所属分类下的所有影片检索信息
func ShieldFilmSearch(cid int64) error {
	// 通过检索id对影片检索信息进行删除
	if err := db.Mdb.Where("cid = ?", cid).Delete(&SearchInfo{}).Error; err != nil {
		log.Println(err)
		return err
	}
	return nil
}

// RecoverFilmSearch 恢复所属分类下的影片检索信息状态
func RecoverFilmSearch(cid int64) error {
	// 通过检索id对影片检索信息进行删除
	if err := db.Mdb.Model(&SearchInfo{}).Unscoped().Where("cid = ?", cid).Update("deleted_at", nil).Error; err != nil {
		log.Println(err)
		return err
	}
	return nil
}

// ================================= 接口数据缓存 =================================

// DataCache  API请求 数据缓存
func DataCache(key string, data map[string]interface{}) {
	val, _ := json.Marshal(data)
	db.Rdb.Set(db.Cxt, key, val, time.Minute*30)
}

// GetCacheData 获取API接口的缓存数据
func GetCacheData(key string) map[string]interface{} {
	data := make(map[string]interface{})
	val, err := db.Rdb.Get(db.Cxt, key).Result()
	if err != nil || len(val) <= 0 {
		return nil
	}
	_ = json.Unmarshal([]byte(val), &data)
	return data
}

// RemoveCache 删除数据缓存
func RemoveCache(key string) {
	db.Rdb.Del(db.Cxt, key)
}

// ================================= OpenApi请求处理 =================================

func FindFilmIds(params map[string]string, page *Page) ([]int64, error) {
	var ids []int64
	query := db.Mdb.Model(&SearchInfo{}).Select("mid")
	for k, v := range params {
		// 如果 v 为空则直接 continue
		if len(v) <= 0 {
			continue
		}
		switch k {
		case "t":
			if cid, err := strconv.ParseInt(v, 10, 64); err == nil {
				query = query.Where("cid = ?", cid)
			}
		case "wd":
			query = query.Where("name like ?", fmt.Sprintf("%%%s%%", v))
		case "h":
			if h, err := strconv.ParseInt(v, 10, 64); err == nil {
				query = query.Where("update_stamp >= ?", time.Now().Unix()-h*3600)
			}
		}
	}
	// 返回分页参数
	var count int64
	query.Count(&count)
	page.Total = int(count)
	page.PageCount = int(page.Total+page.PageSize-1) / page.PageSize
	// 返回满足条件的ids
	err := query.Limit(page.PageSize).Offset(page.Current - 1).Order("update_stamp DESC").Find(&ids).Error
	return ids, err
}
