package system

import (
	"errors"
	"log"

	"server/plugin/common/util"
	"server/plugin/db"
)

/*
	影视采集站点信息
*/

type SourceGrade int

const (
	MasterCollect SourceGrade = iota
	SlaveCollect
)

type CollectResultModel int

const (
	JsonResult CollectResultModel = iota
	XmlResult
)

type ResourceType int

func (rt ResourceType) GetActionType() string {
	var ac string = ""
	switch rt {
	case CollectVideo:
		ac = "detail"
	case CollectArticle:
		ac = "article"
	case CollectActor:
		ac = "actor"
	case CollectRole:
		ac = "role"
	case CollectWebSite:
		ac = "web"
	default:
		ac = "detail"
	}
	return ac
}

const (
	CollectVideo = iota
	CollectArticle
	CollectActor
	CollectRole
	CollectWebSite
)

// FilmSource 影视站点信息保存结构体
type FilmSource struct {
	Id           string             `json:"id" gorm:"primaryKey;size:32"`    // 唯一ID
	Name         string             `json:"name" gorm:"size:64"`             // 采集站点备注名
	Uri          string             `json:"uri" gorm:"uniqueIndex;size:255"` // 采集链接
	ResultModel  CollectResultModel `json:"resultModel"`                     // 接口返回类型, json || xml
	Grade        SourceGrade        `json:"grade"`                           // 采集站等级 主站点 || 附属站
	SyncPictures bool               `json:"syncPictures"`                    // 是否同步图片到服务器
	CollectType  ResourceType       `json:"collectType"`                     // 采集资源类型
	State        bool               `json:"state"`                           // 是否启用
	Interval     int                `json:"interval"`                        // 采集时间间隔 单位/ms
}

func (f *FilmSource) TableName() string {
	return "film_sources"
}

// GetCollectSourceList 获取采集站API列表
func GetCollectSourceList() []FilmSource {
	var list []FilmSource
	if err := db.Mdb.Order("grade ASC").Find(&list).Error; err != nil {
		log.Println("GetCollectSourceList Error:", err)
		return nil
	}
	return list
}

// GetCollectSourceListByGrade 返回指定类型的采集Api信息 Master | Slave
func GetCollectSourceListByGrade(grade SourceGrade) []FilmSource {
	var list []FilmSource
	if err := db.Mdb.Where("grade = ?", grade).Find(&list).Error; err != nil {
		log.Println("GetCollectSourceListByGrade Error:", err)
		return nil
	}
	return list
}

// FindCollectSourceById 通过Id标识获取对应的资源站信息
func FindCollectSourceById(id string) *FilmSource {
	var fs FilmSource
	if err := db.Mdb.Where("id = ?", id).First(&fs).Error; err != nil {
		return nil
	}
	return &fs
}

// DelCollectResource 通过Id删除对应的采集站点信息
func DelCollectResource(id string) {
	db.Mdb.Where("id = ?", id).Delete(&FilmSource{})
}

// AddCollectSource 添加采集站信息
func AddCollectSource(s FilmSource) error {
	var count int64
	db.Mdb.Model(&FilmSource{}).Where("uri = ?", s.Uri).Count(&count)
	if count > 0 {
		return errors.New("当前采集站点信息已存在, 请勿重复添加")
	}
	// 生成一个短uuid
	if s.Id == "" {
		s.Id = util.GenerateSalt()
	}
	return db.Mdb.Create(&s).Error
}

// BatchAddCollectSource 批量添加采集站信息
func BatchAddCollectSource(list []FilmSource) error {
	return db.Mdb.Create(list).Error
}

// UpdateCollectSource 更新采集站信息
func UpdateCollectSource(s FilmSource) error {
	var count int64
	db.Mdb.Model(&FilmSource{}).Where("id != ? AND uri = ?", s.Id, s.Uri).Count(&count)
	if count > 0 {
		return errors.New("当前采集站链接已存在其他站点中, 请勿重复添加")
	}
	return db.Mdb.Save(&s).Error
}

// ClearAllCollectSource 删除所有采集站信息
func ClearAllCollectSource() {
	db.Mdb.Exec("TRUNCATE table film_sources")
}

// ExistCollectSourceList 查询是否已经存在站点list相关数据
func ExistCollectSourceList() bool {
	var count int64
	db.Mdb.Model(&FilmSource{}).Count(&count)
	return count > 0
}

// CreateFilmSourceTable 创建采集源信息表
func CreateFilmSourceTable() {
	if !db.Mdb.Migrator().HasTable(&FilmSource{}) {
		err := db.Mdb.AutoMigrate(&FilmSource{})
		if err != nil {
			log.Println("Create Table FilmSource Failed: ", err)
		}
	}
}
