package handler

import (
	"fmt"
	"strconv"
	"time"

	"server-v2/internal/model"
	"server-v2/internal/service"
	"server-v2/pkg/response"

	"github.com/gin-gonic/gin"
)

type FilmHandler struct{}

var FilmHd = new(FilmHandler)

// FilmSearchPage 获取影视分页数据
func (h *FilmHandler) FilmSearchPage(c *gin.Context) {
	var s = model.SearchVo{Paging: &response.Page{}}
	var err error

	s.Name = c.DefaultQuery("name", "")
	s.Pid, err = strconv.ParseInt(c.DefaultQuery("pid", "0"), 10, 64)
	if err != nil {
		response.Failed("影片分页数据获取失败, 请求参数异常", c)
		return
	}
	s.Cid, err = strconv.ParseInt(c.DefaultQuery("cid", "0"), 10, 64)
	if err != nil {
		response.Failed("影片分页数据获取失败, 请求参数异常", c)
		return
	}
	s.Plot = c.DefaultQuery("plot", "")
	s.Area = c.DefaultQuery("area", "")
	s.Language = c.DefaultQuery("language", "")
	year := c.DefaultQuery("year", "")
	if year == "" {
		s.Year = 0
	} else {
		s.Year, err = strconv.ParseInt(year, 10, 64)
		if err != nil {
			response.Failed("影片分页数据获取失败, 请求参数异常", c)
			return
		}
	}

	s.Remarks = c.DefaultQuery("remarks", "")
	begin := c.DefaultQuery("beginTime", "")
	if begin == "" {
		s.BeginTime = 0
	} else {
		beginTime, e := time.ParseInLocation(time.DateTime, begin, time.Local)
		if e != nil {
			response.Failed("影片分页数据获取失败, 请求参数异常", c)
			return
		}
		s.BeginTime = beginTime.Unix()
	}
	end := c.DefaultQuery("endTime", "")
	if end == "" {
		s.EndTime = 0
	} else {
		endTime, e := time.ParseInLocation(time.DateTime, end, time.Local)
		if e != nil {
			response.Failed("影片分页数据获取失败, 请求参数异常", c)
			return
		}
		s.EndTime = endTime.Unix()
	}

	s.Paging.Current, _ = strconv.Atoi(c.DefaultQuery("current", "1"))
	s.Paging.PageSize, err = strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if s.Paging.PageSize <= 0 || s.Paging.PageSize > 500 {
		s.Paging.PageSize = 10
	}
	if err != nil {
		response.Failed("影片分页数据获取失败, 请求参数异常", c)
		return
	}
	options := service.FilmSvc.GetSearchOptions()
	sl := service.FilmSvc.GetFilmPage(s)
	response.Success(gin.H{
		"params":  s,
		"list":    sl,
		"options": options,
	}, "影片分页信息获取成功", c)
}

// FilmAdd 手动添加影片
func (h *FilmHandler) FilmAdd(c *gin.Context) {
	var fd = model.FilmDetailVo{}
	if err := c.ShouldBindJSON(&fd); err != nil {
		response.Failed("影片添加失败, 影片参数提交异常", c)
		return
	}

	if err := service.FilmSvc.SaveFilmDetail(fd); err != nil {
		response.Failed(fmt.Sprint("影片添加失败, 影片信息保存错误: ", err.Error()), c)
		return
	}
	response.SuccessOnlyMsg("影片信息添加成功", c)
}

// FilmDelete 删除影片检索信息
func (h *FilmHandler) FilmDelete(c *gin.Context) {
	idStr := c.DefaultQuery("id", "")
	if idStr == "" {
		response.Failed("删除失败,缺少ID参数", c)
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.Failed("删除失败,参数类型异常", c)
		return
	}
	if err = service.FilmSvc.DelFilm(id); err != nil {
		response.Failed(fmt.Sprintln("删除失败: ", err.Error()), c)
		return
	}
	response.SuccessOnlyMsg("影片删除成功", c)
}

// ----------------------------------------------------影片分类处理----------------------------------------------------

// FilmClassTree 影片分类树数据
func (h *FilmHandler) FilmClassTree(c *gin.Context) {
	tree := service.FilmSvc.GetFilmClassTree()
	response.Success(tree, "影片分类信息获取成功", c)
}

// FindFilmClass 获取指定ID对应的影片分类信息
func (h *FilmHandler) FindFilmClass(c *gin.Context) {
	idStr := c.DefaultQuery("id", "")
	if idStr == "" {
		response.Failed("影片分类信息获取失败, 分类Id不能为空", c)
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.Failed("影片分类信息获取失败, 参数分类Id格式异常", c)
		return
	}
	class := service.FilmSvc.GetFilmClassById(id)
	if class == nil {
		response.Failed("影片分类信息获取失败, 分类信息不存在", c)
		return
	}
	response.Success(class, "分类信息查找成功", c)
}

// UpdateFilmClass 更新指定分类的影片数据
func (h *FilmHandler) UpdateFilmClass(c *gin.Context) {
	var class = model.CategoryTree{}
	if err := c.ShouldBindJSON(&class); err != nil {
		response.Failed("更新失败, 请求参数异常", c)
		return
	}
	if class.Id == 0 {
		response.Failed("更新失败, 分类Id缺失", c)
		return
	}
	if err := service.FilmSvc.UpdateClass(class); err != nil {
		response.Failed(err.Error(), c)
		return
	}
	service.IndexSvc.ClearIndexCache()
	response.SuccessOnlyMsg("影片分类信息更新成功", c)
}

// DelFilmClass 删除指定ID对应的影片分类
func (h *FilmHandler) DelFilmClass(c *gin.Context) {
	idStr := c.DefaultQuery("id", "")
	if idStr == "" {
		response.Failed("影片分类信息获取失败, 分类Id不能为空", c)
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.Failed("影片分类信息获取失败, 参数分类Id格式异常", c)
		return
	}
	if err = service.FilmSvc.DelClass(id); err != nil {
		response.Failed(err.Error(), c)
		return
	}
	response.SuccessOnlyMsg("当前分类已删除成功", c)
}
