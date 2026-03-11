package repository

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"server/internal/config"
	"server/internal/infra/db"
	"server/internal/model"
	"server/internal/model/dto"
	"server/internal/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ExistSearchTable 检查搜索表是否存在
func ExistSearchTable() bool {
	return db.Mdb.Migrator().HasTable(&model.SearchInfo{})
}

func ExistSearchInMid(mid int64) bool {
	var count int64
	db.Mdb.Model(&model.SearchInfo{}).Where("mid = ?", mid).Count(&count)
	return count > 0
}

// ========= Upsert Logic =========

var upsertColumns = []string{
	"mid", "cid", "pid", "name", "sub_title", "c_name", "class_tag",
	"area", "language", "year", "initial", "score",
	"update_stamp", "hits", "state", "remarks", "release_stamp",
	"picture", "actor", "director", "blurb", "updated_at", "deleted_at",
}

func BatchSaveOrUpdate(list []model.SearchInfo) map[string]int64 {
	// 过滤无意义的空记录
	validList := make([]model.SearchInfo, 0, len(list))
	for _, v := range list {
		if strings.TrimSpace(v.Name) != "" {
			validList = append(validList, v)
		}
	}
	if len(validList) == 0 {
		return nil
	}
	list = validList

	// 1. 基于 ContentKey 进行冲突检测，实现内容级的去重
	if err := db.Mdb.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "content_key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"source_id", "cid", "pid", "name", "sub_title", "c_name", "class_tag",
			"area", "language", "year", "initial", "score",
			"update_stamp", "hits", "state", "remarks", "release_stamp",
			"picture", "actor", "director", "blurb", "updated_at",
		}),
	}).CreateInBatches(&list, 200).Error; err != nil {
		log.Printf("BatchSaveOrUpdate upsert 失败: %v\n", err)
		return nil
	}

	// 清除相关分类的搜索标签缓存
	pidSet := make(map[int64]struct{})
	for _, v := range list {
		pidSet[v.Pid] = struct{}{}
	}
	for pid := range pidSet {
		ClearSearchTagsCache(pid)
	}
	// 清除首页活跃分类树缓存，防止处于采集早期时前台只能看到空缓存
	db.Rdb.Del(db.Cxt, config.ActiveCategoryTreeKey)

	// 2. 建立来源映射关系 (获取最终生效的 GlobalMid)
	var contentKeys []string
	for _, v := range list {
		contentKeys = append(contentKeys, v.ContentKey)
	}

	var latestInfos []model.SearchInfo
	db.Mdb.Where("content_key IN ?", contentKeys).Find(&latestInfos)

	keyToMid := make(map[string]int64)
	for _, info := range latestInfos {
		keyToMid[info.ContentKey] = info.Mid
	}

	var mappings []model.MovieSourceMapping
	for _, v := range list {
		if globalMid, ok := keyToMid[v.ContentKey]; ok {
			mappings = append(mappings, model.MovieSourceMapping{
				SourceId:  v.SourceId,
				SourceMid: v.Mid,
				GlobalMid: globalMid,
			})
		}
	}

	if len(mappings) > 0 {
		db.Mdb.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "source_id"}, {Name: "source_mid"}},
			DoUpdates: clause.AssignmentColumns([]string{"global_mid", "updated_at"}),
		}).CreateInBatches(&mappings, 200)
	}

	BatchHandleSearchTag(list...)
	// 3. 异步同步标签关系表 (规范化存储以供高性能联动)
	go SyncMovieTagRel(latestInfos)

	return keyToMid
}

// SyncMovieTagRel 同步影片与各维度标签的关系表 (供智能联动筛选使用)
func SyncMovieTagRel(list []model.SearchInfo) {
	if len(list) == 0 {
		return
	}

	var mids []int64
	for _, v := range list {
		mids = append(mids, v.Mid)
	}

	// 在独立事务中处理
	_ = db.Mdb.Transaction(func(tx *gorm.DB) error {
		// 1. 清理旧的关系
		tx.Where("mid IN ?", mids).Delete(&model.MovieTagRel{})

		// 2. 构造新的关系集合
		var rels []model.MovieTagRel
		for _, v := range list {
			// Area
			if v.Area != "" && v.Area != "全部" && v.Area != "其它" {
				rels = append(rels, model.MovieTagRel{Mid: v.Mid, TagType: "Area", TagValue: v.Area})
			}
			// Language
			if v.Language != "" && v.Language != "全部" && v.Language != "其它" {
				rels = append(rels, model.MovieTagRel{Mid: v.Mid, TagType: "Language", TagValue: v.Language})
			}
			// Year
			if v.Year > 0 {
				rels = append(rels, model.MovieTagRel{Mid: v.Mid, TagType: "Year", TagValue: fmt.Sprint(v.Year)})
			}
			// Plot (从 class_tag 中拆分)
			if v.ClassTag != "" {
				plots := strings.SplitSeq(v.ClassTag, ",")
				for p := range plots {
					p = strings.TrimSpace(p)
					if p != "" && p != "全部" && p != "其它" {
						rels = append(rels, model.MovieTagRel{Mid: v.Mid, TagType: "Plot", TagValue: p})
					}
				}
			}
		}

		if len(rels) > 0 {
			tx.CreateInBatches(&rels, 500)
		}
		return nil
	})
}

func SaveSearchInfo(s model.SearchInfo) error {
	// 同样采用 ContentKey 去重策略，确保 Mid 唯一归约
	err := db.Mdb.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "content_key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"source_id", "cid", "pid", "name", "sub_title", "c_name", "class_tag",
			"area", "language", "year", "initial", "score",
			"update_stamp", "hits", "state", "remarks", "release_stamp",
			"picture", "actor", "director", "blurb", "updated_at",
		}),
	}).Create(&s).Error

	if err == nil {
		// 记录映射
		var info model.SearchInfo
		db.Mdb.Where("content_key = ?", s.ContentKey).First(&info)
		db.Mdb.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "source_id"}, {Name: "source_mid"}},
			DoUpdates: clause.AssignmentColumns([]string{"global_mid", "updated_at"}),
		}).Create(&model.MovieSourceMapping{
			SourceId:  s.SourceId,
			SourceMid: s.Mid,
			GlobalMid: info.Mid,
		})
		// 同步标签关系表 (规范化)
		go SyncMovieTagRel([]model.SearchInfo{info})
	}

	BatchHandleSearchTag(s)
	// 单条记录更新也需要清除该分类的搜索标签缓存，防止联动数据过时
	ClearSearchTagsCache(s.Pid)
	return err
}

func SaveDetails(id string, list []model.MovieDetail) error {
	var infoList []model.SearchInfo
	for _, v := range list {
		infoList = append(infoList, ConvertSearchInfo(id, v))
	}
	keyToMid := BatchSaveOrUpdate(infoList)

	var details []model.MovieDetailInfo
	for _, v := range list {
		// 获取内容标识
		info := ConvertSearchInfo(id, v)
		globalMid, ok := keyToMid[info.ContentKey]
		if !ok {
			globalMid = v.Id
		}

		v.Id = globalMid // 将详情内的 ID 也归约到 Global ID
		data, _ := json.Marshal(v)
		details = append(details, model.MovieDetailInfo{Mid: globalMid, SourceId: id, Content: string(data)})
	}

	if len(details) > 0 {
		return db.Mdb.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "mid"}},
			DoUpdates: clause.AssignmentColumns([]string{"source_id", "content", "updated_at"}),
		}).Create(&details).Error
	}
	return nil
}

func SaveDetail(id string, detail model.MovieDetail) error {
	searchInfo := ConvertSearchInfo(id, detail)
	// 模拟 BatchSaveOrUpdate 逻辑获取 GlobalMid
	if err := db.Mdb.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "content_key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"source_id", "cid", "pid", "name", "sub_title", "c_name", "class_tag",
			"area", "language", "year", "initial", "score",
			"update_stamp", "hits", "state", "remarks", "release_stamp",
			"picture", "actor", "director", "blurb", "updated_at",
		}),
	}).Create(&searchInfo).Error; err != nil {
		return err
	}

	// 查回最终生效的 Mid
	var info model.SearchInfo
	db.Mdb.Where("content_key = ?", searchInfo.ContentKey).First(&info)
	globalMid := info.Mid

	// 映射记录
	db.Mdb.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "source_id"}, {Name: "source_mid"}},
		DoUpdates: clause.AssignmentColumns([]string{"global_mid", "updated_at"}),
	}).Create(&model.MovieSourceMapping{
		SourceId:  id,
		SourceMid: detail.Id,
		GlobalMid: globalMid,
	})

	detail.Id = globalMid
	data, _ := json.Marshal(detail)
	err := db.Mdb.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "mid"}},
		DoUpdates: clause.AssignmentColumns([]string{"source_id", "content", "updated_at"}),
	}).Create(&model.MovieDetailInfo{Mid: globalMid, SourceId: id, Content: string(data)}).Error

	if err == nil {
		ClearSearchTagsCache(detail.Pid)
		db.Rdb.Del(db.Cxt, config.ActiveCategoryTreeKey)
	}
	return err
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
		if err := db.Mdb.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "source_id"}, {Name: "movie_key"}},
			DoUpdates: clause.AssignmentColumns([]string{"content", "updated_at"}),
		}).Create(&playlists).Error; err != nil {
			log.Printf("SaveSitePlayList Error: %v", err)
			return err
		}
		log.Printf("[Playlist] 为站点 %s 保存了 %d 条记录\n", id, len(playlists))
	}
	return nil
}

// DeletePlaylistBySourceId 根据来源站点 ID 删除所有关联的播放列表资源
func DeletePlaylistBySourceId(sourceId string) error {
	return db.Mdb.Where("source_id = ?", sourceId).Delete(&model.MoviePlaylist{}).Error
}

// ========= Tag Operations =========

var initializedPids sync.Map

func BatchHandleSearchTag(infos ...model.SearchInfo) {
	if len(infos) == 0 {
		return
	}

	// 1. 批量处理分类级的静态标签和分类节点 (按 Pid 去重)
	pids := make(map[int64]bool)
	for _, info := range infos {
		if info.Pid > 0 {
			pids[info.Pid] = true
		}
	}

	for pid := range pids {
		ensureStaticTagsForPid(pid)
	}

	// 2. 处理每部影片的动态标签 (Category, Plot, Area, Language)
	for _, info := range infos {
		if info.Pid <= 0 {
			continue
		}
		// 新增：将所属分类 ID 作为一个动态标签，只有该分类下有视频时才会在筛选栏出现
		// 注意：如果 Cid == Pid，说明是直接关联到大类的异常数据，这种情况下不生成 Category 标签，避免大类名称出现在筛选子项中
		if info.Cid > 0 && info.Cid != info.Pid {
			// 查找分类名称
			cName := info.CName
			if cName == "" {
				// 如果没有 CName (SearchInfo 结构可能未填)，则回退到静态查询或缓存
				// 这里假设 SearchInfo.CName 是有的
			}
			HandleSearchTags(cName, "Category", info.Pid, fmt.Sprint(info.Cid))
		}
		HandleSearchTags(info.ClassTag, "Plot", info.Pid)
		HandleSearchTags(info.Area, "Area", info.Pid)
		HandleSearchTags(info.Language, "Language", info.Pid)
		// 新增：动态年分标签
		if info.Year > 0 {
			HandleSearchTags(fmt.Sprint(info.Year), "Year", info.Pid)
		}
	}

	// 新增：自动清空搜索标签缓存，确保新入库的影片能实时在筛选联动中体现
	ClearAllSearchTagsCache()
}

func SaveSearchTag(search model.SearchInfo) {
	BatchHandleSearchTag(search)
}

func ensureStaticTagsForPid(pid int64) {
	// 1. 内存缓存检查，避免频繁查库
	if _, ok := initializedPids.Load(pid); ok {
		return
	}

	// 此时不再初始化 Year，年份将随数据动态生成

	// 3. 初始化 Initial (A-Z)
	var initialItems []model.SearchTagItem
	for i := 65; i <= 90; i++ {
		v := string(rune(i))
		initialItems = append(initialItems, model.SearchTagItem{Pid: pid, TagType: "Initial", Name: v, Value: v, Score: int64(90 - i)})
	}
	db.Mdb.Clauses(clause.OnConflict{DoNothing: true}).Create(&initialItems)

	// 4. 初始化 Sort
	sortItems := []model.SearchTagItem{
		{Pid: pid, TagType: "Sort", Name: "时间排序", Value: "update_stamp", Score: 3},
		{Pid: pid, TagType: "Sort", Name: "人气排序", Value: "hits", Score: 2},
		{Pid: pid, TagType: "Sort", Name: "评分排序", Value: "score", Score: 1},
		{Pid: pid, TagType: "Sort", Name: "最新上映", Value: "release_stamp", Score: 0},
	}
	db.Mdb.Clauses(clause.OnConflict{DoNothing: true}).Create(&sortItems)

	// 标记为已初始化
	initializedPids.Store(pid, true)
}

func HandleSearchTags(preTags string, tagType string, pid int64, customValues ...string) {
	preTags = regexp.MustCompile(`[\s\n\r]+`).ReplaceAllString(preTags, "")

	upsert := func(v string, customVal ...string) {
		v = strings.TrimSpace(v)
		if v == "" || v == "其它" {
			return
		}

		val := v
		if len(customVal) > 0 {
			val = customVal[0]
		}

		score := int64(1)
		doUpdates := clause.Assignments(map[string]any{"score": gorm.Expr("score + 1")})

		// 年份特殊处理：分值即年份，确保按年份倒序排列
		if tagType == "Year" {
			y, _ := strconv.Atoi(v)
			score = int64(y)
			doUpdates = clause.Assignments(map[string]any{"score": score}) // 年份分数固定为年份值
		}

		db.Mdb.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "pid"}, {Name: "tag_type"}, {Name: "value"}},
			DoUpdates: doUpdates,
		}).Create(&model.SearchTagItem{Pid: pid, TagType: tagType, Name: v, Value: val, Score: score})
	}

	if tagType == "Category" && len(customValues) > 0 {
		upsert(preTags, customValues[0])
		return
	}

	if preTags == "" || preTags == "其它" {
		return
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

func ConvertSearchInfo(sourceId string, detail model.MovieDetail) model.SearchInfo {
	score, _ := strconv.ParseFloat(detail.DbScore, 64)
	stamp, _ := time.ParseInLocation(time.DateTime, detail.UpdateTime, time.Local)
	year, err := strconv.ParseInt(regexp.MustCompile(`[1-9][0-9]{3}`).FindString(detail.ReleaseDate), 10, 64)
	if err != nil {
		year = 0
	}

	// 生成内容指纹：优先使用豆瓣 ID，无豆瓣 ID 则使用名称哈希
	var contentKey string
	if detail.DbId != 0 {
		contentKey = fmt.Sprintf("dbid_%d", detail.DbId)
	} else {
		contentKey = fmt.Sprintf("name_%s", utils.GenerateHashKey(detail.Name))
	}

	// 关键修复 1：基于分类名称 (CName) 从数据库获取我们维护的稳定 Cid
	// 这样即使不同站点的 Cid 不同，只要名称一致（如“动作片”），就能归约为同一个分类 ID
	resolvedCid := GetCidByName(detail.CName)
	if resolvedCid == 0 {
		// 兜底：如果名称没对上，暂时保留原始 Cid (虽然可能导致归位失败，但好过丢掉)
		resolvedCid = detail.Cid
	}

	// 关键修复 2：根据解析后的 Cid 实时查询正确的 Pid
	// 这确保了影片能正确归类到“电影”、“电视剧”等智能根分类下，从而让筛选标签正常显示
	correctPid := GetRootId(resolvedCid)

	return model.SearchInfo{
		Mid:          detail.Id,
		ContentKey:   contentKey,
		SourceId:     sourceId,
		Cid:          resolvedCid,
		Pid:          correctPid,
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
		DbId:         detail.DbId,
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
func GetMovieListByPid(pid int64, page *dto.Page) []model.MovieBasicInfo {
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
func GetMovieListByCid(cid int64, page *dto.Page) []model.MovieBasicInfo {
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

func SearchFilmKeyword(keyword string, page *dto.Page) []model.SearchInfo {
	var searchList []model.SearchInfo
	var count int64
	db.Mdb.Model(&model.SearchInfo{}).Where("name LIKE ?", fmt.Sprint(`%`, keyword, `%`)).Or("sub_title LIKE ?", fmt.Sprint(`%`, keyword, `%`)).Count(&count)
	page.Total = int(count)
	page.PageCount = int((page.Total + page.PageSize - 1) / page.PageSize)

	db.Mdb.Limit(page.PageSize).Offset((page.Current-1)*page.PageSize).
		Where("name LIKE ?", fmt.Sprintf(`%%%s%%`, keyword)).Or("sub_title LIKE ?", fmt.Sprintf(`%%%s%%`, keyword)).Order("year DESC, update_stamp DESC").Find(&searchList)

	return searchList
}

func GetRelateMovieBasicInfo(search model.SearchInfo, page *dto.Page) []model.MovieBasicInfo {
	offset := page.Current
	if offset <= 0 {
		offset = 0
	} else {
		offset = (offset - 1) * page.PageSize
	}
	// 1. 基于分词 (Tokenization) 的核心特征字提取
	rawName := search.Name
	// 定义常用切割符 (如遇到冒号、破折号、空格、括号、特定关键字等，即认为其前部为核心标题)
	delimiters := []string{"：", ":", "·", " - ", "—", " ", "（", "(", "[", "【", "第", "剧场版", "部", "季", "之"}
	coreToken := rawName

	// 寻找最早出现的切割符位置
	minIdx := len(rawName)
	for _, d := range delimiters {
		if idx := strings.Index(rawName, d); idx > 0 && idx < minIdx {
			minIdx = idx
		}
	}

	if minIdx < len(rawName) {
		coreToken = rawName[:minIdx]
	}

	coreToken = strings.TrimSpace(coreToken)

	// 最小长度保障：若因连续符号导致拆分后无词，回退到原名前4个字符
	if len([]rune(coreToken)) < 1 && len([]rune(rawName)) > 2 {
		coreToken = string([]rune(rawName)[:4])
	}

	// 如果拆分出来的 coreToken 只有 1 个字符（例如中文单字），可能区分度不够，
	// 回退到至少取两个字符（如果有的话）。
	if len([]rune(coreToken)) == 1 && len([]rune(rawName)) > 1 {
		coreToken = string([]rune(rawName)[:2])
	}

	nameLike := fmt.Sprintf("%%%s%%", coreToken)
	prefixLike := fmt.Sprintf("%s%%", coreToken)

	// 2. 构造查询条件
	// 基础池：扩大候选集（名称包含核心词，或者标签相似）
	condition := db.Mdb.Where("name LIKE ? OR sub_title LIKE ?", nameLike, nameLike)

	search.ClassTag = strings.ReplaceAll(search.ClassTag, " ", "")
	classTags := make([]string, 0)
	if strings.Contains(search.ClassTag, ",") {
		classTags = strings.Split(search.ClassTag, ",")
	} else if strings.Contains(search.ClassTag, "/") {
		classTags = strings.Split(search.ClassTag, "/")
	} else if strings.TrimSpace(search.ClassTag) != "" {
		classTags = []string{search.ClassTag}
	}

	for _, tag := range classTags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		condition = condition.Or("class_tag LIKE ?", fmt.Sprintf("%%%s%%", tag))
	}

	// 3. 执行带高阶权重的原生 SQL 排序查询 (彻底解决 Gorm 链式参数错位 Bug)
	var list []model.SearchInfo
	args := []interface{}{search.Pid, search.Mid}

	// 构建 WHERE 子句
	whereSQL := "WHERE pid = ? AND mid != ? AND deleted_at IS NULL"
	whereSQL += " AND ((name LIKE ? OR sub_title LIKE ?)"
	args = append(args, nameLike, nameLike)

	tags := strings.Split(search.ClassTag, ",")
	for _, t := range tags {
		if tag := strings.TrimSpace(t); tag != "" {
			whereSQL += " OR class_tag LIKE ?"
			args = append(args, "%"+tag+"%")
		}
	}
	whereSQL += ")"

	// 构建 ORDER BY 子句
	sortSQL := `ORDER BY 
		(name = ?) DESC, 
		(name LIKE ?) DESC, 
		(name LIKE ?) DESC, 
		(cid = ?) DESC, 
		update_stamp DESC`
	args = append(args, coreToken, prefixLike, nameLike, search.Cid)

	// 最终 SQL 组装 (不使用 Gorm Builder 而是手动注入参数列表)
	finalSQL := fmt.Sprintf("SELECT * FROM search_info %s %s LIMIT ? OFFSET ?", whereSQL, sortSQL)
	args = append(args, page.PageSize, offset)

	if err := db.Mdb.Raw(finalSQL, args...).Scan(&list).Error; err != nil {
		log.Println("GetRelateMovieBasicInfo Raw SQL Error:", err)
		return make([]model.MovieBasicInfo, 0)
	}

	// 4. 重大兜底机制：如果没有匹配到任何相关结果 (Cid/Name/Tags 全落空)，推荐同二级分类的最新影片
	if len(list) == 0 {
		db.Mdb.Model(&model.SearchInfo{}).
			Where("cid = ? AND mid != ?", search.Cid, search.Mid).
			Order("update_stamp DESC").
			Offset(offset).Limit(page.PageSize).
			Find(&list)
	}

	basicList := make([]model.MovieBasicInfo, 0)
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

func GetTagsByTitle(pid int64, tagType string, activeValues map[string]bool, stickyValue string) []string {
	var tags []string

	var items []model.SearchTagItem
	// 对于 Plot, Area, Language, Category 提高展示上限到 30
	limit := 30
	if tagType == "Plot" {
		limit = 11 // 剧情类保持较少
	}

	query := db.Mdb.Where("pid = ? AND tag_type = ?", pid, tagType)
	if tagType == "Category" {
		// 排除大类本身 (Value 等于 Pid 的项)
		query = query.Where("value != ?", fmt.Sprint(pid))
	}
	query.Order("score DESC").Limit(limit).Find(&items)

	// 核心逻辑：即时存在性校验 (Result-Driven)
	for _, item := range items {
		// 粘性逻辑：如果是当前选中的，无论如何都显示
		if stickyValue != "" && item.Value == stickyValue {
			tags = append(tags, fmt.Sprintf("%s:%s", item.Name, item.Value))
			continue
		}

		// 存在性判断
		if activeValues != nil {
			// 如果外部传入了精准的已锁定标签集（如联动查询时），直接使用
			if activeValues[item.Value] {
				tags = append(tags, fmt.Sprintf("%s:%s", item.Name, item.Value))
			}
		} else if tagType == "Plot" {
			// Plot 在没有预计算 activeValues 时走模糊匹配兜底 (例如简单的面板初始化)
			var exists int64
			db.Mdb.Model(&model.SearchInfo{}).Where("pid = ? AND class_tag LIKE ?", pid, fmt.Sprintf("%%%s%%", item.Value)).Limit(1).Count(&exists)
			if exists > 0 {
				tags = append(tags, fmt.Sprintf("%s:%s", item.Name, item.Value))
			}
		} else {
			// 其他类型在没有 activeValues 时暂时不做即时校验 (通常走缓存)
			tags = append(tags, fmt.Sprintf("%s:%s", item.Name, item.Value))
		}
	}

	// 关键修复：如果数据库中缺失静态标签（Year, Sort），自动提供默认值
	if len(tags) == 0 {
		switch tagType {
		case "Sort":
			tags = []string{
				"时间排序:update_stamp",
				"人气排序:hits",
				"评分排序:score",
				"最新上映:release_stamp",
			}
		}
	}
	return tags
}

// GetTopTagValues 获取某个维度的“热门/展示”值集，用于“其它”逻辑的排除参考
func GetTopTagValues(pid int64, tagType string) []string {
	var items []model.SearchTagItem
	limit := 30
	if tagType == "Plot" {
		limit = 11
	}

	query := db.Mdb.Where("pid = ? AND tag_type = ?", pid, tagType)
	if tagType == "Category" {
		query = query.Where("value != ?", fmt.Sprint(pid))
	}
	query.Order("score DESC").Limit(limit).Find(&items)

	var vals []string
	for _, item := range items {
		vals = append(vals, item.Value)
	}
	return vals
}

func HandleTagStr(pid int64, title string, activeValues map[string]bool, stickyValue string, tags ...string) []map[string]string {
	list := make([]map[string]string, 0)

	// 除排序外，默认都有“全部”选项
	hasAll := !strings.EqualFold(title, "Sort")
	if hasAll {
		list = append(list, map[string]string{"Name": "全部", "Value": ""})
	}

	hotValueMap := make(map[string]bool)
	for _, t := range tags {
		if sl := strings.Split(t, ":"); len(sl) > 1 {
			list = append(list, map[string]string{"Name": sl[0], "Value": sl[1]})
			hotValueMap[sl[1]] = true
		}
	}

	// 针对特定类型，恢复显示“其它”选项
	if strings.EqualFold(title, "Plot") || strings.EqualFold(title, "Area") ||
		strings.EqualFold(title, "Language") || strings.EqualFold(title, "Year") {

		// 粘性逻辑：如果当前选中的就是“其它”，必须强制显示
		if stickyValue == "其它" {
			list = append(list, map[string]string{"Name": "其它", "Value": "其它"})
		} else if activeValues == nil {
			// 如果没有联动上下文（如初次加载或兜底），默认显示“其它”以保证入口存在
			list = append(list, map[string]string{"Name": "其它", "Value": "其它"})
		} else {
			// 【性能优化】在联动上下文中，不再执行 Count 语句
			// 逻辑：如果当前维度的有效值集 (activeValues) 中包含不在热门列表 (hotValueMap) 里的值，就显示“其它”
			hasOthers := false
			for val := range activeValues {
				if !hotValueMap[val] {
					hasOthers = true
					break
				}
			}
			if hasOthers {
				list = append(list, map[string]string{"Name": "其它", "Value": "其它"})
			}
		}
	}

	return list
}

// GetSearchTag 获取搜索标签 (带联动感知与复合 Redis 缓存)
func GetSearchTag(st model.SearchTagsVO) map[string]any {
	pid := st.Pid
	// 1. 生成复合缓存 Key (含所有筛选维度)
	// 格式: SearchTags:{pid}:{cid}:{area}:{language}:{year}:{plot}
	// 这里直接使用原值拼接（或简单清理），比 GenerateHashKey 更快且更直观
	cacheKey := fmt.Sprintf("SearchTags:%d:%d:%s:%s:%s:%s",
		pid, st.Cid,
		st.Area, st.Language, st.Year, st.Plot,
	)

	// 2. 尝试从 Redis 获取缓存
	if data, err := db.Rdb.Get(db.Cxt, cacheKey).Result(); err == nil && data != "" {
		var res map[string]any
		if json.Unmarshal([]byte(data), &res) == nil {
			return res
		}
	}

	res := make(map[string]any)
	sortList := []string{"Category", "Plot", "Area", "Language", "Year", "Sort"}
	res["titles"] = map[string]string{
		"Category": "类型",
		"Plot":     "剧情",
		"Area":     "地区",
		"Language": "语言",
		"Year":     "年份",
		"Sort":     "排序",
	}

	tagMap := make(map[string]any)
	activeSortList := make([]string, 0)

	// 多维联动逻辑 (Elegant Faceted)：计算每一行的选项时，锁定其他所有已选中的维度的关系映射
	for _, t := range sortList {
		var sticky string
		var activeSet map[string]bool

		// 基础过滤集：限定在当前大类 (PID) 下
		// 使用 search_info 开启主查询，通过 Joins 联动其他维度
		query := db.Mdb.Table("search_info").Select("search_info.mid").Where("search_info.pid = ?", pid)

		// 联动核心：计算当前行时，应用除本行外其他维度的已选条件
		if t != "Category" && st.Cid > 0 {
			if IsRootCategory(st.Cid) {
				query = query.Where("search_info.pid = ?", st.Cid)
			} else {
				query = query.Where("search_info.cid = ?", st.Cid)
			}
		}
		if t != "Area" && st.Area != "" && st.Area != "全部" {
			if st.Area == "其它" {
				topVals := GetTopTagValues(pid, "Area")
				if len(topVals) > 0 {
					query = query.Where("search_info.area NOT IN ?", topVals)
				}
			} else {
				query = query.Joins("JOIN movie_tag_rel r_area ON r_area.mid = search_info.mid AND r_area.tag_type = 'Area' AND r_area.tag_value = ?", st.Area)
			}
		}
		if t != "Language" && st.Language != "" && st.Language != "全部" {
			if st.Language == "其它" {
				topVals := GetTopTagValues(pid, "Language")
				if len(topVals) > 0 {
					query = query.Where("search_info.language NOT IN ?", topVals)
				}
			} else {
				query = query.Joins("JOIN movie_tag_rel r_lang ON r_lang.mid = search_info.mid AND r_lang.tag_type = 'Language' AND r_lang.tag_value = ?", st.Language)
			}
		}
		if t != "Year" && st.Year != "" && st.Year != "全部" {
			if st.Year == "其它" {
				topVals := GetTopTagValues(pid, "Year")
				if len(topVals) > 0 {
					query = query.Where("search_info.year NOT IN ?", topVals)
				}
			} else {
				query = query.Joins("JOIN movie_tag_rel r_year ON r_year.mid = search_info.mid AND r_year.tag_type = 'Year' AND r_year.tag_value = ?", st.Year)
			}
		}
		if t != "Plot" && st.Plot != "" && st.Plot != "全部" {
			if st.Plot == "其它" {
				topVals := GetTopTagValues(pid, "Plot")
				for _, v := range topVals {
					query = query.Where("search_info.class_tag NOT LIKE ?", fmt.Sprintf("%%%s%%", v))
				}
			} else {
				query = query.Joins("JOIN movie_tag_rel r_plot ON r_plot.mid = search_info.mid AND r_plot.tag_type = 'Plot' AND r_plot.tag_value = ?", st.Plot)
			}
		}

		switch t {
		case "Category":
			sticky = fmt.Sprint(st.Cid)
			if st.Cid == 0 {
				sticky = ""
			}
			var vals []int64
			// 分类联动依然基于 search_infos 表查分类 ID
			db.Mdb.Model(&model.SearchInfo{}).Where("mid IN (?)", query).Distinct().Pluck("cid", &vals)
			activeSet = make(map[string]bool)
			for _, v := range vals {
				activeSet[fmt.Sprint(v)] = true
			}
		case "Sort":
			// 排序行不需要开启联动感知
		default:
			// Plot, Area, Language, Year 统一从 movie_tag_rel 表获取联动后的有效标签集
			switch t {
			case "Plot":
				sticky = st.Plot
			case "Area":
				sticky = st.Area
			case "Language":
				sticky = st.Language
			case "Year":
				sticky = st.Year
			}

			var vals []string
			db.Mdb.Model(&model.MovieTagRel{}).
				Where("tag_type = ? AND mid IN (?)", t, query).
				Distinct().Pluck("tag_value", &vals)

			activeSet = make(map[string]bool)
			for _, v := range vals {
				activeSet[v] = true
			}
		}

		tags := HandleTagStr(pid, t, activeSet, sticky, GetTagsByTitle(pid, t, activeSet, sticky)...)
		if t == "Sort" || len(tags) > 1 || (sticky != "" && sticky != "全部") {
			tagMap[t] = tags
			activeSortList = append(activeSortList, t)
		}
	}
	res["sortList"] = activeSortList
	res["tags"] = tagMap

	// 3. 写入 Redis 缓存 (所有组合均缓存 24 小时)
	if data, err := json.Marshal(res); err == nil {
		db.Rdb.Set(db.Cxt, cacheKey, string(data), time.Hour*24)
	}

	return res
}

func GetSearchOptions(st model.SearchTagsVO) map[string]any {
	// 复用 GetSearchTag 的逻辑
	full := GetSearchTag(st)
	if tags, ok := full["tags"].(map[string]any); ok {
		// 返回业务需要的四个核心维度
		res := make(map[string]any)
		for _, t := range []string{"Plot", "Area", "Language", "Year"} {
			res[t] = tags[t]
		}
		return res
	}

	// 回退逻辑 (兜底)
	tagMap := make(map[string]any)
	for _, t := range []string{"Plot", "Area", "Language", "Year"} {
		tagMap[t] = HandleTagStr(st.Pid, t, nil, "", GetTagsByTitle(st.Pid, t, nil, "")...)
	}
	return tagMap
}

func GetSearchPage(s model.SearchVo) []model.SearchInfo {
	query := db.Mdb.Model(&model.SearchInfo{})
	if s.Name != "" {
		query = query.Where("name LIKE ?", fmt.Sprintf("%%%s%%", s.Name))
	}

	// 严格精准分类过滤
	if s.Cid > 0 {
		if IsRootCategory(s.Cid) {
			query = query.Where("pid = ?", s.Cid)
		} else {
			// 严格匹配子类 ID，不进行任何向下包含或通用记录兼容
			query = query.Where("cid = ?", s.Cid)
		}
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
	if s.Year > 0 {
		query = query.Where("year = ?", s.Year)
	}
	switch s.Remarks {
	case "完结":
		query = query.Where("remarks IN ?", []string{"完结", "HD"})
	case "":
	default:
		query = query.Not(map[string]any{"remarks": []string{"完结", "HD"}})
	}
	if s.BeginTime > 0 {
		query = query.Where("update_stamp >= ? ", s.BeginTime)
	}
	if s.EndTime > 0 {
		query = query.Where("update_stamp <= ? ", s.EndTime)
	}

	dto.GetPage(query, s.Paging)
	var sl []model.SearchInfo
	if err := query.Limit(s.Paging.PageSize).Offset((s.Paging.Current - 1) * s.Paging.PageSize).Find(&sl).Error; err != nil {
		log.Printf("GetSearchPage Error: %v", err)
		return nil
	}
	return sl
}

func GetSearchInfosByTags(st model.SearchTagsVO, page *dto.Page) []model.SearchInfo {
	qw := db.Mdb.Model(&model.SearchInfo{})
	t := reflect.TypeFor[model.SearchTagsVO]()
	v := reflect.ValueOf(st)

	// 记录是否已经处理了分类过滤，防止 Pid 和 Cid 产生冲突
	categoryFiltered := false

	for i := 0; i < t.NumField(); i++ {
		value := v.Field(i).Interface()
		fieldName := t.Field(i).Name
		k := strings.ToLower(fieldName)

		if !dto.IsEmpty(value) {
			switch k {
			case "pid", "cid":
				if categoryFiltered {
					continue
				}
				// 严格逻辑与 GetSearchPage 保持高度一致
				targetCid := st.Cid
				if targetCid > 0 {
					if IsRootCategory(targetCid) {
						qw = qw.Where("pid = ?", targetCid)
					} else {
						// 严格匹配子分类，去掉通用记录向下包含逻辑
						qw = qw.Where("cid = ?", targetCid)
					}
				} else if st.Pid > 0 {
					qw = qw.Where("pid = ?", st.Pid)
				}
				categoryFiltered = true
			case "year":
				if vStr, ok := value.(string); ok && strings.EqualFold(vStr, "其它") {
					topVals := GetTopTagValues(st.Pid, fieldName)
					if len(topVals) > 0 {
						qw = qw.Where(fmt.Sprintf("%s NOT IN ?", k), topVals)
					}
					break
				}
				qw = qw.Where(fmt.Sprintf("%s = ?", k), value)
			case "area", "language":
				if vStr, ok := value.(string); ok && strings.EqualFold(vStr, "其它") {
					topVals := GetTopTagValues(st.Pid, fieldName)
					if len(topVals) > 0 {
						qw = qw.Where(fmt.Sprintf("%s NOT IN ?", k), topVals)
					}
					break
				}
				qw = qw.Where(fmt.Sprintf("%s = ?", k), value)
			case "plot":
				if vStr, ok := value.(string); ok && strings.EqualFold(vStr, "其它") {
					topVals := GetTopTagValues(st.Pid, fieldName)
					for _, v := range topVals {
						qw = qw.Where("class_tag NOT LIKE ?", fmt.Sprintf("%%%v%%", v))
					}
					break
				}
				qw = qw.Where("class_tag LIKE ?", fmt.Sprintf("%%%v%%", value))
			case "sort":
				if sVal, ok := value.(string); ok && strings.EqualFold(sVal, "release_stamp") {
					qw.Order("year DESC, release_stamp DESC")
				} else {
					qw.Order(fmt.Sprintf("%v DESC", value))
				}
			default:
				break
			}
		}
	}

	dto.GetPage(qw, page)
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
	// 获取记录所在分类，以便后续清除缓存
	info := GetSearchInfoById(id)

	// 开启事务保证清理的一致性
	err := db.Mdb.Transaction(func(tx *gorm.DB) error {
		// 1. 删除检索信息
		if err := tx.Where("mid = ?", id).Delete(&model.SearchInfo{}).Error; err != nil {
			return err
		}
		// 2. 删除主站详情记录
		if err := tx.Where("mid = ?", id).Delete(&model.MovieDetailInfo{}).Error; err != nil {
			return err
		}
		// 3. 删除来源映射记录
		if err := tx.Where("global_mid = ?", id).Delete(&model.MovieSourceMapping{}).Error; err != nil {
			return err
		}
		// 4. 删除相关的 Banner (横幅)
		if err := tx.Where("mid = ?", id).Delete(&model.Banner{}).Error; err != nil {
			return err
		}
		// 5. 删除标签关系
		if err := tx.Where("mid = ?", id).Delete(&model.MovieTagRel{}).Error; err != nil {
			return err
		}
		return nil
	})

	// 清除对应分类的搜索标签缓存
	if err == nil && info != nil {
		ClearSearchTagsCache(info.Pid)
	}

	return err
}

func ShieldFilmSearch(cid int64) error {
	// 获取相关 MID 列表以便从关系表中删除
	var mids []int64
	db.Mdb.Model(&model.SearchInfo{}).Where("cid = ?", cid).Pluck("mid", &mids)

	err := db.Mdb.Transaction(func(tx *gorm.DB) error {
		// 1. 软删除检索信息
		if err := tx.Where("cid = ?", cid).Delete(&model.SearchInfo{}).Error; err != nil {
			return err
		}
		// 2. 硬删除对应的标签关系 (保持查询结果干净)
		if len(mids) > 0 {
			if err := tx.Where("mid IN ?", mids).Delete(&model.MovieTagRel{}).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		log.Printf("ShieldFilmSearch Error: %v", err)
		return err
	}

	// 清除对应 Pid 的搜索标签缓存
	if pId := GetParentId(cid); pId > 0 {
		ClearSearchTagsCache(pId)
	}
	return nil
}

func RecoverFilmSearch(cid int64) error {
	err := db.Mdb.Transaction(func(tx *gorm.DB) error {
		// 1. 恢复检索信息
		if err := tx.Model(&model.SearchInfo{}).Unscoped().Where("cid = ?", cid).Update("deleted_at", nil).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		log.Printf("RecoverFilmSearch Error: %v", err)
		return err
	}

	// 2. 异步重新同步该分类下所有影片的标签关系 (因为 Shield 时被硬删除了)
	var infos []model.SearchInfo
	db.Mdb.Where("cid = ?", cid).Find(&infos)
	if len(infos) > 0 {
		go SyncMovieTagRel(infos)
	}

	// 清除对应 Pid 的搜索标签缓存
	if pId := GetParentId(cid); pId > 0 {
		ClearSearchTagsCache(pId)
	}
	return nil
}

// ClearSearchTagsCache 清除特定分类的所有复合搜索标签缓存
func ClearSearchTagsCache(pid int64) {
	// 使用通配符前缀：SearchTags:{pid}:*
	pattern := fmt.Sprintf("SearchTags:%d:*", pid)
	ctx := db.Cxt
	iter := db.Rdb.Scan(ctx, 0, pattern, config.MaxScanCount).Iterator()
	for iter.Next(ctx) {
		db.Rdb.Del(ctx, iter.Val())
	}
	// 同时兼容旧版/基础版 key: SearchTags:{pid}
	db.Rdb.Del(ctx, fmt.Sprintf(config.SearchTagsKey, pid))
}

// ClearTVBoxConfigCache 清除 TVBox 配置缓存
func ClearTVBoxConfigCache() {
	db.Rdb.Del(db.Cxt, config.TVBoxConfigCacheKey)
}

// ClearAllSearchTagsCache 清除所有分类的搜索标签缓存 (扫描清理)
func ClearAllSearchTagsCache() {
	// 基于 config 里的模板生成通配符，防止硬编码 prefix 不一致
	pattern := strings.Replace(config.SearchTagsKey, "%d", "*", 1)
	keys, err := db.Rdb.Keys(db.Cxt, pattern).Result()
	if err == nil && len(keys) > 0 {
		db.Rdb.Del(db.Cxt, keys...)
	}
	ClearTVBoxConfigCache()
}

// FilmZero 删除所有库存数据 (包含 MySQL 持久化表)
func FilmZero() {
	// 清理 MySQL
	tables := []string{
		model.TableMovieDetail,
		model.TableSearchInfo,
		model.TableMoviePlaylist,
		model.TableCategory,
		model.TableVirtualPicture,
		model.TableSearchTag,
		model.TableBanners,
		model.TableMovieTagRel,
	}
	for _, t := range tables {
		if err := db.Mdb.Exec(fmt.Sprintf("TRUNCATE table %s", t)).Error; err != nil {
			log.Printf("TRUNCATE TABLE %s Error: %v\n", t, err)
		}
	}

	// 3. 同步清理采集失败记录，确保彻底清空
	TruncateRecordTable()

	// 4. 清除所有 Redis 缓存
	ClearCategoryCache()
}

// MasterFilmZero 仅清理主站相关数据 (search_infos / movie_detail_infos / category)
// 保留附属站 movie_playlists 数据，用于主站切换时防止附属站数据丢失
func MasterFilmZero() {
	tables := []string{
		model.TableSearchInfo,
		model.TableMovieDetail,
		model.TableCategory,
		model.TableVirtualPicture,
		model.TableSearchTag,
		model.TableBanners,
		model.TableMovieTagRel,
	}
	for _, t := range tables {
		if err := db.Mdb.Exec(fmt.Sprintf("TRUNCATE table %s", t)).Error; err != nil {
			log.Printf("TRUNCATE TABLE %s Error: %v\n", t, err)
		}
	}

	// 清除所有 Redis 缓存
	ClearCategoryCache()
}

// CleanEmptyFilms 清理所有片名为空的无效记录
func CleanEmptyFilms() int64 {
	var infos []model.SearchInfo
	db.Mdb.Where("name = ? OR name IS NULL", "").Find(&infos)
	if len(infos) == 0 {
		return 0
	}
	for _, info := range infos {
		_ = DelFilmSearch(info.Mid)
		ClearSearchTagsCache(info.Pid)
	}
	// DelFilmSearch 内部会精确清除对应的 Pid 缓存
	return int64(len(infos))
}

// CleanOrphanPlaylists 清理 movie_playlists 中与 search_infos 不匹配的孤儿记录
// 仅当 search_infos 存在数据时执行，避免主站清空后误删全部播放列表
func CleanOrphanPlaylists() int64 {
	// 1. 取出所有主站影片
	var films []struct {
		Name string
		DbId int64
	}
	db.Mdb.Model(&model.SearchInfo{}).Select("name", "db_id").Scan(&films)
	if len(films) == 0 {
		log.Println("[CleanOrphan] search_infos 为空，跳过孤儿清理")
		return 0
	}

	// 2. 生成有效 movie_key 集合
	validKeys := make(map[string]struct{}, len(films)*2)
	re := regexp.MustCompile(`第一季$`)
	for _, f := range films {
		// 基于名称的哈希
		validKeys[utils.GenerateHashKey(f.Name)] = struct{}{}
		if trimmed := re.ReplaceAllString(f.Name, ""); trimmed != f.Name {
			validKeys[utils.GenerateHashKey(trimmed)] = struct{}{}
		}
		// 基于豆瓣ID的哈希 (如果存在)
		if f.DbId != 0 {
			validKeys[utils.GenerateHashKey(f.DbId)] = struct{}{}
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
func GetHotMovieByPid(pid int64, page *dto.Page) []model.SearchInfo {
	var s []model.SearchInfo
	t := time.Now().AddDate(0, -1, 0).Unix()
	if err := db.Mdb.Limit(page.PageSize).Offset((page.Current-1)*page.PageSize).Where("pid = ? AND update_stamp > ?", pid, t).Order(" year DESC, hits DESC").Find(&s).Error; err != nil {
		log.Printf("GetHotMovieByPid Error: %v", err)
		return nil
	}
	return s
}

// GetHotMovieByCid 获取当前分类下的热门影片
func GetHotMovieByCid(cid int64, page *dto.Page) []model.SearchInfo {
	var s []model.SearchInfo
	t := time.Now().AddDate(0, -1, 0).Unix()
	if err := db.Mdb.Limit(page.PageSize).Offset((page.Current-1)*page.PageSize).Where("cid = ? AND update_stamp > ?", cid, t).Order(" year DESC, hits DESC").Find(&s).Error; err != nil {
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
func GetMovieListBySort(t int, pid int64, page *dto.Page) []model.MovieBasicInfo {
	var sl []model.SearchInfo
	qw := db.Mdb.Model(&model.SearchInfo{}).Where("pid = ? OR cid = ?", pid, pid).Limit(page.PageSize).Offset((page.Current - 1) * page.PageSize)
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
