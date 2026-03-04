package spider

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"server-v2/config"
	"server-v2/internal/model"
	"server-v2/internal/model/collect"
	"server-v2/internal/repository"
	"server-v2/pkg/conver"
	"server-v2/pkg/utils"
)

/*
	采集逻辑 v3

*/

var spiderCore = &JsonCollect{}

// activeTasks 存储当前活跃采集任务的信息
var activeTasks sync.Map

// taskMu 保护同一站点 cancel+Store 的原子性，防止并发截停竞态
var taskMu sync.Mutex

type collectTask struct {
	cancel context.CancelFunc
	reqId  string
}

// ======================================================= 通用采集方法  =======================================================

// HandleCollect 影视采集  id-采集站ID h-时长/h
func HandleCollect(id string, h int) error {
	// 同站截停：原子地中断旧任务并注册新任务，防止两个并发请求同时通过 Load 后各自 Store
	reqId := utils.GenerateSalt()
	ctx, cancel := context.WithCancel(context.Background())
	taskMu.Lock()
	if val, ok := activeTasks.Load(id); ok {
		log.Printf("[Spider] 站点 %s 已有任务运行，正在抢断旧任务...\n", id)
		val.(collectTask).cancel()
	}
	activeTasks.Store(id, collectTask{cancel: cancel, reqId: reqId})
	taskMu.Unlock()

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
	s := repository.FindCollectSourceById(id)
	if s == nil {
		return errors.New("采集站点不存在")
	} else if !s.State {
		return errors.New("采集站点已停用")
	}

	// 如果是主站点且状态为启用则先获取分类tree信息
	if s.Grade == model.MasterCollect && s.State {
		// 是否存在分类树信息, 不存在则获取
		if !repository.ExistsCategoryTree() {
			CollectCategory(s)
		}
	}

	// 生成 RequestInfo
	r := utils.RequestInfo{Uri: s.Uri, Params: url.Values{}}
	// 如果 h == 0 则直接返回错误信息
	if h == 0 {
		return errors.New("采集时长不能为 0")
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
	case model.CollectVideo:
		// 采集视频资源
		// 如果页数较少, 使用简单的循环串行采集; 否则进入并发模式
		if pageCount <= config.MAXGoroutine*2 {
			for i := 1; i <= pageCount; i++ {
				select {
				case <-ctx.Done():
					log.Printf("[Spider] 站点 %s 采集任务被中断(同步模式)\n", s.Name)
					return nil
				default:
					collectFilm(ctx, s, h, i)
					// 如果设置了采集间隔，每采集完一页后等待
					if s.Interval > 0 {
						time.Sleep(time.Duration(s.Interval) * time.Millisecond)
					}
				}
			}
		} else {
			// 并发模式 (并发 Worker 内部已包含 Sleep 逻辑)
			ConcurrentPageSpider(ctx, pageCount, s, h, collectFilm)
		}
		// 视频数据采集完成后清理相关缓存
		switch s.Grade {
		case model.MasterCollect:
			// 开启图片同步
			if s.SyncPictures {
				repository.SyncFilmPicture()
			}
			// 主站数据变更：清理所有影片相关缓存
			ClearCache()
		case model.SlaveCollect:
			// 从站只更新播放源，仅清 Cache:Play:*
			repository.RemoveCacheByPattern("Cache:Play:*")
		}

	case model.CollectArticle, model.CollectActor, model.CollectRole, model.CollectWebSite:
		log.Println("暂未开放此采集功能!!!")
		return errors.New("暂未开放此采集功能")
	}
	return nil
}

// CollectCategory 影视分类采集
func CollectCategory(s *model.FilmSource) {
	// 获取分类树形数据
	categoryTree, err := spiderCore.GetCategoryTree(utils.RequestInfo{Uri: s.Uri, Params: url.Values{}})
	if err != nil {
		log.Println("GetCategoryTree Error: ", err)
		return
	}
	// 保存 tree 到 MySQL (及可选缓存)
	err = repository.SaveCategoryTree(categoryTree)
	if err != nil {
		log.Println("SaveCategoryTree Error: ", err)
	}
}

// saveCollectedFilm 将已采集的 list 按站点类型写入存储，消除 collectFilm/collectFilmById 中的重复 switch 块。
// saveMaster 由调用方注入，区分批量(SaveDetails)与单条(SaveDetail)两种写入策略。
func saveCollectedFilm(s *model.FilmSource, list []model.MovieDetail, saveMaster func([]model.MovieDetail) error) {
	var err error
	switch s.Grade {
	case model.MasterCollect:
		if err = saveMaster(list); err != nil {
			log.Println("SaveDetails Error: ", err)
		}
		if s.SyncPictures {
			if err = repository.SaveVirtualPic(conver.ConvertVirtualPicture(list)); err != nil {
				log.Println("SaveVirtualPic Error: ", err)
			}
		}
	case model.SlaveCollect:
		if err = repository.SaveSitePlayList(s.Id, list); err != nil {
			log.Println("SaveSitePlayList Error: ", err)
		}
	}
}

// collectFilm 影视详情采集 (单一源分页全采集)
func collectFilm(ctx context.Context, s *model.FilmSource, h, pg int) {
	select {
	case <-ctx.Done():
		return
	default:
	}
	r := utils.RequestInfo{Uri: s.Uri, Params: url.Values{}}
	r.Params.Set("pg", fmt.Sprint(pg))
	if h > 0 {
		r.Params.Set("h", fmt.Sprint(h))
	}
	list, err := spiderCore.GetFilmDetail(r)
	if err != nil || len(list) <= 0 {
		fr := model.FailureRecord{OriginId: s.Id, OriginName: s.Name, Uri: s.Uri, CollectType: model.CollectVideo, PageNumber: pg, Hour: h, Cause: fmt.Sprintln(err), Status: 1}
		repository.SaveFailureRecord(fr)
		log.Println("GetMovieDetail Error: ", err)
		return
	}
	saveCollectedFilm(s, list, repository.SaveDetails)
}

// collectFilmById 采集指定ID的影片信息
func collectFilmById(ids string, s *model.FilmSource) {
	r := utils.RequestInfo{Uri: s.Uri, Params: url.Values{}}
	r.Params.Set("pg", "1")
	r.Params.Set("ids", ids)
	list, err := spiderCore.GetFilmDetail(r)
	if err != nil || len(list) <= 0 {
		log.Println("GetMovieDetail Error: ", err)
		return
	}
	saveCollectedFilm(s, list, func(l []model.MovieDetail) error {
		return repository.SaveDetail(l[0])
	})
}

// ConcurrentPageSpider 并发分页采集, 不限类型
func ConcurrentPageSpider(ctx context.Context, capacity int, s *model.FilmSource, h int, collectFunc func(ctx context.Context, s *model.FilmSource, hour, pageNumber int)) {
	// 开启协程并发执行
	ch := make(chan int, capacity)
	for i := 1; i <= capacity; i++ {
		ch <- i
	}
	close(ch)
	GoroutineNum := config.MAXGoroutine
	if capacity < GoroutineNum {
		GoroutineNum = capacity
	}
	// waitCh 必须带缓冲(容量=GoroutineNum)：ctx 取消时等待循环提前退出，
	// worker 仍会执行 waitCh<-0，无缓冲则永久阻塞导致 goroutine 泄漏
	waitCh := make(chan int, GoroutineNum)
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
					// 如果设置了采集间隔，每个 worker 采集完一页后都要等待
					if s.Interval > 0 {
						time.Sleep(time.Duration(s.Interval) * time.Millisecond)
					}
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
		if fs := repository.FindCollectSourceById(id); fs != nil && fs.State {
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
	for _, s := range repository.GetCollectSourceList() {
		// 如果当前站点为启用状态 则执行 HandleCollect 进行数据采集
		if s.State {
			// 为每个站点开启独立的协程执行，实现并发全量采集
			go func(fs model.FilmSource) {
				if err := HandleCollect(fs.Id, h); err != nil {
					log.Printf("[Spider] 自动采集站点 %s 失败: %v\n", fs.Name, err)
				}
			}(s)
		}
	}
}

// ClearSpider  删除所有已采集的影片信息
func ClearSpider() {
	repository.FilmZero()
}

// ClearRedisOnly 仅清空 Redis 缓存
func ClearRedisOnly() {
	repository.RedisOnlyFlush()
}

// MasterOnlyCollect 仅对已启用的主站执行采集
// 清空重采场景下使用：优先保证主站数据就绪，从站由定时任务补充
func MasterOnlyCollect(h int) {
	masters := repository.GetCollectSourceListByGrade(model.MasterCollect)
	enabled := make([]model.FilmSource, 0)
	for _, s := range masters {
		if s.State {
			enabled = append(enabled, s)
		}
	}
	if len(enabled) == 0 {
		log.Println("[StarZero] 未找到已启用的主站，跳过采集")
		return
	}
	for _, s := range enabled {
		go func(fs model.FilmSource) {
			log.Printf("[StarZero] 开始主站全量采集: %s", fs.Name)
			if err := HandleCollect(fs.Id, h); err != nil {
				log.Printf("[StarZero] 主站采集失败 %s: %v", fs.Name, err)
			}
		}(s)
	}
}

// StarZero 清空所有影视数据后仅重采主站
// 从站数据量大、独立更新，由定时任务或手动触发补采
func StarZero(h int) {
	repository.FilmZero()
	MasterOnlyCollect(h)
}

// CollectSingleFilm 通过影片唯一ID获取影片信息
func CollectSingleFilm(ids string) {
	// 获取采集站列表信息
	fl := repository.GetCollectSourceList()
	// 循环遍历所有采集站信息
	for _, f := range fl {
		// 目前仅对主站点进行处理
		if f.Grade == model.MasterCollect && f.State {
			collectFilmById(ids, &f)
			return
		}
	}
}

// ======================================================= 采集拓展内容  =======================================================

// SingleRecoverSpider 二次采集
func SingleRecoverSpider(fr *model.FailureRecord) {
	// 将记录状态修改为已处理
	repository.ChangeRecord(fr, 0)
	// 仅对当前失败记录所属站点+失败页进行重试，不干扰正在运行的采集任务
	s := repository.FindCollectSourceById(fr.OriginId)
	if s == nil {
		log.Printf("[Spider] 重试失败: 站点 %s 不存在\n", fr.OriginId)
		return
	}
	collectFilm(context.Background(), s, fr.Hour, fr.PageNumber)
}

// FullRecoverSpider 扫描记录表中的失败记录, 并发重试各失败页
func FullRecoverSpider() {
	list := repository.PendingRecord()
	var wg sync.WaitGroup
	for i := range list {
		fr := list[i]
		repository.ChangeRecord(&fr, 0)
		s := repository.FindCollectSourceById(fr.OriginId)
		if s == nil {
			log.Printf("[Spider] 重试失败: 站点 %s 不存在\n", fr.OriginId)
			continue
		}
		wg.Add(1)
		go func(src *model.FilmSource, record model.FailureRecord) {
			defer wg.Done()
			collectFilm(context.Background(), src, record.Hour, record.PageNumber)
		}(s, fr)
	}
	wg.Wait()
}

// ======================================================= 公共方法  =======================================================

// CollectApiTest 测试采集接口是否可用
func CollectApiTest(s model.FilmSource) error {
	// 使用 ac=list 测试：获取分类列表，所有标准 Mac CMS 站均支持，
	// 且不需要额外过滤参数（ac=detail 在无 h/t 参数时部分站点会返回 400）
	r := utils.RequestInfo{Uri: s.Uri, Params: url.Values{}}
	r.Params.Set("ac", "list")
	r.Params.Set("pg", "1")
	err := utils.ApiTest(&r)
	// 首先核对接口返回值类型
	if err == nil {
		// 如果返回值类型为Json则执行Json序列化
		if s.ResultModel == model.JsonResult {
			lp := collect.FilmListPage{}
			if err = json.Unmarshal(r.Resp, &lp); err != nil {
				return errors.New(fmt.Sprint("测试失败, 返回数据异常, JSON序列化失败: ", err))
			}
			return nil
		} else if s.ResultModel == model.XmlResult {
			// 如果返回值类型为XML则执行XML序列化
			rd := collect.RssD{}
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
		log.Printf("[Spider] 已强制停止 %d 个活跃采集任务\n", count)
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
