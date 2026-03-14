package handler

import (
	"errors"
	"fmt"
	"strings"

	"server/internal/model"
	"server/internal/model/dto"
	"server/internal/service"
	"server/internal/spider"

	"github.com/gin-gonic/gin"
)

type CronHandler struct{}

var CronHd = new(CronHandler)

// FilmCronTaskList 获取所有的定时任务信息
func (h *CronHandler) FilmCronTaskList(c *gin.Context) {
	tl := service.CronSvc.GetFilmCrontab()
	dto.Success(tl, "定时任务列表获取成功", c)
}

// GetFilmCronTask 通过Id获取对应的定时任务信息
func (h *CronHandler) GetFilmCronTask(c *gin.Context) {
	id := c.DefaultQuery("id", "")
	if id == "" {
		dto.Failed("定时任务信息获取失败,任务Id不能为空", c)
		return
	}
	task, err := service.CronSvc.GetFilmCrontabById(id)
	if err != nil {
		dto.Failed(fmt.Sprint("定时任务信息获取失败", err.Error()), c)
		return
	}
	dto.Success(task, "定时任务详情获取成功!!!", c)
}

// FilmCronAdd 添加定时任务
func (h *CronHandler) FilmCronAdd(c *gin.Context) {
	vo := model.FilmCronVo{}
	if err := c.ShouldBindJSON(&vo); err != nil {
		dto.Failed("请求参数异常!!!", c)
		return
	}
	if err := validTaskAddVo(vo); err != nil {
		dto.Failed(err.Error(), c)
		return
	}
	vo.Spec = strings.TrimSpace(vo.Spec)
	if err := service.CronSvc.AddFilmCrontab(vo); err != nil {
		dto.Failed(fmt.Sprint("定时任务添加失败: ", err.Error()), c)
		return
	}
	dto.SuccessOnlyMsg("定时任务添加成功", c)
}

// FilmCronUpdate 更新定时任务信息
func (h *CronHandler) FilmCronUpdate(c *gin.Context) {
	t := model.FilmCollectTask{}
	if err := c.ShouldBindJSON(&t); err != nil {
		dto.Failed("请求参数异常!!!", c)
		return
	}
	task, err := service.CronSvc.GetFilmCrontabById(t.Id)
	if err != nil {
		dto.Failed(fmt.Sprint("更新失败: ", err.Error()), c)
		return
	}
	if err := validTaskInfo(t, task.Model); err != nil {
		dto.Failed(err.Error(), c)
		return
	}
	task.Ids = t.Ids
	task.Time = t.Time
	task.State = t.State
	task.Remark = t.Remark
	service.CronSvc.UpdateFilmCron(task)
	dto.SuccessOnlyMsg(fmt.Sprintf("定时任务[%s]更新成功", task.Id), c)
}

// ChangeTaskState 开启 | 关闭Id 对应的定时任务
func (h *CronHandler) ChangeTaskState(c *gin.Context) {
	t := model.FilmCollectTask{}
	if err := c.ShouldBindJSON(&t); err != nil {
		dto.Failed("请求参数异常!!!", c)
		return
	}
	if err := service.CronSvc.ChangeFilmCrontab(t.Id, t.State); err != nil {
		dto.Failed(fmt.Sprint("更新失败: ", err.Error()), c)
		return
	}
	dto.SuccessOnlyMsg(fmt.Sprintf("定时任务[%s]更新成功", t.Id), c)
}

// DelFilmCron 删除定时任务
func (h *CronHandler) DelFilmCron(c *gin.Context) {
	id := c.DefaultQuery("id", "")
	if id == "" {
		dto.Failed("定时任务清除失败, 任务ID不能为空", c)
		return
	}
	if err := service.CronSvc.DelFilmCrontab(id); err != nil {
		dto.Failed(err.Error(), c)
		return
	}
	dto.SuccessOnlyMsg(fmt.Sprintf("定时任务[%s]已删除", id), c)
}

func validTaskInfo(t model.FilmCollectTask, modelType int) error {
	if len(t.Id) <= 0 {
		return errors.New("参数校验失败, 任务Id信息不能为空")
	}
	if (modelType == 0 || modelType == 1) && t.Time == 0 {
		return errors.New("参数校验失败, 采集时长不能为零值")
	}
	if modelType == 1 && len(t.Ids) <= 0 {
		return errors.New("参数校验失败, 自定义更新未绑定任何资源站点")
	}
	return nil
}

func validTaskAddVo(vo model.FilmCronVo) error {
	switch vo.Model {
	case 0:
		if vo.Time == 0 {
			return errors.New("参数校验失败, 采集时长不能为零值")
		}
	case 1:
		if vo.Time == 0 {
			return errors.New("参数校验失败, 采集时长不能为零值")
		}
		if len(vo.Ids) <= 0 {
			return errors.New("参数校验失败, 自定义更新未绑定任何资源站点")
		}
	case 2:
		break
	case 3:
		break
	default:
		return errors.New("参数校验失败, 未定义的任务类型")
	}
	if err := spider.ValidSpec(vo.Spec); err != nil {
		return errors.New(fmt.Sprint("参数校验失败 cron表达式校验失败: ", err.Error()))
	}
	return nil
}
