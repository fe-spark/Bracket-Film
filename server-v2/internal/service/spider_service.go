package service

import (
	"errors"
	"log"

	"server-v2/internal/model"
	"server-v2/internal/repository"
	"server-v2/internal/spider"
)

type SpiderService struct{}

var SpiderSvc = new(SpiderService)

// StartCollect 执行对指定站点的采集任务
func (s *SpiderService) StartCollect(id string, h int) error {
	fs := repository.FindCollectSourceById(id)
	if fs == nil {
		return errors.New("采集任务开启失败，采集站信息不存在")
	}
	if !fs.State {
		return errors.New("采集任务开启失败，该采集站已被禁用，请先启用后再采集")
	}
	// 从站必须在主站有数据后才能采集
	if fs.Grade == model.SlaveCollect && !repository.HasMasterData() {
		return errors.New("附属站点采集失败，请先完成主站采集")
	}
	go func() {
		err := spider.HandleCollect(id, h)
		if err != nil {
			log.Printf("[SpiderService] 资源站[%s]采集任务执行失败: %s", id, err)
		}
	}()
	return nil
}

// BatchCollect 批量采集
func (s *SpiderService) BatchCollect(time int, ids []string) error {
	// 如果包含从站且主站尚无数据，驱逐所有从站 id，只保留主站
	if !repository.HasMasterData() {
		var masterOnlyIds []string
		for _, id := range ids {
			if fs := repository.FindCollectSourceById(id); fs != nil && fs.Grade == model.MasterCollect {
				masterOnlyIds = append(masterOnlyIds, id)
			}
		}
		if len(masterOnlyIds) == 0 {
			return errors.New("采集未开启：所选内容均为附属站点，请先创建进行主站采集")
		}
		ids = masterOnlyIds
		log.Println("[BatchCollect] 主站无数据，已自动屏術附属站点")
	}
	go spider.BatchCollect(time, ids...)
	return nil
}

// AutoCollect 自动采集
func (s *SpiderService) AutoCollect(time int) {
	go spider.AutoCollect(time)
}

// ClearFilms 删除采集的数据信息
func (s *SpiderService) ClearFilms() {
	go spider.ClearSpider()
}

// ClearRedisOnly 仅清空 Redis 缓存
func (s *SpiderService) ClearRedisOnly() {
	go spider.ClearRedisOnly()
}

// ZeroCollect 数据清除从零开始采集
func (s *SpiderService) ZeroCollect(time int) {
	go spider.StarZero(time)
}

// SyncCollect 同步采集
func (s *SpiderService) SyncCollect(ids string) {
	go spider.CollectSingleFilm(ids)
}

// FilmClassCollect 影视分类采集, 直接覆盖当前分类数据
func (s *SpiderService) FilmClassCollect() error {
	l := repository.GetCollectSourceListByGrade(model.MasterCollect)
	if l == nil {
		return errors.New("未获取到主采集站信息")
	}
	for _, fs := range l {
		if fs.State {
			go spider.CollectCategory(&fs)
			return nil
		}
	}
	return errors.New("未获取到已启用的主采集站信息")
}

// HasMasterData 查询主站是否已有采集数据
func (s *SpiderService) HasMasterData() bool {
	return repository.HasMasterData()
}
