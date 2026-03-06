package repository

import (
	"encoding/json"
	"log"
	"regexp"
	"strings"

	"server/internal/infra/db"
	"server/internal/model"
	"server/internal/utils"
)

// SaveCategoryTree 保存影片分类信息 (MySQL 持久化)
func SaveCategoryTree(tree *model.CategoryTree) error {
	data, _ := json.Marshal(tree)
	if err := db.Mdb.Save(&model.CategoryPersistent{Content: string(data)}).Error; err != nil {
		return err
	}
	return nil
}

// GetCategoryTree 获取影片分类信息（直查 MySQL）
func GetCategoryTree() model.CategoryTree {
	var cp model.CategoryPersistent
	tree := model.CategoryTree{}
	if err := db.Mdb.Order("id DESC").First(&cp).Error; err != nil {
		return tree
	}
	_ = json.Unmarshal([]byte(cp.Content), &tree)
	return tree
}

// ExistsCategoryTree 查询分类信息是否存在（直查 MySQL）
func ExistsCategoryTree() bool {
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

// ReMapCategoryByName 基于分类名称智能对比，重新关联影片的 Cid 和 Pid
func ReMapCategoryByName() error {
	tree := GetCategoryTree()
	if len(tree.Children) == 0 {
		return nil
	}

	// 1. 获取库中所有不重复的 c_name
	var cNames []string
	db.Mdb.Model(&model.SearchInfo{}).Distinct().Pluck("c_name", &cNames)

	// 2. 预解析分类树，建立标准名称映射 (简体化)
	type catInfo struct {
		pid int64
		cid int64
	}
	nameMap := make(map[string]catInfo)

	// 内置核心大类模糊规则 (正则表达式)
	coreRules := map[string]*regexp.Regexp{
		"电影":  regexp.MustCompile(`电影|影院|蓝光`),
		"连续剧": regexp.MustCompile(`电视剧|剧集|连续剧|.*剧`),
		"动漫":  regexp.MustCompile(`动漫|动画`),
		"综艺":  regexp.MustCompile(`综艺|真人秀`),
	}

	// 填充映射表
	for _, t := range tree.Children {
		nameMap[utils.TraditionalToSimplified(t.Category.Name)] = catInfo{t.Id, t.Id}
		for _, c := range t.Children {
			nameMap[utils.TraditionalToSimplified(c.Category.Name)] = catInfo{t.Id, c.Id}
		}
	}

	// 3. 匹配并执行更新
	for _, rawName := range cNames {
		normName := utils.TraditionalToSimplified(rawName)
		var target *catInfo

		// 3.1 尝试精确匹配 (简体)
		if info, ok := nameMap[normName]; ok {
			target = &info
		} else {
			// 3.2 尝试模糊包含匹配 (例如 "动作" 匹配 "动作片")
			for catName, info := range nameMap {
				if strings.Contains(normName, catName) || strings.Contains(catName, normName) {
					target = &info
					break
				}
			}

			// 3.3 核心大类兜底规则匹配
			if target == nil {
				for rootName, reg := range coreRules {
					if reg.MatchString(normName) {
						// 寻找树中对应的大类节点
						for _, t := range tree.Children {
							tNorm := utils.TraditionalToSimplified(t.Category.Name)
							if strings.Contains(tNorm, rootName) || strings.Contains(rootName, tNorm) {
								target = &catInfo{t.Id, t.Id}
								break
							}
						}
					}
					if target != nil {
						break
					}
				}
			}
		}

		// 3.4 执行数据库批量更新
		if target != nil {
			db.Mdb.Model(&model.SearchInfo{}).
				Where("c_name = ?", rawName).
				Updates(map[string]any{"pid": target.pid, "cid": target.cid})
		}
	}
	log.Printf("[Category] 完成全库分类动态对齐，处理了 %d 个原始分类项\n", len(cNames))
	return nil
}
