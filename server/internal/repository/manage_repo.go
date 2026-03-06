package repository

import (
	"encoding/json"
	"log"
	"sort"

	"server/internal/config"
	"server/internal/model"
	"server/internal/infra/db"

	"gorm.io/gorm/clause"
)

// ExistSiteConfig 判断 MySQL 中是否已有网站配置
func ExistSiteConfig() bool {
	var count int64
	db.Mdb.Model(&model.SiteConfigRecord{}).Count(&count)
	return count > 0
}

// ExistBannersConfig 判断 MySQL 中是否已有轮播配置
func ExistBannersConfig() bool {
	var count int64
	db.Mdb.Model(&model.BannersRecord{}).Count(&count)
	return count > 0
}

// SaveSiteBasic 保存网站基本配置信息 (MySQL + Redis 短期缓存)
func SaveSiteBasic(c model.BasicConfig) error {
	rec := model.SiteConfigRecord{
		SiteName: c.SiteName, Domain: c.Domain, Logo: c.Logo,
		Keyword: c.Keyword, Describe: c.Describe, State: c.State, Hint: c.Hint,
	}
	db.Mdb.Clauses(clause.OnConflict{UpdateAll: true}).Where("id > 0").Delete(&model.SiteConfigRecord{})
	if err := db.Mdb.Create(&rec).Error; err != nil {
		return err
	}
	// write-through
	data, _ := json.Marshal(c)
	_ = db.Rdb.Set(db.Cxt, config.SiteConfigBasic, data, config.ConfigCacheTTL).Err()
	return nil
}

// GetSiteBasic 获取网站基本配置信息 (Redis 缓存优先，MySQL 兜底)
func GetSiteBasic() model.BasicConfig {
	c := model.BasicConfig{}
	// 1. Redis 缓存
	if data := db.Rdb.Get(db.Cxt, config.SiteConfigBasic).Val(); data != "" {
		_ = json.Unmarshal([]byte(data), &c)
		return c
	}
	// 2. MySQL 兜底
	var rec model.SiteConfigRecord
	if err := db.Mdb.Order("id DESC").First(&rec).Error; err != nil {
		log.Println("GetSiteBasic MySQL Error:", err)
		return c
	}
	c = model.BasicConfig{
		SiteName: rec.SiteName, Domain: rec.Domain, Logo: rec.Logo,
		Keyword: rec.Keyword, Describe: rec.Describe, State: rec.State, Hint: rec.Hint,
	}
	// 回填缓存
	data, _ := json.Marshal(c)
	_ = db.Rdb.Set(db.Cxt, config.SiteConfigBasic, data, config.ConfigCacheTTL).Err()
	return c
}

// GetBanners 获取轮播配置信息 (Redis 缓存优先，MySQL 兜底)
func GetBanners() model.Banners {
	bl := make(model.Banners, 0)
	// 1. Redis 缓存
	data := db.Rdb.Get(db.Cxt, config.BannersKey).Val()
	if data != "" && data != "null" {
		if err := json.Unmarshal([]byte(data), &bl); err == nil && len(bl) > 0 {
			sort.Sort(bl)
			return bl
		}
	}
	// 2. MySQL 兜底
	var rec model.BannersRecord
	if err := db.Mdb.Order("id DESC").First(&rec).Error; err != nil || rec.Content == "" {
		return bl
	}
	if err := json.Unmarshal([]byte(rec.Content), &bl); err != nil {
		log.Println("GetBanners Unmarshal Error:", err)
	}
	sort.Sort(bl)
	// 回填缓存
	_ = db.Rdb.Set(db.Cxt, config.BannersKey, rec.Content, config.ConfigCacheTTL).Err()
	return bl
}

// SaveBanners 保存轮播配置信息 (MySQL + Redis 短期缓存)
func SaveBanners(bl model.Banners) error {
	data, _ := json.Marshal(bl)
	// MySQL 单行 upsert
	db.Mdb.Where("id > 0").Delete(&model.BannersRecord{})
	if err := db.Mdb.Create(&model.BannersRecord{Content: string(data)}).Error; err != nil {
		return err
	}
	// write-through
	_ = db.Rdb.Set(db.Cxt, config.BannersKey, data, config.ConfigCacheTTL).Err()
	return nil
}
