package model

// Category 分类信息
type Category struct {
	Id   int64  `gorm:"primaryKey;autoIncrement:false" json:"id"` // 分类ID
	Pid  int64  `gorm:"index" json:"pid"`                         // 父级分类ID
	Name string `gorm:"size:64" json:"name"`                      // 分类名称
	Show bool   `json:"show"`                                     // 是否展示
}

// CategoryTree 分类信息树形结构
type CategoryTree struct {
	*Category
	Children []*CategoryTree `json:"children"` // 子分类信息
}
