package service

import (
	"errors"
	"fmt"
	"time"

	"server-v2/internal/model"
	"server-v2/internal/repository"
	"server-v2/internal/spider/conver"
)

type FilmService struct{}

var FilmSvc = new(FilmService)

// GetFilmPage 获取影片检索信息分页数据
func (s *FilmService) GetFilmPage(vo model.SearchVo) []model.SearchInfo {
	return repository.GetSearchPage(vo)
}

// GetSearchOptions 获取影片检索的select的选项options
func (s *FilmService) GetSearchOptions() map[string]any {
	var options = make(map[string]any)
	tree := repository.GetCategoryTree()
	tree.Name = "全部分类"
	options["class"] = conver.ConvertCategoryList(tree)
	options["remarks"] = []map[string]string{{"Name": `全部`, "Value": ``}, {"Name": `完结`, "Value": `完结`}, {"Name": `未完结`, "Value": `未完结`}}
	options["year"] = make([]map[string]string, 0)
	var tagGroup = make(map[int64]map[string]any)
	if tree.Children != nil {
		for _, t := range tree.Children {
			option := repository.GetSearchOptions(t.Id)
			if len(option) > 0 {
				tagGroup[t.Id] = repository.GetSearchOptions(t.Id)
				if v, ok := options["year"].([]map[string]string); !ok || len(v) == 0 {
					options["year"] = tagGroup[t.Id]["Year"]
				}
			}

		}
	}
	options["tags"] = tagGroup
	return options
}

// SaveFilmDetail 自定义上传保存影片信息
func (s *FilmService) SaveFilmDetail(fd model.FilmDetailVo) error {
	now := time.Now()
	fd.UpdateTime = now.Format(time.DateTime)
	fd.AddTime = fd.UpdateTime
	if fd.Id == 0 {
		fd.Id = now.Unix()
	}
	detail, err := conver.CovertFilmDetailVo(fd)
	if err != nil || detail.PlayList == nil {
		return errors.New("影片参数格式异常或缺少关键信息")
	}

	return repository.SaveDetail(detail)
}

// DelFilm 删除分类影片
func (s *FilmService) DelFilm(id int64) error {
	sInfo := repository.GetSearchInfoById(id)
	if sInfo == nil || sInfo.ID == 0 {
		return errors.New("影片信息不存在")
	}
	return repository.DelFilmSearch(id)
}

// GetFilmClassTree 获取影片分类信息
func (s *FilmService) GetFilmClassTree() model.CategoryTree {
	return repository.GetCategoryTree()
}

// GetFilmClassById 通过ID获取影片分类信息
func (s *FilmService) GetFilmClassById(id int64) *model.CategoryTree {
	tree := repository.GetCategoryTree()
	for _, c := range tree.Children {
		if c.Id == id {
			return c
		}
		if c.Children != nil {
			for _, subC := range c.Children {
				if subC.Id == id {
					return subC
				}
			}
		}
	}
	return nil
}

// UpdateClass 更新分类信息
func (s *FilmService) UpdateClass(class model.CategoryTree) error {
	tree := repository.GetCategoryTree()
	for _, c := range tree.Children {
		if c.Id == class.Id {
			if class.Name != "" {
				c.Name = class.Name
			}
			c.Show = class.Show
			if err := repository.SaveCategoryTree(&tree); err != nil {
				return fmt.Errorf("影片分类信息更新失败: %s", err.Error())
			}
			return nil
		}
		if c.Children != nil {
			for _, subC := range c.Children {
				if subC.Id == class.Id {
					if class.Name != "" {
						subC.Name = class.Name
					}
					if class.Show {
						if err := repository.RecoverFilmSearch(class.Id); err != nil {
							return err
						}
					} else {
						if err := repository.ShieldFilmSearch(class.Id); err != nil {
							return err
						}
					}
					subC.Show = class.Show
					if err := repository.SaveCategoryTree(&tree); err != nil {
						return fmt.Errorf("影片分类信息更新失败: %s", err.Error())
					}
					return nil
				}
			}
		}
	}
	return errors.New("需要更新的分类信息不存在")
}

// DelClass 删除分类信息
func (s *FilmService) DelClass(id int64) error {
	tree := repository.GetCategoryTree()
	for i, c := range tree.Children {
		if c.Id == id {
			tree.Children = append(tree.Children[:i], tree.Children[i+1:]...)
			if err := repository.SaveCategoryTree(&tree); err != nil {
				return fmt.Errorf("影片分类信息删除失败: %s", err.Error())
			}
			return nil
		}
		if c.Children != nil {
			for j, subC := range c.Children {
				if subC.Id == id {
					c.Children = append(c.Children[:j], c.Children[j+1:]...)
					if err := repository.SaveCategoryTree(&tree); err != nil {
						return fmt.Errorf("影片分类信息删除失败: %s", err.Error())
					}
					return nil
				}
			}
		}
	}
	return errors.New("需要删除的分类信息不存在")
}
