package service

import (
	"errors"
	"log"

	"server-v2/internal/model"
	"server-v2/internal/repository"
	"server-v2/internal/spider"
)

type CollectService struct{}

var CollectSvc = new(CollectService)

func (s *CollectService) GetFilmSourceList() []model.FilmSource {
	return repository.GetCollectSourceList()
}

func (s *CollectService) GetFilmSource(id string) *model.FilmSource {
	return repository.FindCollectSourceById(id)
}

func (s *CollectService) UpdateFilmSource(source model.FilmSource) error {
	old := repository.FindCollectSourceById(source.Id)
	if old == nil {
		return errors.New("采集站信息不存在")
	}
	// 检测到主站切换: 原来是附属站、现在升级为主站，或 URI 发生变更
	// 旧主站数据的 mid 与新主站不同，需要清理 search_infos / movie_detail_infos / category
	// 保留 movie_playlists（附属站数据按名字 hash 匹配，切换后仍可复用）
	masterSwitch := old.Grade == model.SlaveCollect && source.Grade == model.MasterCollect
	masterUriChanged := old.Grade == model.MasterCollect && source.Grade == model.MasterCollect && old.Uri != source.Uri
	if masterSwitch || masterUriChanged {
		log.Printf("[Collect] 检测到主站变更 (switch=%v, uriChanged=%v)，清理旧主站数据...", masterSwitch, masterUriChanged)
		repository.MasterFilmZero()
	}
	return repository.UpdateCollectSource(source)
}

func (s *CollectService) SaveFilmSource(source model.FilmSource) error {
	return repository.AddCollectSource(source)
}

func (s *CollectService) DelFilmSource(id string) error {
	src := repository.FindCollectSourceById(id)
	if src == nil {
		return errors.New("当前资源站信息不存在, 请勿重复操作")
	}
	if src.Grade == model.MasterCollect {
		return errors.New("主站点无法直接删除, 请先降级为附属站点再进行删除")
	}
	repository.DelCollectResource(id)
	return nil
}

func (s *CollectService) GetRecordList(params model.RecordRequestVo) []model.FailureRecord {
	return repository.FailureRecordList(params)
}

func (s *CollectService) GetRecordOptions() model.OptionGroup {
	options := make(model.OptionGroup)
	options["collectType"] = []model.Option{{Name: "全部", Value: -1}, {Name: "影片详情", Value: 0}, {Name: "文章", Value: 1}, {Name: "演员", Value: 2}, {Name: "角色", Value: 3}, {Name: "网站", Value: 4}}
	options["status"] = []model.Option{{Name: "全部", Value: -1}, {Name: "待重试", Value: 1}, {Name: "已处理", Value: 0}}

	originOptions := []model.Option{{Name: "全部", Value: ""}}
	for _, v := range repository.GetCollectSourceList() {
		originOptions = append(originOptions, model.Option{Name: v.Name, Value: v.Id})
	}
	options["origin"] = originOptions
	return options
}

func (s *CollectService) CollectRecover(id int) error {
	fr := repository.FindRecordById(uint(id))
	if fr == nil {
		return errors.New("采集重试执行失败: 失败记录信息获取异常")
	}
	go spider.SingleRecoverSpider(fr)
	return nil
}

func (s *CollectService) RecoverAll() {
	go spider.FullRecoverSpider()
}

func (s *CollectService) ClearDoneRecord() {
	repository.DelDoneRecord()
}

func (s *CollectService) ClearAllRecord() {
	repository.TruncateRecordTable()
}
