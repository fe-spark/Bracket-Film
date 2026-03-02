package system

import (
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"server/config"
	"server/plugin/common/util"
	"server/plugin/db"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FileInfo 图片信息对象
type FileInfo struct {
	gorm.Model
	Link        string `json:"link"`        // 图片链接
	Uid         int    `json:"uid"`         // 上传人ID
	RelevanceId int64  `json:"relevanceId"` // 关联资源ID
	Type        int    `json:"type"`        // 文件类型 (0 影片封面, 1 用户头像)
	Fid         string `json:"fid"`         // 图片唯一标识, 通常为文件名
	FileType    string `json:"fileType"`    // 文件类型, txt, png, jpg
	//Size        int    `json:"size"`        // 文件大小
}

// VirtualPicture 采集入站,待同步的图片信息
type VirtualPicture struct {
	Id   int64  `json:"id"`
	Link string `json:"link"`
}

// VirtualPictureQueue 待同步图片队列 (MySQL)
type VirtualPictureQueue struct {
	gorm.Model
	Mid  int64  `gorm:"uniqueIndex"`
	Link string `gorm:"type:text"`
}

// CreateVirtualPictureTable 创建待同步图片表
func CreateVirtualPictureTable() {
	if !db.Mdb.Migrator().HasTable(&VirtualPictureQueue{}) {
		_ = db.Mdb.AutoMigrate(&VirtualPictureQueue{})
	}
}

//------------------------------------------------本地图库------------------------------------------------

// TableName 设置图片存储表的表名
func (f *FileInfo) TableName() string {
	return config.FileTableName
}

// StoragePath 获取文件的保存路径
func (f *FileInfo) StoragePath() string {
	var storage string
	switch f.FileType {
	case "jpeg", "jpg", "png", "webp":
		storage = strings.Replace(f.Link, config.FilmPictureAccess, fmt.Sprint(config.FilmPictureUploadDir, "/"), 1)
	default:
	}
	return storage
}

// CreateFileTable 创建图片关联信息存储表
func CreateFileTable() {
	// 如果不存在则创建表 并设置自增ID初始值为10000
	if !ExistFileTable() {
		err := db.Mdb.AutoMigrate(&FileInfo{})
		if err != nil {
			log.Println("Create Table FileInfo Failed: ", err)
		}
	}
}

// ExistFileTable 是否存在Picture表
func ExistFileTable() bool {
	// 1. 判断表中是否存在当前表
	return db.Mdb.Migrator().HasTable(&FileInfo{})
}

// SaveGallery 保存图片关联信息
func SaveGallery(f FileInfo) {
	db.Mdb.Create(&f)
}

// ExistFileInfoByRid 查找图片信息是否存在
func ExistFileInfoByRid(rid int64) bool {
	var count int64
	db.Mdb.Model(&FileInfo{}).Where("relevance_id = ?", rid).Count(&count)
	return count > 0
}

// GetFileInfoByRid 通过关联的资源id获取对应的图片信息
func GetFileInfoByRid(rid int64) FileInfo {
	var f FileInfo
	db.Mdb.Where("relevance_id = ?", rid).First(&f)
	return f
}

// GetFileInfoById 通过ID获取对应的图片信息
func GetFileInfoById(id uint) FileInfo {
	var f = FileInfo{}
	db.Mdb.First(&f, id)
	return f
}

// GetFileInfoPage 获取文件关联信息分页数据
func GetFileInfoPage(tl []string, page *Page) []FileInfo {
	var fl []FileInfo
	query := db.Mdb.Model(&FileInfo{}).Where("file_type IN ?", tl).Order("id DESC")
	// 获取分页相关参数
	GetPage(query, page)
	// 获取分页数据
	if err := query.Limit(page.PageSize).Offset((page.Current - 1) * page.PageSize).Find(&fl).Error; err != nil {
		log.Println(err)
		return nil
	}
	return fl
}

func DelFileInfo(id uint) {
	db.Mdb.Unscoped().Delete(&FileInfo{}, id)
}

//------------------------------------------------图片同步------------------------------------------------

// SaveVirtualPic 保存待同步的图片信息 (MySQL 持久化)
func SaveVirtualPic(pl []VirtualPicture) error {
	var queue []VirtualPictureQueue
	for _, p := range pl {
		queue = append(queue, VirtualPictureQueue{
			Mid:  p.Id,
			Link: p.Link,
		})
	}
	if len(queue) > 0 {
		return db.Mdb.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "mid"}},
			DoUpdates: clause.AssignmentColumns([]string{"link", "updated_at"}),
		}).Create(&queue).Error
	}
	return nil
}

// SyncFilmPicture 同步新采集入栈还未同步的图片 (从 MySQL 获取)
func SyncFilmPicture() {
	var queue []VirtualPictureQueue
	// 每次扫描 MaxScanCount 条
	if err := db.Mdb.Limit(config.MaxScanCount).Find(&queue).Error; err != nil || len(queue) == 0 {
		return
	}

	for _, item := range queue {
		// 判断当前影片是否已经同步过图片
		if ExistFileInfoByRid(item.Mid) {
			db.Mdb.Unscoped().Delete(&item)
			continue
		}
		// 将图片同步到服务器中
		fileName, err := util.SaveOnlineFile(item.Link, config.FilmPictureUploadDir)
		if err != nil {
			// 如果下载失败，逻辑上可以保留重试或者删除看业务需求，这里先删除防止死循环
			db.Mdb.Unscoped().Delete(&item)
			continue
		}
		// 完成同步后将图片信息保存到 Gallery 中
		SaveGallery(FileInfo{
			Link:        fmt.Sprint(config.FilmPictureAccess, fileName),
			Uid:         config.UserIdInitialVal,
			RelevanceId: item.Mid,
			Type:        0,
			Fid:         regexp.MustCompile(`\.[^.]+$`).ReplaceAllString(fileName, ""),
			FileType:    strings.TrimPrefix(filepath.Ext(fileName), "."),
		})
		// 同步成功后从队列删除
		db.Mdb.Unscoped().Delete(&item)
	}
	// 递归执行直到图片暂存信息为空
	SyncFilmPicture()
}

// ReplaceDetailPic 将影片详情中的图片地址替换为自己的
func ReplaceDetailPic(d *MovieDetail) {
	// 查询影片对应的本地图片信息
	if ExistFileInfoByRid(d.Id) {
		// 如果存在关联的本地图片, 则查询对应的图片信息
		f := GetFileInfoByRid(d.Id)
		// 替换采集站的图片链接为本地链接
		d.Picture = f.Link
	}
}

// ReplaceBasicDetailPic 替换影片基本数据中的封面图为本地图片
func ReplaceBasicDetailPic(d *MovieBasicInfo) {
	// 查询影片对应的本地图片信息
	if ExistFileInfoByRid(d.Id) {
		// 如果存在关联的本地图片, 则查询对应的图片信息
		f := GetFileInfoByRid(d.Id)
		// 替换采集站的图片链接为本地链接
		d.Picture = f.Link
	}
}
