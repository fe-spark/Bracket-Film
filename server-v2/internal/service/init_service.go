package service

import (
	"log"

	"server-v2/config"
	"server-v2/internal/model"
	"server-v2/internal/repository"
	"server-v2/internal/spider"
	"server-v2/pkg/utils"
)

type InitService struct{}

var InitSvc = new(InitService)

func (s *InitService) DefaultDataInit() {
	repository.RedisOnlyFlush()

	if !repository.ExistUserTable() {
		s.TableInit()
	}

	s.BasicConfigInit()
	s.BannersInit()
	s.SpiderInit()
}

func (s *InitService) TableInit() {
	repository.CreateUserTable()
	repository.InitAdminAccount()
	repository.CreateSearchTable()
	repository.CreateFileTable()
	repository.CreateFailureRecordTable()
	repository.CreateMovieDetailTable()
	repository.CreateCategoryTable()
	repository.CreateMoviePlaylistTable()
	repository.CreateVirtualPictureTable()
	repository.CreateFilmSourceTable()
	repository.CreateSearchTagTable()
	repository.CreateCrontabTable()
	repository.CreateSiteConfigTable()
	repository.CreateBannersTable()
}

func (s *InitService) BasicConfigInit() {
	if repository.ExistSiteConfig() {
		return
	}
	bc := model.BasicConfig{
		SiteName: "Bracket",
		Domain:   "http://127.0.0.1:3600",
		Logo:     "https://s2.loli.net/2023/12/05/O2SEiUcMx5aWlv4.jpg",
		Keyword:  "在线视频, 免费观影",
		Describe: "自动采集, 多播放源集成,在线观影网站",
		State:    true,
		Hint:     "网站升级中, 暂时无法访问 !!!",
	}
	_ = repository.SaveSiteBasic(bc)
}

func (s *InitService) BannersInit() {
	if repository.ExistBannersConfig() {
		return
	}
	bl := model.Banners{
		model.Banner{Id: utils.GenerateSalt(), Name: "樱花庄的宠物女孩", Year: 2020, CName: "日韩动漫", Poster: "https://s2.loli.net/2024/02/21/Wt1QDhabdEI7HcL.jpg", Picture: "https://img.bfzypic.com/upload/vod/20230424-43/06e79232a4650aea00f7476356a49847.jpg", Remark: "已完结"},
		model.Banner{Id: utils.GenerateSalt(), Name: "从零开始的异世界生活", Year: 2020, CName: "日韩动漫", Poster: "https://s2.loli.net/2024/02/21/UkpdhIRO12fsy6C.jpg", Picture: "https://img.bfzypic.com/upload/vod/20230424-43/06e79232a4650aea00f7476356a49847.jpg", Remark: "已完结"},
		model.Banner{Id: utils.GenerateSalt(), Name: "五等分的花嫁", Year: 2020, CName: "日韩动漫", Poster: "https://s2.loli.net/2024/02/21/wXJr59Zuv4tcKNp.jpg", Picture: "https://img.bfzypic.com/upload/vod/20230424-43/06e79232a4650aea00f7476356a49847.jpg", Remark: "已完结"},
		model.Banner{Id: utils.GenerateSalt(), Name: "我的青春恋爱物语果然有问题", Year: 2020, CName: "日韩动漫", Poster: "https://s2.loli.net/2024/02/21/oMAGzSliK2YbhRu.jpg", Picture: "https://img.bfzypic.com/upload/vod/20230424-43/06e79232a4650aea00f7476356a49847.jpg", Remark: "已完结"},
	}
	_ = repository.SaveBanners(bl)
}

func (s *InitService) SpiderInit() {
	s.FilmSourceInit()
	s.CollectCrontabInit()
}

func (s *InitService) FilmSourceInit() {
	if repository.ExistCollectSourceList() {
		return
	}
	l := []model.FilmSource{
		{Id: utils.GenerateSalt(), Name: "HD(LZ)", Uri: `https://cj.lziapi.com/api.php/provide/vod/`, ResultModel: model.JsonResult, Grade: model.MasterCollect, SyncPictures: false, CollectType: model.CollectVideo, State: false},
		{Id: utils.GenerateSalt(), Name: "HD(BF)", Uri: `https://bfzyapi.com/api.php/provide/vod/`, ResultModel: model.JsonResult, Grade: model.SlaveCollect, SyncPictures: false, CollectType: model.CollectVideo, State: false, Interval: 2500},
		{Id: utils.GenerateSalt(), Name: "HD(FF)", Uri: `http://cj.ffzyapi.com/api.php/provide/vod/`, ResultModel: model.JsonResult, Grade: model.SlaveCollect, SyncPictures: false, CollectType: model.CollectVideo, State: false},
		{Id: utils.GenerateSalt(), Name: "HD(OK)", Uri: `https://okzyapi.com/api.php/provide/vod/`, ResultModel: model.JsonResult, Grade: model.SlaveCollect, SyncPictures: false, CollectType: model.CollectVideo, State: false},
		{Id: utils.GenerateSalt(), Name: "HD(HM)", Uri: `https://json.heimuer.xyz/api.php/provide/vod/`, ResultModel: model.JsonResult, Grade: model.SlaveCollect, SyncPictures: false, CollectType: model.CollectVideo, State: false},
		{Id: utils.GenerateSalt(), Name: "HD(LY)", Uri: `https://360zy.com/api.php/provide/vod/at/json`, ResultModel: model.JsonResult, Grade: model.SlaveCollect, SyncPictures: false, CollectType: model.CollectVideo, State: false},
		{Id: utils.GenerateSalt(), Name: "HD(SN)", Uri: `https://suoniapi.com/api.php/provide/vod/from/snm3u8/`, ResultModel: model.JsonResult, Grade: model.SlaveCollect, SyncPictures: false, CollectType: model.CollectVideo, State: false, Interval: 2000},
		{Id: utils.GenerateSalt(), Name: "HD(DB)", Uri: `https://caiji.dbzy.tv/api.php/provide/vod/from/dbm3u8/at/json/`, ResultModel: model.JsonResult, Grade: model.SlaveCollect, SyncPictures: false, CollectType: model.CollectVideo, State: false},
		{Id: utils.GenerateSalt(), Name: "HD(IK)", Uri: `https://ikunzyapi.com/api.php/provide/vod/at/json`, ResultModel: model.JsonResult, Grade: model.SlaveCollect, SyncPictures: false, CollectType: model.CollectVideo, State: false},
	}
	if err := repository.BatchAddCollectSource(l); err != nil {
		log.Println("BatchAddCollectSource Error: ", err)
	}
}

func (s *InitService) CollectCrontabInit() {
	if repository.ExistTask() {
		for _, task := range repository.GetAllFilmTask() {
			switch task.Model {
			case 0:
				cid, err := spider.AddAutoUpdateCron(task.Id, task.Spec)
				if err != nil {
					log.Println("影视自动更新任务添加失败: ", err.Error())
					continue
				}
				task.Cid = cid
			case 1:
				cid, err := spider.AddFilmUpdateCron(task.Id, task.Spec)
				if err != nil {
					log.Println("影视更新定时任务添加失败: ", err.Error())
					continue
				}
				task.Cid = cid
			case 2:
				cid, err := spider.AddFilmRecoverCron(task.Id, task.Spec)
				if err != nil {
					log.Println("自动清理失败采集记录定时任务添加失败: ", err.Error())
					continue
				}
				task.Cid = cid
			case 3:
				cid, err := spider.AddOrphanCleanCron(task.Id, task.Spec)
				if err != nil {
					log.Println("孤儿数据清理定时任务添加失败: ", err.Error())
					continue
				}
				task.Cid = cid
			}
			spider.RegisterTaskCid(task.Id, task.Cid)
			repository.UpdateFilmTask(task)
		}
	} else {
		task := model.FilmCollectTask{
			Id: utils.GenerateSalt(), Time: config.DefaultUpdateTime, Spec: config.DefaultUpdateSpec,
			Model: 0, State: false, Remark: "每20分钟执行一次已启用站点数据的自动更新",
		}
		cid, err := spider.AddAutoUpdateCron(task.Id, task.Spec)
		if err != nil {
			log.Println("影视更新定时任务添加失败: ", err.Error())
			return
		}
		task.Cid = cid
		spider.RegisterTaskCid(task.Id, cid)
		repository.SaveFilmTask(task)

		recoverTask := model.FilmCollectTask{
			Id: utils.GenerateSalt(), Time: 0, Spec: config.EveryWeekSpec,
			Model: 2, State: false, Remark: "每周日凌晈12点4点清理一次采集失败的采集记录",
		}
		cid, err = spider.AddFilmRecoverCron(recoverTask.Id, recoverTask.Spec)
		if err != nil {
			log.Println("失败采集恢复定时任务添加失败: ", err.Error())
			return
		}
		recoverTask.Cid = cid
		spider.RegisterTaskCid(recoverTask.Id, cid)
		repository.SaveFilmTask(recoverTask)

		orphanTask := model.FilmCollectTask{
			Id: utils.GenerateSalt(), Time: 0, Spec: "0 0 3 * * 0",
			Model: 3, State: false, Remark: "每周日凌晈13点清理已无效的孤儿播放列表记录",
		}
		cid, err = spider.AddOrphanCleanCron(orphanTask.Id, orphanTask.Spec)
		if err != nil {
			log.Println("孤儿数据清理定时任务添加失败: ", err.Error())
			return
		}
		orphanTask.Cid = cid
		spider.RegisterTaskCid(orphanTask.Id, cid)
		repository.SaveFilmTask(orphanTask)
	}

	spider.CronCollect.Start()
}
