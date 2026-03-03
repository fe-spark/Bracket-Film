package model

import "gorm.io/gorm"

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
