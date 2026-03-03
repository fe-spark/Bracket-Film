package service

import (
	"errors"
	"fmt"
	"time"

	"server-v2/internal/model"
	"server-v2/internal/repository"
	"server-v2/internal/spider"
	"server-v2/pkg/utils"
)

type CronService struct{}

var CronSvc = new(CronService)

// AddFilmCrontab 添加影片更新任务
func (s *CronService) AddFilmCrontab(cv model.FilmCronVo) error {
	if err := spider.ValidSpec(cv.Spec); err != nil {
		return err
	}
	task := model.FilmCollectTask{Id: utils.GenerateSalt(), Ids: cv.Ids, Time: cv.Time, Spec: cv.Spec, Model: cv.Model, State: cv.State, Remark: cv.Remark}
	switch task.Model {
	case 0:
		cid, err := spider.AddAutoUpdateCron(task.Id, task.Spec)
		if err != nil {
			return errors.New(fmt.Sprint("影视自动更新任务添加失败: ", err.Error()))
		}
		task.Cid = cid
	case 1:
		cid, err := spider.AddFilmUpdateCron(task.Id, task.Spec)
		if err != nil {
			return errors.New(fmt.Sprint("影视更新定时任务添加失败: ", err.Error()))
		}
		task.Cid = cid
	case 2:
		cid, err := spider.AddFilmRecoverCron(task.Spec)
		if err != nil {
			return errors.New(fmt.Sprint("失败采集处理定时任务添加失败: ", err.Error()))
		}
		task.Cid = cid
	case 3:
		cid, err := spider.AddOrphanCleanCron(task.Spec)
		if err != nil {
			return errors.New(fmt.Sprint("孤儿数据清理定时任务添加失败: ", err.Error()))
		}
		task.Cid = cid
	}
	spider.RegisterTaskCid(task.Id, task.Cid)
	repository.SaveFilmTask(task)
	return nil
}

// GetFilmCrontab 获取所有定时任务信息
func (s *CronService) GetFilmCrontab() []model.CronTaskVo {
	cst := time.FixedZone("UTC", 8*3600)
	var l []model.CronTaskVo
	tl := repository.GetAllFilmTask()
	for _, t := range tl {
		e := spider.GetEntryByTaskId(t.Id)
		taskVo := model.CronTaskVo{FilmCollectTask: t, PreV: e.Prev.In(cst).Format(time.DateTime), Next: e.Next.In(cst).Format(time.DateTime)}
		l = append(l, taskVo)
	}
	return l
}

// GetFilmCrontabById 通过ID获取对应的定时任务信息
func (s *CronService) GetFilmCrontabById(id string) (model.FilmCollectTask, error) {
	t, err := repository.GetFilmTaskById(id)
	return t, err
}

// ChangeFilmCrontab 改变定时任务的状态 开启 | 停止
func (s *CronService) ChangeFilmCrontab(id string, state bool) error {
	ft, err := repository.GetFilmTaskById(id)
	if err != nil {
		return fmt.Errorf("定时任务停止失败: %w", err)
	}
	ft.State = state
	repository.UpdateFilmTask(ft)
	return err
}

// UpdateFilmCron 更新定时任务的状态信息
func (s *CronService) UpdateFilmCron(t model.FilmCollectTask) {
	repository.UpdateFilmTask(t)
}

// DelFilmCrontab 删除定时任务
func (s *CronService) DelFilmCrontab(id string) error {
	if _, err := repository.GetFilmTaskById(id); err != nil {
		return fmt.Errorf("定时任务删除失败: %w", err)
	}
	spider.RemoveCronByTaskId(id)
	repository.DelFilmTask(id)
	return nil
}
