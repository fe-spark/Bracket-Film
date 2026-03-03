package repository

import (
	"encoding/json"

	"server-v2/config"
	"server-v2/internal/model"
	"server-v2/pkg/db"
)

// CreateCategoryTable 创建分类持久化表
func CreateCategoryTable() {
	if !db.Mdb.Migrator().HasTable(&model.CategoryPersistent{}) {
		_ = db.Mdb.AutoMigrate(&model.CategoryPersistent{})
	}
}

// SaveCategoryTree 保存影片分类信息 (MySQL 持久化 + Redis write-through)
func SaveCategoryTree(tree *model.CategoryTree) error {
	data, _ := json.Marshal(tree)
	// 1. 持久化到 MySQL
	if err := db.Mdb.Save(&model.CategoryPersistent{Content: string(data)}).Error; err != nil {
		return err
	}
	// 2. write-through：直接更新 Redis，下次读取无需再打 MySQL
	_ = db.Rdb.Set(db.Cxt, config.CategoryTreeKey, string(data), config.ConfigCacheTTL).Err()
	return nil
}

// GetCategoryTree 获取影片分类信息
func GetCategoryTree() model.CategoryTree {
	// 1. 优先从 Redis 缓存获取
	data := db.Rdb.Get(db.Cxt, config.CategoryTreeKey).Val()
	if data == "" {
		// 2. Redis 未命中，从 MySQL 获取最新一条
		var cp model.CategoryPersistent
		if err := db.Mdb.Order("id DESC").First(&cp).Error; err == nil {
			data = cp.Content
			// 回填缓存
			_ = db.Rdb.Set(db.Cxt, config.CategoryTreeKey, data, config.ConfigCacheTTL).Err()
		}
	}
	tree := model.CategoryTree{}
	if data != "" {
		_ = json.Unmarshal([]byte(data), &tree)
	}
	return tree
}

// ExistsCategoryTree 查询分类信息是否存在
func ExistsCategoryTree() bool {
	exists, _ := db.Rdb.Exists(db.Cxt, config.CategoryTreeKey).Result()
	if exists == 1 {
		return true
	}
	var count int64
	db.Mdb.Model(&model.CategoryPersistent{}).Count(&count)
	return count > 0
}

// GetChildrenTree 根据影片Id获取对应分类的子分类信息
func GetChildrenTree(id int64) []*model.CategoryTree {
	tree := GetCategoryTree()
	for _, t := range tree.Children {
		if t.Id == id {
			return t.Children
		}
	}
	return nil
}

// GetParentId 获取指定分类的父级 ID，如果本身是一级分类则返回其自身 ID
func GetParentId(id int64) int64 {
	tree := GetCategoryTree()
	for _, t := range tree.Children {
		if t.Id == id {
			return t.Id
		}
		for _, c := range t.Children {
			if c.Id == id {
				return t.Id
			}
		}
	}
	return id
}
