package system

import (
	"encoding/json"
	"server/config"
	"server/plugin/db"
	"time"

	"gorm.io/gorm"
)

// Category 分类信息
type Category struct {
	Id   int64  `json:"id"`   // 分类ID
	Pid  int64  `json:"pid"`  // 父级分类ID
	Name string `json:"name"` // 分类名称
	Show bool   `json:"show"` // 是否展示
}

// CategoryTree 分类信息树形结构
type CategoryTree struct {
	*Category
	Children []*CategoryTree `json:"children"` // 子分类信息
}

// CategoryPersistent 分类持久化模型 (MySQL)
type CategoryPersistent struct {
	gorm.Model
	Content string `gorm:"type:longtext"` // 存储序列化后的完整 CategoryTree JSON
}

// 影视分类展示树形结构

// SaveCategoryTree 保存影片分类信息 (MySQL + Redis 10分钟缓存)
func SaveCategoryTree(tree *CategoryTree) error {
	data, _ := json.Marshal(tree)
	// 1. 持久化到 MySQL
	db.Mdb.Save(&CategoryPersistent{Content: string(data)})
	// 2. 缓存到 Redis (短时间)
	return db.Rdb.Set(db.Cxt, config.CategoryTreeKey, data, time.Minute*10).Err()
}

// GetCategoryTree 获取影片分类信息
func GetCategoryTree() CategoryTree {
	// 1. 优先从 Redis 获取
	data := db.Rdb.Get(db.Cxt, config.CategoryTreeKey).Val()
	if data == "" {
		// 2. Redis 未命中，从 MySQL 获取最新一条
		var cp CategoryPersistent
		if err := db.Mdb.Order("id DESC").First(&cp).Error; err == nil {
			data = cp.Content
			// 回填缓存
			_ = db.Rdb.Set(db.Cxt, config.CategoryTreeKey, data, time.Minute*10).Err()
		}
	}
	tree := CategoryTree{}
	_ = json.Unmarshal([]byte(data), &tree)
	return tree
}

// ExistsCategoryTree 查询分类信息是否存在
func ExistsCategoryTree() bool {
	exists, _ := db.Rdb.Exists(db.Cxt, config.CategoryTreeKey).Result()
	if exists == 1 {
		return true
	}
	var count int64
	db.Mdb.Model(&CategoryPersistent{}).Count(&count)
	return count > 0
}

// CreateCategoryTable 创建分类持久化表
func CreateCategoryTable() {
	if !db.Mdb.Migrator().HasTable(&CategoryPersistent{}) {
		_ = db.Mdb.AutoMigrate(&CategoryPersistent{})
	}
}

// GetChildrenTree 根据影片Id获取对应分类的子分类信息
func GetChildrenTree(id int64) []*CategoryTree {
	tree := GetCategoryTree()
	for _, t := range tree.Children {
		if t.Id == id {
			return t.Children
		}
	}
	return nil

}
