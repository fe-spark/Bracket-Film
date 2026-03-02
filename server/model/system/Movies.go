package system

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"regexp"
	"server/config"
	"server/plugin/db"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Movie 影片基本信息
type Movie struct {
	Id       int64  `json:"id"`       // 影片ID
	Name     string `json:"name"`     // 影片名
	Cid      int64  `json:"cid"`      // 所属分类ID
	CName    string `json:"CName"`    // 所属分类名称
	EnName   string `json:"enName"`   // 英文片名
	Time     string `json:"time"`     // 更新时间
	Remarks  string `json:"remarks"`  // 备注 | 清晰度
	PlayFrom string `json:"playFrom"` // 播放来源
}

// MovieDescriptor 影片详情介绍信息
type MovieDescriptor struct {
	SubTitle    string `json:"subTitle"`    //子标题
	CName       string `json:"cName"`       //分类名称
	EnName      string `json:"enName"`      //英文名
	Initial     string `json:"initial"`     //首字母
	ClassTag    string `json:"classTag"`    //分类标签
	Actor       string `json:"actor"`       //主演
	Director    string `json:"director"`    //导演
	Writer      string `json:"writer"`      //作者
	Blurb       string `json:"blurb"`       //简介, 残缺,不建议使用
	Remarks     string `json:"remarks"`     // 更新情况
	ReleaseDate string `json:"releaseDate"` //上映时间
	Area        string `json:"area"`        // 地区
	Language    string `json:"language"`    //语言
	Year        string `json:"year"`        //年份
	State       string `json:"state"`       //影片状态 正片|预告...
	UpdateTime  string `json:"updateTime"`  //更新时间
	AddTime     int64  `json:"addTime"`     //资源添加时间戳
	DbId        int64  `json:"dbId"`        //豆瓣id
	DbScore     string `json:"dbScore"`     // 豆瓣评分
	Hits        int64  `json:"hits"`        //影片热度
	Content     string `json:"content"`     //内容简介
}

// MovieBasicInfo 影片基本信息
type MovieBasicInfo struct {
	Id       int64  `json:"id"`       //影片Id
	Cid      int64  `json:"cid"`      //分类ID
	Pid      int64  `json:"pid"`      //一级分类ID
	Name     string `json:"name"`     //片名
	SubTitle string `json:"subTitle"` //子标题
	CName    string `json:"cName"`    //分类名称
	State    string `json:"state"`    //影片状态 正片|预告...
	Picture  string `json:"picture"`  //简介图片
	Actor    string `json:"actor"`    //主演
	Director string `json:"director"` //导演
	Blurb    string `json:"blurb"`    //简介, 不完整
	Remarks  string `json:"remarks"`  // 更新情况
	Area     string `json:"area"`     // 地区
	Year     string `json:"year"`     //年份
}

// MovieUrlInfo 影视资源url信息
type MovieUrlInfo struct {
	Episode string `json:"episode"` // 集数
	Link    string `json:"link"`    // 播放地址
}

// MovieDetail 影片详情信息
type MovieDetail struct {
	Id       int64    `json:"id"`       //影片Id
	Cid      int64    `json:"cid"`      //分类ID
	Pid      int64    `json:"pid"`      //一级分类ID
	Name     string   `json:"name"`     //片名
	Picture  string   `json:"picture"`  //简介图片
	PlayFrom []string `json:"playFrom"` // 播放来源
	DownFrom string   `json:"DownFrom"` //下载来源 例: http
	//PlaySeparator   string              `json:"playSeparator"` // 播放信息分隔符
	PlayList        [][]MovieUrlInfo    `json:"playList"`     //播放地址url
	DownloadList    [][]MovieUrlInfo    `json:"downloadList"` // 下载url地址
	MovieDescriptor `json:"descriptor"` //影片描述信息
}

// MoviePlaySource 多站播放源信息
type MoviePlaySource struct {
	SiteName string         `json:"siteName"` // 站点名称
	PlayList []MovieUrlInfo `json:"playList"` // 播放列表
}

// MovieDetailInfo 影片详情持久化模型 (MySQL)
type MovieDetailInfo struct {
	gorm.Model
	Mid     int64  `gorm:"uniqueIndex"`
	Cid     int64  `gorm:"index"`
	Content string `gorm:"type:longtext"` // 存储序列化后的完整 MovieDetail JSON
}

// CreateMovieDetailTable 创建影视详情表
func CreateMovieDetailTable() {
	if !db.Mdb.Migrator().HasTable(&MovieDetailInfo{}) {
		err := db.Mdb.AutoMigrate(&MovieDetailInfo{})
		if err != nil {
			log.Println("Create Table MovieDetailInfo Failed: ", err)
		}
	}
}

// MoviePlaylist 多源播放列表持久化模型 (MySQL)
type MoviePlaylist struct {
	gorm.Model
	SourceId string `gorm:"index"`
	MovieKey string `gorm:"index"` // hash(name) or hash(dbid)
	Content  string `gorm:"type:text"`
}

// CreateMoviePlaylistTable 创建多源播放列表表
func CreateMoviePlaylistTable() {
	if !db.Mdb.Migrator().HasTable(&MoviePlaylist{}) {
		_ = db.Mdb.AutoMigrate(&MoviePlaylist{})
	}
}

// =================================== 数据持久化与缓存交互 ========================================================

// SaveDetails 保存影片详情信息 (优先 MySQL，可选 Redis 预热)
func SaveDetails(list []MovieDetail) (err error) {
	// 1. 批量保存检索信息到 MySQL
	BatchSaveSearchInfo(list)

	// 2. 批量保存详细信息到 MySQL movie_details 表
	var details []MovieDetailInfo
	for _, v := range list {
		data, _ := json.Marshal(v)
		details = append(details, MovieDetailInfo{Mid: v.Id, Cid: v.Cid, Content: string(data)})
	}

	if len(details) > 0 {
		err = db.Mdb.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "mid"}},
			DoUpdates: clause.AssignmentColumns([]string{"cid", "content", "updated_at"}),
		}).Create(&details).Error
	}
	return err
}

// SaveDetail 保存单部影片信息 (优先 MySQL，可选 Redis 预热)
func SaveDetail(detail MovieDetail) (err error) {
	searchInfo := ConvertSearchInfo(detail)
	// 1. 保存检索信息到 MySQL
	err = SaveSearchInfo(searchInfo)
	if err != nil {
		return err
	}

	// 2. 保存到 MySQL movie_details 持久化表
	data, _ := json.Marshal(detail)
	err = db.Mdb.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "mid"}},
		DoUpdates: clause.AssignmentColumns([]string{"cid", "content", "updated_at"}),
	}).Create(&MovieDetailInfo{Mid: detail.Id, Cid: detail.Cid, Content: string(data)}).Error

	return err
}

// SaveSitePlayList 仅保存播放url列表信息到当前站点 (MySQL 持久化)
func SaveSitePlayList(id string, list []MovieDetail) (err error) {
	if len(list) <= 0 {
		return nil
	}
	var playlists []MoviePlaylist
	for _, d := range list {
		if len(d.PlayList) > 0 {
			data, _ := json.Marshal(d.PlayList[0])
			if strings.Contains(d.CName, "解说") {
				continue
			}
			// 保存 DbId key
			if d.DbId != 0 {
				playlists = append(playlists, MoviePlaylist{
					SourceId: id,
					MovieKey: GenerateHashKey(d.DbId),
					Content:  string(data),
				})
			}
			// 保存 Name key
			playlists = append(playlists, MoviePlaylist{
				SourceId: id,
				MovieKey: GenerateHashKey(d.Name),
				Content:  string(data),
			})
		}
	}
	if len(playlists) > 0 {
		// 使用 Upsert 逻辑
		db.Mdb.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "source_id"}, {Name: "movie_key"}},
			DoUpdates: clause.AssignmentColumns([]string{"content", "updated_at"}),
		}).Create(&playlists)
	}
	return nil
}

// BatchSaveSearchInfo 批量保存Search信息
func BatchSaveSearchInfo(list []MovieDetail) {
	var infoList []SearchInfo
	for _, v := range list {
		infoList = append(infoList, ConvertSearchInfo(v))
	}
	// 直接批量更新或保存到 MySQL
	BatchSaveOrUpdate(infoList)
}

// GetMovieDetailByDBID 获取所有站点内 匹配当前影片 ID或名称 的播放列表信息 (从 MySQL 获取)
func GetMovieDetailByDBID(mid int64, name string) []MoviePlaySource {
	var mps []MoviePlaySource
	// 1. 获取所有已启用的附属站点
	sources := GetCollectSourceList()
	// 2. 遍历站点，从 MySQL 中查询匹配项
	for _, s := range sources {
		if s.Grade == SlaveCollect && s.State {
			var playlist MoviePlaylist
			// 优先匹配 DBID，其次匹配名称
			key := GenerateHashKey(mid)
			if mid == 0 {
				key = GenerateHashKey(name)
			}
			if err := db.Mdb.Where("source_id = ? AND movie_key = ?", s.Id, key).First(&playlist).Error; err == nil {
				var ps MoviePlaySource
				_ = json.Unmarshal([]byte(playlist.Content), &ps)
				ps.SiteName = s.Name
				mps = append(mps, ps)
			} else if mid != 0 {
				// 如果传了 mid 但没搜到，再尝试搜一次 name
				if err := db.Mdb.Where("source_id = ? AND movie_key = ?", s.Id, GenerateHashKey(name)).First(&playlist).Error; err == nil {
					var ps MoviePlaySource
					_ = json.Unmarshal([]byte(playlist.Content), &ps)
					ps.SiteName = s.Name
					mps = append(mps, ps)
				}
			}
		}
	}
	return mps
}

// ConvertSearchInfo 将detail信息处理成 searchInfo
func ConvertSearchInfo(detail MovieDetail) SearchInfo {
	score, _ := strconv.ParseFloat(detail.DbScore, 64)
	stamp, _ := time.ParseInLocation(time.DateTime, detail.UpdateTime, time.Local)
	// detail中的年份信息并不准确, 因此采用 ReleaseDate中的年份
	year, err := strconv.ParseInt(regexp.MustCompile(`[1-9][0-9]{3}`).FindString(detail.ReleaseDate), 10, 64)
	if err != nil {
		year = 0
	}
	return SearchInfo{
		Mid:         detail.Id,
		Cid:         detail.Cid,
		Pid:         detail.Pid,
		Name:        detail.Name,
		SubTitle:    detail.SubTitle,
		CName:       detail.CName,
		ClassTag:    detail.ClassTag,
		Area:        detail.Area,
		Language:    detail.Language,
		Year:        year,
		Initial:     detail.Initial,
		Score:       score,
		Hits:        detail.Hits,
		UpdateStamp: stamp.Unix(),
		State:       detail.State,
		Remarks:     detail.Remarks,
		// ReleaseDate 部分影片缺失该参数, 所以使用添加时间作为上映时间排序
		ReleaseStamp: detail.AddTime,
		Picture:      detail.Picture,
		Actor:        detail.Actor,
		Director:     detail.Director,
		Blurb:        detail.Blurb,
	}
}

// GetBasicInfoByKey 获取Id对应的影片基本信息
func GetBasicInfoByKey(key string) MovieBasicInfo {
	// 1. 优先从 Redis 缓存获取
	data := []byte(db.Rdb.Get(db.Cxt, key).Val())
	basic := MovieBasicInfo{}

	if len(data) > 0 {
		_ = json.Unmarshal(data, &basic)
	} else {
		// 2. Redis 未命中，从 MySQL 获取详情并转换
		var cid, mid int64
		_, _ = fmt.Sscanf(key, config.MovieBasicInfoKey, &cid, &mid)

		var info MovieDetailInfo
		if err := db.Mdb.Where("mid = ?", mid).First(&info).Error; err == nil {
			var detail MovieDetail
			_ = json.Unmarshal([]byte(info.Content), &detail)
			// 转换为 BasicInfo
			basic = MovieBasicInfo{
				Id: detail.Id, Cid: detail.Cid, Pid: detail.Pid, Name: detail.Name,
				SubTitle: detail.SubTitle, CName: detail.CName, State: detail.State,
				Picture: detail.Picture, Actor: detail.Actor, Director: detail.Director,
				Blurb: detail.Blurb, Remarks: detail.Remarks, Area: detail.Area, Year: detail.Year,
			}
			// 3. 回控 Redis 缓存
			bd, _ := json.Marshal(basic)
			_ = db.Rdb.Set(db.Cxt, key, bd, config.FilmExpired).Err()
		}
	}

	// 执行本地图片匹配
	ReplaceBasicDetailPic(&basic)
	return basic
}

// GetDetailByKey 获取影片对应的详情信息
func GetDetailByKey(key string) MovieDetail {
	// 1. 优先从 Redis 缓存获取
	data := []byte(db.Rdb.Get(db.Cxt, key).Val())
	detail := MovieDetail{}

	if len(data) > 0 {
		_ = json.Unmarshal(data, &detail)
	} else {
		// 2. Redis 未命中，从 MySQL 获取 (解析 key 中的 id)
		// key 格式: MovieDetail:Cid%d:Id%d
		var cid, mid int64
		_, _ = fmt.Sscanf(key, config.MovieDetailKey, &cid, &mid)

		var info MovieDetailInfo
		if err := db.Mdb.Where("mid = ?", mid).First(&info).Error; err == nil {
			_ = json.Unmarshal([]byte(info.Content), &detail)
			// 3. 回填 Redis 缓存
			_ = db.Rdb.Set(db.Cxt, key, info.Content, config.FilmExpired).Err()
		}
	}

	// 执行本地图片匹配
	ReplaceDetailPic(&detail)
	return detail
}

// GetBasicInfoBySearchInfos 通过searchInfo 获取影片的基本信息
func GetBasicInfoBySearchInfos(infos ...SearchInfo) []MovieBasicInfo {
	var list []MovieBasicInfo
	for _, s := range infos {
		list = append(list, GetBasicInfoByKey(fmt.Sprintf(config.MovieBasicInfoKey, s.Cid, s.Mid)))
	}
	return list
}

/*
	对附属播放源入库时的name|dbID进行处理,保证唯一性
1. 去除name中的所有空格
2. 去除name中含有的别名～.*～
3. 去除name首尾的标点符号
4. 将处理完成后的name转化为hash值作为存储时的key
*/
// GenerateHashKey 存储播放源信息时对影片名称进行处理, 提高各站点间同一影片的匹配度
func GenerateHashKey[K string | ~int | int64](key K) string {
	mName := fmt.Sprint(key)
	//1. 去除name中的所有空格
	mName = regexp.MustCompile(`\s`).ReplaceAllString(mName, "")
	//2. 去除name中含有的别名～.*～
	mName = regexp.MustCompile(`～.*～$`).ReplaceAllString(mName, "")
	//3. 去除name首尾的标点符号
	mName = regexp.MustCompile(`^[[:punct:]]+|[[:punct:]]+$`).ReplaceAllString(mName, "")
	// 部分站点包含 动画版, 特殊别名 等字符, 需进行删除
	//mName = regexp.MustCompile(`动画版`).ReplaceAllString(mName, "")
	mName = regexp.MustCompile(`季.*`).ReplaceAllString(mName, "季")
	//4. 将处理完成后的name转化为hash值作为存储时的key
	h := fnv.New32a()
	_, err := h.Write([]byte(mName))
	if err != nil {
		return ""
	}
	return fmt.Sprint(h.Sum32())
}
