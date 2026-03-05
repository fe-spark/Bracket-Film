package model

import (
	"server-v2/internal/model/dto"

	"gorm.io/gorm"
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
	SubTitle    string `json:"subTitle"`    // 子标题
	CName       string `json:"cName"`       // 分类名称
	EnName      string `json:"enName"`      // 英文名
	Initial     string `json:"initial"`     // 首字母
	ClassTag    string `json:"classTag"`    // 分类标签
	Actor       string `json:"actor"`       // 主演
	Director    string `json:"director"`    // 导演
	Writer      string `json:"writer"`      // 作者
	Blurb       string `json:"blurb"`       // 简介, 残缺,不建议使用
	Remarks     string `json:"remarks"`     // 更新情况
	ReleaseDate string `json:"releaseDate"` // 上映时间
	Area        string `json:"area"`        // 地区
	Language    string `json:"language"`    // 语言
	Year        string `json:"year"`        // 年份
	State       string `json:"state"`       // 影片状态 正片|预告...
	UpdateTime  string `json:"updateTime"`  // 更新时间
	AddTime     int64  `json:"addTime"`     // 资源添加时间戳
	DbId        int64  `json:"dbId"`        // 豆瓣id
	DbScore     string `json:"dbScore"`     // 豆瓣评分
	Hits        int64  `json:"hits"`        // 影片热度
	Content     string `json:"content"`     // 内容简介
}

// MovieBasicInfo 影片基本信息
type MovieBasicInfo struct {
	Id       int64  `json:"id"`       // 影片Id
	Cid      int64  `json:"cid"`      // 分类ID
	Pid      int64  `json:"pid"`      // 一级分类ID
	Name     string `json:"name"`     // 片名
	SubTitle string `json:"subTitle"` // 子标题
	CName    string `json:"cName"`    // 分类名称
	State    string `json:"state"`    // 影片状态 正片|预告...
	Picture  string `json:"picture"`  // 简介图片
	Actor    string `json:"actor"`    // 主演
	Director string `json:"director"` // 导演
	Blurb    string `json:"blurb"`    // 简介, 不完整
	Remarks  string `json:"remarks"`  // 更新情况
	Area     string `json:"area"`     // 地区
	Year     string `json:"year"`     // 年份
}

// MovieUrlInfo 影视资源url信息
type MovieUrlInfo struct {
	Episode string `json:"episode"` // 集数
	Link    string `json:"link"`    // 播放地址
}

// MovieDetail 影片详情信息
type MovieDetail struct {
	Id              int64               `json:"id"`           // 影片Id
	Cid             int64               `json:"cid"`          // 分类ID
	Pid             int64               `json:"pid"`          // 一级分类ID
	Name            string              `json:"name"`         // 片名
	Picture         string              `json:"picture"`      // 简介图片
	PlayFrom        []string            `json:"playFrom"`     // 播放来源
	DownFrom        string              `json:"DownFrom"`     // 下载来源 例: http
	PlayList        [][]MovieUrlInfo    `json:"playList"`     // 播放地址url
	DownloadList    [][]MovieUrlInfo    `json:"downloadList"` // 下载url地址
	MovieDescriptor `json:"descriptor"` // 影片描述信息
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

// MoviePlaylist 多源播放列表持久化模型 (MySQL)
type MoviePlaylist struct {
	gorm.Model
	SourceId string `gorm:"index"`
	MovieKey string `gorm:"index"` // hash(name) or hash(dbid)
	Content  string `gorm:"type:longtext"`
}

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

// SearchTagItem 影片检索标签持久化模型 (MySQL)
type SearchTagItem struct {
	gorm.Model
	Pid     int64  `gorm:"uniqueIndex:uidx_search_tag;not null"`
	TagType string `gorm:"uniqueIndex:uidx_search_tag;size:32;not null"`  // Category/Plot/Area/Language/Year/Initial/Sort
	Name    string `gorm:"size:128;not null"`                             // 展示名称
	Value   string `gorm:"uniqueIndex:uidx_search_tag;size:128;not null"` // 筛选值
	Score   int64  `gorm:"default:0"`                                     // 热度权重，用于排序
}

// Tag 影片分类标签结构体
type Tag struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

// SearchTagsVO 搜索标签请求参数
type SearchTagsVO struct {
	Pid      int64  `json:"pid"`
	Cid      int64  `json:"cid"`
	Plot     string `json:"plot"`
	Area     string `json:"area"`
	Language string `json:"language"`
	Year     int64  `json:"year"`
	Sort     string `json:"sort"`
}

// SearchVo 影片信息搜索参数
type SearchVo struct {
	Name      string    `json:"name"`      // 影片名
	Pid       int64     `json:"pid"`       // 一级分类ID
	Cid       int64     `json:"cid"`       // 二级分类ID
	Plot      string    `json:"plot"`      // 剧情
	Area      string    `json:"area"`      // 地区
	Language  string    `json:"language"`  // 语言
	Year      int64     `json:"year"`      // 年份
	Remarks   string    `json:"remarks"`   // 完结 | 未完结
	BeginTime int64     `json:"beginTime"` // 更新时间戳起始值
	EndTime   int64     `json:"endTime"`   // 更新时间戳结束值
	Paging    *dto.Page `json:"paging"`    // 分页参数
}

// FilmDetailVo 添加影片对象
type FilmDetailVo struct {
	Id           int64    `json:"id"`           // 影片id
	Cid          int64    `json:"cid"`          // 分类ID
	Pid          int64    `json:"pid"`          // 一级分类ID
	Name         string   `json:"name"`         // 片名
	Picture      string   `json:"picture"`      // 简介图片
	PlayFrom     []string `json:"playFrom"`     // 播放来源
	DownFrom     string   `json:"DownFrom"`     // 下载来源 例: http
	PlayLink     string   `json:"playLink"`     // 播放地址url
	DownloadLink string   `json:"downloadLink"` // 下载url地址
	SubTitle     string   `json:"subTitle"`     // 子标题
	CName        string   `json:"cName"`        // 分类名称
	EnName       string   `json:"enName"`       // 英文名
	Initial      string   `json:"initial"`      // 首字母
	ClassTag     string   `json:"classTag"`     // 分类标签
	Actor        string   `json:"actor"`        // 主演
	Director     string   `json:"director"`     // 导演
	Writer       string   `json:"writer"`       // 作者
	Remarks      string   `json:"remarks"`      // 更新情况
	ReleaseDate  string   `json:"releaseDate"`  // 上映时间
	Area         string   `json:"area"`         // 地区
	Language     string   `json:"language"`     // 语言
	Year         string   `json:"year"`         // 年份
	State        string   `json:"state"`        // 影片状态 正片|预告...
	UpdateTime   string   `json:"updateTime"`   // 更新时间
	AddTime      string   `json:"addTime"`      // 资源添加时间戳
	DbId         int64    `json:"dbId"`         // 豆瓣id
	DbScore      string   `json:"dbScore"`      // 豆瓣评分
	Hits         int64    `json:"hits"`         // 影片热度
	Content      string   `json:"content"`      // 内容简介
}

// PlayLinkVo 多站点播放链接数据列表
type PlayLinkVo struct {
	Id       string         `json:"id"`
	Name     string         `json:"name"`
	LinkList []MovieUrlInfo `json:"linkList"`
}

// MovieDetailVo 影片详情数据, 播放源合并版
type MovieDetailVo struct {
	MovieDetail
	List []PlayLinkVo `json:"list"`
}
