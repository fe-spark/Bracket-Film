package spider

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"net/url"
	"server/config"
	"server/model/collect"
	"server/model/system"
	"server/plugin/common/conver"
	"server/plugin/common/util"
	"sync"
	"time"
)

/*
	采集逻辑 v3

*/

var spiderCore = &JsonCollect{}

// 存储当前活跃采集任务的信息
var activeTasks sync.Map

type collectTask struct {
	cancel context.CancelFunc
	reqId  string
}

// ======================================================= 通用采集方法  =======================================================

// HandleCollect 影视采集  id-采集站ID h-时长/h
func HandleCollect(id string, h int) error {
	// 1. 同站抢断：中断之前正在进行的该源采集任务
	if val, ok := activeTasks.Load(id); ok {
		log.Printf("[Spider] 站点 %s 已有任务运行，正在抢断旧任务...\n", id)
		val.(collectTask).cancel()
	}

	// 创建新的 context 和唯一请求 ID
	reqId := util.GenerateSalt()
	ctx, cancel := context.WithCancel(context.Background())
	activeTasks.Store(id, collectTask{cancel: cancel, reqId: reqId})

	// 任务完成后清理（仅当当前任务仍是自己时）
	defer func() {
		if val, ok := activeTasks.Load(id); ok {
			if val.(collectTask).reqId == reqId {
				activeTasks.Delete(id)
				log.Printf("[Spider] 站点 %s 任务结束\n", id)
			}
		}
	}()

	log.Printf("[Spider] 站点 %s 任务启动 (reqId: %s)\n", id, reqId)

	// 1. 首先通过ID获取对应采集站信息
	s := system.FindCollectSourceById(id)
	if s == nil {
		log.Println("Cannot Find Collect Source Site")
		return errors.New(" Cannot Find Collect Source Site ")
	} else if !s.State {
		log.Println(" The acquisition site was disabled ")
		return errors.New(" The acquisition site was disabled ")
	}

	// 如果是主站点且状态为启用则先获取分类tree信息
	if s.Grade == system.MasterCollect && s.State {
		// 是否存在分类树信息, 不存在则获取
		if !system.ExistsCategoryTree() {
			CollectCategory(s)
		}
	}

	// 生成 RequestInfo
	r := util.RequestInfo{Uri: s.Uri, Params: url.Values{}}
	// 如果 h == 0 则直接返回错误信息
	if h == 0 {
		log.Println(" Collect time cannot be zero ")
		return errors.New(" Collect time cannot be zer ")
	}
	// 如果 h = -1 则进行全量采集
	if h > 0 {
		r.Params.Set("h", fmt.Sprint(h))
	}
	// 2. 首先获取分页采集的页数
	pageCount, err := spiderCore.GetPageCount(r)
	if err != nil {
		// 分页页数失败 则再进行一次尝试
		pageCount, err = spiderCore.GetPageCount(r)
		if err != nil {
			return err
		}
	}
	// pageCount = 0 说明该站点在当前时间段内无新数据，任务无需执行
	if pageCount <= 0 {
		log.Printf("[Spider] 站点 %s 无需采集 (pageCount=%d，可能该时间段内无新内容)\n", s.Name, pageCount)
		return nil
	}
	log.Printf("[Spider] 站点 %s 共 %d 页，开始采集...\n", s.Name, pageCount)

	// 通过采集类型分别执行不同的采集方法
	switch s.CollectType {
	case system.CollectVideo:
		// 采集视频资源
		if s.Interval > 500 {
			for i := 1; i <= pageCount; i++ {
				select {
				case <-ctx.Done():
					log.Printf("[Spider] 站点 %s 采集任务被中断(单线程模式)\n", s.Name)
					return nil
				default:
					collectFilm(ctx, s, h, i)
					time.Sleep(time.Duration(s.Interval) * time.Millisecond)
				}
			}
		} else if pageCount <= config.MAXGoroutine*2 {
			for i := 1; i <= pageCount; i++ {
				select {
				case <-ctx.Done():
					log.Printf("[Spider] 站点 %s 采集任务被中断(同步模式)\n", s.Name)
					return nil
				default:
					collectFilm(ctx, s, h, i)
				}
			}
		} else {
			// 并发模式
			ConcurrentPageSpider(ctx, pageCount, s, h, collectFilm)
		}
		// 视频数据采集完成后同步相关信息到mysql
		if s.Grade == system.MasterCollect {
			// 执行影片信息更新操作
			if h > 0 {
				// 执行数据更新操作
				system.SyncSearchInfo(1)
			} else {
				// 清空searchInfo中的数据并重新添加, 否则执行
				system.SyncSearchInfo(0)
			}
			// 开启图片同步
			if s.SyncPictures {
				system.SyncFilmPicture()
			}
			// 每次成功执行完都清理redis中的相关API接口数据缓存
			ClearCache()
		}

	case system.CollectArticle, system.CollectActor, system.CollectRole, system.CollectWebSite:
		log.Println("暂未开放此采集功能!!!")
		return errors.New("暂未开放此采集功能")
	}
	log.Println("Spider Task Exercise Success")
	return nil
}

// CollectCategory 影视分类采集
func CollectCategory(s *system.FilmSource) {
	// 获取分类树形数据
	categoryTree, err := spiderCore.GetCategoryTree(util.RequestInfo{Uri: s.Uri, Params: url.Values{}})
	if err != nil {
		log.Println("GetCategoryTree Error: ", err)
		return
	}
	// 保存 tree 到redis
	err = system.SaveCategoryTree(categoryTree)
	if err != nil {
		log.Println("SaveCategoryTree Error: ", err)
	}
}

// collectFilm 影视详情采集 (单一源分页全采集)
func collectFilm(ctx context.Context, s *system.FilmSource, h, pg int) {
	// 检查取消信号
	select {
	case <-ctx.Done():
		return
	default:
	}

	// 生成请求参数
	r := util.RequestInfo{Uri: s.Uri, Params: url.Values{}}
	// 设置分页页数
	r.Params.Set("pg", fmt.Sprint(pg))
	// 如果 h = -1 则进行全量采集
	if h > 0 {
		r.Params.Set("h", fmt.Sprint(h))
	}
	// 执行采集方法 获取影片详情list
	list, err := spiderCore.GetFilmDetail(r)
	if err != nil || len(list) <= 0 {
		// 添加采集失败记录
		fr := system.FailureRecord{OriginId: s.Id, OriginName: s.Name, Uri: s.Uri, CollectType: system.CollectVideo, PageNumber: pg, Hour: h, Cause: fmt.Sprintln(err), Status: 1}
		system.SaveFailureRecord(fr)
		log.Println("GetMovieDetail Error: ", err)
		return
	}
	// 通过采集站 Grade 类型, 执行不同的存储逻辑
	switch s.Grade {
	case system.MasterCollect:
		// 主站点 	保存完整影片详情信息到 redis
		if err = system.SaveDetails(list); err != nil {
			log.Println("SaveDetails Error: ", err)
		}
		// 如果主站点开启了图片同步, 则将图片url以及对应的mid存入ZSet集合中
		if s.SyncPictures {
			if err = system.SaveVirtualPic(conver.ConvertVirtualPicture(list)); err != nil {
				log.Println("SaveVirtualPic Error: ", err)
			}
		}
	case system.SlaveCollect:
		// 附属站点	仅保存影片播放信息到redis
		if err = system.SaveSitePlayList(s.Id, list); err != nil {
			log.Println("SaveDetails Error: ", err)
		}
	}
}

// collectFilmById 采集指定ID的影片信息
func collectFilmById(ids string, s *system.FilmSource) {
	// 生成请求参数
	r := util.RequestInfo{Uri: s.Uri, Params: url.Values{}}
	// 设置分页页数
	r.Params.Set("pg", "1")
	// 设置影片IDS参数信息
	r.Params.Set("ids", ids)
	// 执行采集方法 获取影片详情list
	list, err := spiderCore.GetFilmDetail(r)
	if err != nil || len(list) <= 0 {
		log.Println("GetMovieDetail Error: ", err)
		return
	}
	// 通过采集站 Grade 类型, 执行不同的存储逻辑
	switch s.Grade {
	case system.MasterCollect:
		// 主站点 	保存完整影片详情信息到 redis 和 mysql 中
		if err = system.SaveDetail(list[0]); err != nil {
			log.Println("SaveDetails Error: ", err)
		}
		// 如果主站点开启了图片同步, 则将图片url以及对应的mid存入ZSet集合中
		if s.SyncPictures {
			if err = system.SaveVirtualPic(conver.ConvertVirtualPicture(list)); err != nil {
				log.Println("SaveVirtualPic Error: ", err)
			}
		}
	case system.SlaveCollect:
		// 附属站点	仅保存影片播放信息到redis
		if err = system.SaveSitePlayList(s.Id, list); err != nil {
			log.Println("SaveDetails Error: ", err)
		}
	}
}

// ConcurrentPageSpider 并发分页采集, 不限类型
func ConcurrentPageSpider(ctx context.Context, capacity int, s *system.FilmSource, h int, collectFunc func(ctx context.Context, s *system.FilmSource, hour, pageNumber int)) {
	// 开启协程并发执行
	ch := make(chan int, capacity)
	waitCh := make(chan int)
	for i := 1; i <= capacity; i++ {
		ch <- i
	}
	close(ch)
	// 开启 MAXGoroutine 数量的协程, 如果分页页数小于协程数则将协程数限制为分页页数
	var GoroutineNum = config.MAXGoroutine
	if capacity < GoroutineNum {
		GoroutineNum = capacity
	}
	for i := 0; i < GoroutineNum; i++ {
		go func() {
			defer func() { waitCh <- 0 }()
			for {
				select {
				case <-ctx.Done():
					return
				case pg, ok := <-ch:
					if !ok {
						return
					}
					// 执行对应的采集方法
					collectFunc(ctx, s, h, pg)
				}
			}
		}()
	}
	for i := 0; i < GoroutineNum; i++ {
		select {
		case <-waitCh:
		case <-ctx.Done():
			log.Printf("[Spider] 站点 %s 并发采集任务被中断\n", s.Name)
			return
		}
	}
}

// BatchCollect 批量采集, 采集指定的所有站点最近x小时内更新的数据
func BatchCollect(h int, ids ...string) {
	for _, id := range ids {
		// 如果查询到对应Id的资源站信息, 且资源站处于启用状态
		if fs := system.FindCollectSourceById(id); fs != nil && fs.State {
			// 采用协程并发执行, 每个站点单独开启一个协程执行
			go func(sourceId string, hour int, sourceName string) {
				if err := HandleCollect(sourceId, hour); err != nil {
					log.Printf("[Spider] 批量采集站点 %s 失败: %v\n", sourceName, err)
				}
			}(fs.Id, h, fs.Name)
		}
	}
}

// AutoCollect 自动进行对所有已启用站点的采集任务
func AutoCollect(h int) {
	// 获取采集站中所有站点, 进行遍历
	for _, s := range system.GetCollectSourceList() {
		// 如果当前站点为启用状态 则执行 HandleCollect 进行数据采集
		if s.State {
			// 为每个站点开启独立的协程执行，实现并发全量采集
			go func(fs system.FilmSource) {
				if err := HandleCollect(fs.Id, h); err != nil {
					log.Printf("[Spider] 自动采集站点 %s 失败: %v\n", fs.Name, err)
				}
			}(s)
		}
	}
}

// ClearSpider  删除所有已采集的影片信息
func ClearSpider() {
	system.FilmZero()
}

// StarZero 清空站点内所有影片信息并从零开始采集
func StarZero(h int) {
	// 1. 清除影视信息
	system.FilmZero()

	// 2. 开启自动采集（每个站点的 HandleCollect 会自动抢断同站旧任务）
	AutoCollect(h)
}

// CollectSingleFilm 通过影片唯一ID获取影片信息
func CollectSingleFilm(ids string) {
	// 获取采集站列表信息
	fl := system.GetCollectSourceList()
	// 循环遍历所有采集站信息
	for _, f := range fl {
		// 目前仅对主站点进行处理
		if f.Grade == system.MasterCollect && f.State {
			collectFilmById(ids, &f)
			return
		}
	}
}

// ======================================================= 采集拓展内容  =======================================================

// SingleRecoverSpider 二次采集
func SingleRecoverSpider(fr *system.FailureRecord) {
	// 将记录状态修改为已处理
	system.ChangeRecord(fr, 0)
	// 仅对当前失败记录所属站点+失败页进行重试，不干扰正在运行的采集任务
	s := system.FindCollectSourceById(fr.OriginId)
	if s == nil {
		log.Printf("[Spider] 重试失败: 站点 %s 不存在\n", fr.OriginId)
		return
	}
	collectFilm(context.Background(), s, fr.Hour, fr.PageNumber)
}

// FullRecoverSpider 扫描记录表中的失败记录, 逐条重试对应的失败页
func FullRecoverSpider() {
	list := system.PendingRecord()
	for _, fr := range list {
		// 将记录状态修改为已处理
		system.ChangeRecord(&fr, 0)
		// 仅对当前失败记录所属站点+失败页进行重试
		s := system.FindCollectSourceById(fr.OriginId)
		if s == nil {
			log.Printf("[Spider] 重试失败: 站点 %s 不存在\n", fr.OriginId)
			continue
		}
		collectFilm(context.Background(), s, fr.Hour, fr.PageNumber)
	}
}

// ======================================================= 公共方法  =======================================================

// CollectApiTest 测试采集接口是否可用
func CollectApiTest(s system.FilmSource) error {
	// 使用当前采集站接口采集一页数据
	r := util.RequestInfo{Uri: s.Uri, Params: url.Values{}}
	r.Params.Set("ac", s.CollectType.GetActionType())
	r.Params.Set("pg", "3")
	err := util.ApiTest(&r)
	// 首先核对接口返回值类型
	if err == nil {
		// 如果返回值类型为Json则执行Json序列化
		if s.ResultModel == system.JsonResult {
			var dp = collect.FilmDetailLPage{}
			if err = json.Unmarshal(r.Resp, &dp); err != nil {
				return errors.New(fmt.Sprint("测试失败, 返回数据异常, JSON序列化失败: ", err))
			}
			return nil
		} else if s.ResultModel == system.XmlResult {
			// 如果返回值类型为XML则执行XML序列化
			var rd = collect.RssD{}
			if err = xml.Unmarshal(r.Resp, &rd); err != nil {
				return errors.New(fmt.Sprint("测试失败, 返回数据异常, XML序列化失败", err))
			}
			return nil
		}
		return errors.New("测试失败, 接口返回值类型不符合规范")
	}
	return errors.New(fmt.Sprint("测试失败, 请求响应异常 : ", err.Error()))
}

// GetActiveTasks 返回当前正在采集的任务 ID 列表
func GetActiveTasks() []string {
	ids := make([]string, 0)
	activeTasks.Range(func(key, value any) bool {
		ids = append(ids, key.(string))
		return true
	})
	log.Printf("[Spider] GetActiveTasks 当前活跃任务: %v\n", ids)
	return ids
}

// StopAllTasks 强制停止当前系统中所有正在进行的采集任务
func StopAllTasks() {
	count := 0
	activeTasks.Range(func(key, value any) bool {
		if ct, ok := value.(collectTask); ok {
			ct.cancel()
			count++
		}
		activeTasks.Delete(key)
		return true
	})
	if count > 0 {
		log.Printf("[Spider] 检测到新任务启动，已强制中断当前系统中所有运行的 %d 个活跃采集任务\n", count)
	}
}

// StopTask 强行停止指定站点的采集任务
func StopTask(id string) {
	if val, ok := activeTasks.Load(id); ok {
		val.(collectTask).cancel()
		activeTasks.Delete(id)
	}
}

// IsTaskRunning 查询指定站点的采集任务是否正在运行
func IsTaskRunning(id string) bool {
	_, ok := activeTasks.Load(id)
	return ok
}
