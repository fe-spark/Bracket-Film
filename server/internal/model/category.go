package model

import "time"

// Category 分类信息 (统一层级模型)
type Category struct {
	Id        int64     `gorm:"primaryKey;autoIncrement:true" json:"id"`      // 分类ID
	Pid       int64     `gorm:"index;constraint:OnDelete:CASCADE" json:"pid"` // 父级分类ID (Pid=0 表示顶级大类)
	Name      string    `gorm:"size:64;uniqueIndex" json:"name"`            // 分类名称
	Alias     string    `gorm:"size:128" json:"alias"`                      // 别名/匹配规则 (仅大类有用)
	Show      bool      `gorm:"default:true" json:"show"`                   // 是否展示
	Sort      int       `gorm:"default:0" json:"sort"`                      // 排序权重
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Category) TableName() string {
	return TableCategory
}

// CategoryTree 分類信息樹形結構 (扁平化 JSON)
type CategoryTree struct {
	Id        int64           `json:"id"`
	Pid       int64           `json:"pid"`
	Name      string          `json:"name"`
	Alias     string          `json:"alias"`
	Show      bool            `json:"show"`
	Sort      int             `json:"sort"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Children  []*CategoryTree `json:"children"` // 子分類信息
}
