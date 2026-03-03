package handler

import (
	"fmt"
	"strconv"
	"time"

	"server-v2/internal/model"
	"server-v2/internal/service"
	"server-v2/internal/spider"
	"server-v2/pkg/response"

	"github.com/gin-gonic/gin"
)

type CollectHandler struct{}

var CollectHd = new(CollectHandler)

func (h *CollectHandler) FilmSourceList(c *gin.Context) {
	response.Success(service.CollectSvc.GetFilmSourceList(), "影视源站点信息获取成功", c)
}

func (h *CollectHandler) FindFilmSource(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		response.Failed("参数异常, 资源站标识不能为空", c)
		return
	}
	fs := service.CollectSvc.GetFilmSource(id)
	if fs == nil {
		response.Failed("数据异常,资源站信息不存在", c)
		return
	}
	response.Success(fs, "原站点详情信息查找成功", c)
}

func (h *CollectHandler) FilmSourceAdd(c *gin.Context) {
	s := model.FilmSource{}
	if err := c.ShouldBindJSON(&s); err != nil {
		response.Failed("请求参数异常", c)
		return
	}
	if err := validFilmSource(s); err != nil {
		response.Failed(err.Error(), c)
		return
	}
	if s.SyncPictures && (s.Grade == model.SlaveCollect) {
		response.Failed("附属站点无法开启图片同步功能", c)
		return
	}
	if err := spider.CollectApiTest(s); err != nil {
		response.Failed(fmt.Sprint("资源接口测试失败: ", err.Error()), c)
		return
	}
	if err := service.CollectSvc.SaveFilmSource(s); err != nil {
		response.Failed(fmt.Sprint("资源站添加失败: ", err.Error()), c)
		return
	}
	response.SuccessOnlyMsg("添加成功", c)
}

func (h *CollectHandler) FilmSourceUpdate(c *gin.Context) {
	s := model.FilmSource{}
	if err := c.ShouldBindJSON(&s); err != nil {
		response.Failed("请求参数异常", c)
		return
	}
	if err := validFilmSource(s); err != nil {
		response.Failed(err.Error(), c)
		return
	}
	if s.SyncPictures && (s.Grade == model.SlaveCollect) {
		response.Failed("附属站点无法开启图片同步功能", c)
		return
	}
	if s.Id == "" {
		response.Failed("参数异常, 资源站标识不能为空", c)
		return
	}
	fs := service.CollectSvc.GetFilmSource(s.Id)
	if fs == nil {
		response.Failed("数据异常,资源站信息不存在", c)
		return
	}
	if fs.Uri != s.Uri {
		if err := spider.CollectApiTest(s); err != nil {
			response.Failed(fmt.Sprint("资源接口测试失败: ", err.Error()), c)
			return
		}
	}
	if err := service.CollectSvc.UpdateFilmSource(s); err != nil {
		response.Failed(fmt.Sprint("资源站更新失败: ", err.Error()), c)
		return
	}
	response.SuccessOnlyMsg("更新成功", c)
}

func (h *CollectHandler) FilmSourceChange(c *gin.Context) {
	s := model.FilmSource{}
	if err := c.ShouldBindJSON(&s); err != nil {
		response.Failed("请求参数异常", c)
		return
	}
	if s.Id == "" {
		response.Failed("参数异常, 资源站标识不能为空", c)
		return
	}
	fs := service.CollectSvc.GetFilmSource(s.Id)
	if fs == nil {
		response.Failed("数据异常,资源站信息不存在", c)
		return
	}
	if s.SyncPictures && (fs.Grade == model.SlaveCollect) {
		response.Failed("附属站点无法开启图片同步功能", c)
		return
	}
	if s.State != fs.State || s.SyncPictures != fs.SyncPictures {
		upds := model.FilmSource{
			Id:           fs.Id,
			Name:         fs.Name,
			Uri:          fs.Uri,
			ResultModel:  fs.ResultModel,
			Grade:        fs.Grade,
			SyncPictures: s.SyncPictures,
			CollectType:  fs.CollectType,
			State:        s.State,
			Interval:     fs.Interval,
		}
		if err := service.CollectSvc.UpdateFilmSource(upds); err != nil {
			response.Failed(fmt.Sprint("资源站更新失败: ", err.Error()), c)
			return
		}
	}
	response.SuccessOnlyMsg("更新成功", c)
}

func (h *CollectHandler) FilmSourceDel(c *gin.Context) {
	id := c.Query("id")
	if len(id) <= 0 {
		response.Failed("资源站ID信息不能为空", c)
		return
	}
	if spider.IsTaskRunning(id) {
		response.Failed("站点正在采集, 请先停止采集后再尝试删除操作", c)
		return
	}
	if err := service.CollectSvc.DelFilmSource(id); err != nil {
		response.Failed("删除资源站失败", c)
		return
	}
	response.SuccessOnlyMsg("删除成功", c)
}

func (h *CollectHandler) FilmSourceTest(c *gin.Context) {
	s := model.FilmSource{}
	if err := c.ShouldBindJSON(&s); err != nil {
		response.Failed("请求参数异常", c)
		return
	}
	if err := validFilmSource(s); err != nil {
		response.Failed(err.Error(), c)
		return
	}
	if err := spider.CollectApiTest(s); err != nil {
		response.Failed(err.Error(), c)
		return
	}
	response.SuccessOnlyMsg("测试成功!!!", c)
}

func (h *CollectHandler) CollectingState(c *gin.Context) {
	ids := spider.GetActiveTasks()
	response.Success(ids, "正在采集的任务 ID 列表获取成功", c)
}

func (h *CollectHandler) StopCollect(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		response.Failed("非法请求, 缺失站点ID", c)
		return
	}
	spider.StopTask(id)
	response.SuccessOnlyMsg("采集任务正在停止...", c)
}

func (h *CollectHandler) GetNormalFilmSource(c *gin.Context) {
	var l []model.FilmTaskOptions
	for _, v := range service.CollectSvc.GetFilmSourceList() {
		if v.State {
			l = append(l, model.FilmTaskOptions{Id: v.Id, Name: v.Name})
		}
	}
	response.Success(l, "影视源信息获取成功", c)
}

// ------------------------------------------------------ 失败采集记录 ------------------------------------------------------

func (h *CollectHandler) FailureRecordList(c *gin.Context) {
	params := model.RecordRequestVo{Paging: &response.Page{}}
	var err error
	params.OriginId = c.DefaultQuery("originId", "")
	params.Hour, _ = strconv.Atoi(c.DefaultQuery("hour", "0"))
	params.Status, _ = strconv.Atoi(c.DefaultQuery("status", "-1"))

	begin := c.DefaultQuery("beginTime", "")
	if begin != "" {
		beginTime, e := time.ParseInLocation(time.DateTime, begin, time.Local)
		if e != nil {
			response.Failed("影片分页数据获取失败, 请求参数异常", c)
			return
		}
		params.BeginTime = beginTime
	}
	end := c.DefaultQuery("endTime", "")
	if end != "" {
		endTime, e := time.ParseInLocation(time.DateTime, end, time.Local)
		if e != nil {
			response.Failed("影片分页数据获取失败, 请求参数异常", c)
			return
		}
		params.EndTime = endTime
	}

	params.Paging.Current, _ = strconv.Atoi(c.DefaultQuery("current", "1"))
	params.Paging.PageSize, err = strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if err != nil {
		response.Failed("影片分页数据获取失败, 分页参数异常", c)
		return
	}
	if params.Paging.PageSize <= 0 || params.Paging.PageSize > 500 {
		params.Paging.PageSize = 10
	}

	options := service.CollectSvc.GetRecordOptions()
	list := service.CollectSvc.GetRecordList(params)
	response.Success(gin.H{"params": params, "list": list, "options": options}, "影片分页信息获取成功", c)
}

func (h *CollectHandler) CollectRecover(c *gin.Context) {
	id, err := strconv.Atoi(c.DefaultQuery("id", "0"))
	if err != nil && id != 0 {
		response.Failed("采集重试开启失败, 采集记录ID参数异常", c)
		return
	}
	err = service.CollectSvc.CollectRecover(id)
	if err != nil {
		response.Failed(err.Error(), c)
		return
	}
	response.SuccessOnlyMsg("采集重试已开启, 请勿重复操作", c)
}

func (h *CollectHandler) CollectRecoverAll(c *gin.Context) {
	service.CollectSvc.RecoverAll()
	response.SuccessOnlyMsg("恢复任务已成功开启!!!", c)
}

func (h *CollectHandler) ClearDoneRecord(c *gin.Context) {
	service.CollectSvc.ClearDoneRecord()
	response.SuccessOnlyMsg("处理完成的记录信息已删除!!!", c)
}

func (h *CollectHandler) ClearAllRecord(c *gin.Context) {
	service.CollectSvc.ClearAllRecord()
	response.SuccessOnlyMsg("采集异常记录信息已清空!!!", c)
}
