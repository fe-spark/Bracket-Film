package system

import (
	"encoding/json"
	"log"
	"sort"
	"time"

	"server/config"
	"server/plugin/db"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BasicConfig 网站基本信息
type BasicConfig struct {
	SiteName string `json:"siteName"` // 网站名称
	Domain   string `json:"domain"`   // 网站域名
	Logo     string `json:"logo"`     // 网站logo
	Keyword  string `json:"keyword"`  // seo关键字
	Describe string `json:"describe"` // 网站描述信息
	State    bool   `json:"state"`    // 网站状态 开启 || 关闭
	Hint     string `json:"hint"`     // 网站关闭提示
}

// Banner 首页横幅信息
type Banner struct {
	Id      string `json:"id"`      // 唯一标识
	Mid     int64  `json:"mid"`     // 绑定所属影片Id
	Name    string `json:"name"`    // 影片名称
	Year    int64  `json:"year"`    // 上映年份
	CName   string `json:"cName"`   // 分类名称
	Poster  string `json:"poster"`  // 海报图片链接
	Picture string `json:"picture"` // 横幅大图链接
	Remark  string `json:"remark"`  // 更新状态描述信息
	Sort    int64  `json:"sort"`    // 排序分値
}

type Banners []Banner

func (bl Banners) Len() int           { return len(bl) }
func (bl Banners) Less(i, j int) bool { return bl[i].Sort < bl[j].Sort }
func (bl Banners) Swap(i, j int)      { bl[i], bl[j] = bl[j], bl[i] }

// ------------------------------------------------------ MySQL 持久化模型 ---

// SiteConfigRecord 网站基础配置持久化 (MySQL单行表)
type SiteConfigRecord struct {
	gorm.Model
	SiteName string `gorm:"size:128"`
	Domain   string `gorm:"size:256"`
	Logo     string `gorm:"size:512"`
	Keyword  string `gorm:"size:256"`
	Describe string `gorm:"size:512"`
	State    bool
	Hint     string `gorm:"size:512"`
}

// BannersRecord 轮播配置持久化 (MySQL, JSON blob)
type BannersRecord struct {
	gorm.Model
	Content string `gorm:"type:text"`
}

// CreateSiteConfigTable 创建网站配置表
func CreateSiteConfigTable() {
	if !db.Mdb.Migrator().HasTable(&SiteConfigRecord{}) {
		_ = db.Mdb.AutoMigrate(&SiteConfigRecord{})
	}
}

// CreateBannersTable 创建轮播配置表
func CreateBannersTable() {
	if !db.Mdb.Migrator().HasTable(&BannersRecord{}) {
		_ = db.Mdb.AutoMigrate(&BannersRecord{})
	}
}

// ExistSiteConfig 判断 MySQL 中是否已有网站配置
func ExistSiteConfig() bool {
	var count int64
	db.Mdb.Model(&SiteConfigRecord{}).Count(&count)
	return count > 0
}

// ExistBannersConfig 判断 MySQL 中是否已有轮播配置
func ExistBannersConfig() bool {
	var count int64
	db.Mdb.Model(&BannersRecord{}).Count(&count)
	return count > 0
}

// ------------------------------------------------------ 业务函数 ---

// SaveSiteBasic 保存网站基本配置信息 (MySQL + Redis 短期缓存)
func SaveSiteBasic(c BasicConfig) error {
	rec := SiteConfigRecord{
		SiteName: c.SiteName, Domain: c.Domain, Logo: c.Logo,
		Keyword: c.Keyword, Describe: c.Describe, State: c.State, Hint: c.Hint,
	}
	// MySQL 单行 upsert：删除旧记录再写入，保证永远只有一行
	db.Mdb.Clauses(clause.OnConflict{UpdateAll: true}).Where("id > 0").Delete(&SiteConfigRecord{})
	if err := db.Mdb.Create(&rec).Error; err != nil {
		return err
	}
	// 回写 Redis 缓存
	data, _ := json.Marshal(c)
	_ = db.Rdb.Set(db.Cxt, config.SiteConfigBasic, data, 30*time.Minute).Err()
	return nil
}

// GetSiteBasic 获取网站基本配置信息 (Redis 缓存优先，MySQL 兆底)
func GetSiteBasic() BasicConfig {
	c := BasicConfig{}
	// 1. Redis 缓存
	if data := db.Rdb.Get(db.Cxt, config.SiteConfigBasic).Val(); data != "" {
		_ = json.Unmarshal([]byte(data), &c)
		return c
	}
	// 2. MySQL 兆底
	var rec SiteConfigRecord
	if err := db.Mdb.Order("id DESC").First(&rec).Error; err != nil {
		log.Println("GetSiteBasic MySQL Error:", err)
		return c
	}
	c = BasicConfig{
		SiteName: rec.SiteName, Domain: rec.Domain, Logo: rec.Logo,
		Keyword: rec.Keyword, Describe: rec.Describe, State: rec.State, Hint: rec.Hint,
	}
	// 回写缓存
	data, _ := json.Marshal(c)
	_ = db.Rdb.Set(db.Cxt, config.SiteConfigBasic, data, 30*time.Minute).Err()
	return c
}

// GetBanners 获取轮播配置信息 (Redis 缓存优先，MySQL 兆底)
func GetBanners() Banners {
	bl := make(Banners, 0)
	// 1. Redis 缓存
	if data := db.Rdb.Get(db.Cxt, config.BannersKey).Val(); data != "" {
		_ = json.Unmarshal([]byte(data), &bl)
		sort.Sort(bl)
		return bl
	}
	// 2. MySQL 兆底
	var rec BannersRecord
	if err := db.Mdb.Order("id DESC").First(&rec).Error; err != nil || rec.Content == "" {
		return bl
	}
	if err := json.Unmarshal([]byte(rec.Content), &bl); err != nil {
		log.Println("GetBanners Unmarshal Error:", err)
	}
	sort.Sort(bl)
	// 回写缓存
	_ = db.Rdb.Set(db.Cxt, config.BannersKey, rec.Content, 30*time.Minute).Err()
	return bl
}

// SaveBanners 保存轮播配置信息 (MySQL + Redis 短期缓存)
func SaveBanners(bl Banners) error {
	data, _ := json.Marshal(bl)
	// MySQL 单行 upsert
	db.Mdb.Where("id > 0").Delete(&BannersRecord{})
	if err := db.Mdb.Create(&BannersRecord{Content: string(data)}).Error; err != nil {
		return err
	}
	// 回写 Redis 缓存
	_ = db.Rdb.Set(db.Cxt, config.BannersKey, data, 30*time.Minute).Err()
	return nil
}
