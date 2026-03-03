package spider

import (
	"errors"
	"fmt"
	"log"

	"server-v2/config"
	"server-v2/internal/repository"

	"github.com/robfig/cron/v3"
)

var CronCollect *cron.Cron = CreateCron()

// taskCidMap 运行时内存注册表：task.Id → cron.EntryID
// Cid 是内存值，不持久化到 DB，每次重启重新注册
var taskCidMap = make(map[string]cron.EntryID)

// RegisterTaskCid 将 taskId 与运行时 cron.EntryID 关联
func RegisterTaskCid(taskId string, cid cron.EntryID) {
	taskCidMap[taskId] = cid
}

// GetEntryByTaskId 通过 taskId 查找运行时 cron.Entry（含上次/下次执行时间）
func GetEntryByTaskId(taskId string) cron.Entry {
	if cid, ok := taskCidMap[taskId]; ok {
		return CronCollect.Entry(cid)
	}
	return cron.Entry{}
}

// RemoveCronByTaskId 通过 taskId 删除定时任务并注销注册
func RemoveCronByTaskId(taskId string) {
	if cid, ok := taskCidMap[taskId]; ok {
		CronCollect.Remove(cid)
		delete(taskCidMap, taskId)
	}
}

// CreateCron 创建定时任务
func CreateCron() *cron.Cron {
	return cron.New(cron.WithSeconds())
}

// AddFilmUpdateCron 添加 指定站点的影片更新定时任务
func AddFilmUpdateCron(id, spec string) (cron.EntryID, error) {
	// 校验 spec 表达式的有效性
	if err := ValidSpec(spec); err != nil {
		return -99, errors.New(fmt.Sprint("定时任务添加失败,Cron表达式校验失败: ", err.Error()))
	}
	return CronCollect.AddFunc(spec, func() {
		// 通过创建任务时生成的 Id 获取任务相关数据
		ft, err := repository.GetFilmTaskById(id)
		if err != nil {
			log.Println("FilmCollectCron Exec Failed: ", err)
		}
		// 如果当前定时任务状态为开启则执行对应的采集任务
		if ft.State && ft.Model == 1 {
			// 对指定ids的资源站数据进行更新操作
			BatchCollect(ft.Time, ft.Ids...)
		}
		// 任务执行完毕
		log.Printf("执行一次定时任务: Task[%s]\n", ft.Id)
	})
}

// AddAutoUpdateCron 添加 所有已启用站点的影片更新定时任务
func AddAutoUpdateCron(id, spec string) (cron.EntryID, error) {
	// 校验 spec 表达式的有效性
	if err := ValidSpec(spec); err != nil {
		return -99, errors.New(fmt.Sprint("定时任务添加失败,Cron表达式校验失败: ", err.Error()))
	}
	return CronCollect.AddFunc(spec, func() {
		// 通过 Id 获取任务相关数据
		ft, err := repository.GetFilmTaskById(id)
		if err != nil {
			log.Println("FilmCollectCron Exec Failed: ", err)
		}
		// 开启对系统中已启用站点的自动更新
		if ft.State && ft.Model == 0 {
			AutoCollect(ft.Time)
			log.Println("执行一次自动更新任务")
		}
	})
}

// AddFilmRecoverCron 失败采集记录处理
func AddFilmRecoverCron(spec string) (cron.EntryID, error) {
	// 校验 spec 表达式的有效性
	if err := ValidSpec(spec); err != nil {
		return -99, errors.New(fmt.Sprint("定时任务添加失败,Cron表达式校验失败: ", err.Error()))
	}
	return CronCollect.AddFunc(spec, func() {
		// 执行失败采集记录恢复
		FullRecoverSpider()
		log.Println("执行一次失败采集恢复任务")
	})
}

// RemoveCron 删除定时任务
func RemoveCron(id cron.EntryID) {
	// 通过定时任务EntryID移出对应的定时任务
	CronCollect.Remove(id)
}

// GetEntryById 返回定时任务的相关时间信息
func GetEntryById(id cron.EntryID) cron.Entry {
	return CronCollect.Entry(id)
}

// ValidSpec 校验cron表达式是否有效
func ValidSpec(spec string) error {
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	_, err := parser.Parse(spec)
	return err
}

// AddOrphanCleanCron 添加孤儿数据清理定时任务
// 定期删除 movie_playlists 中 movie_key 不再匹配任何 search_infos 记录的孤儿行
func AddOrphanCleanCron(spec string) (cron.EntryID, error) {
	if err := ValidSpec(spec); err != nil {
		return -99, errors.New(fmt.Sprint("定时任务添加失败，Cron 表达式校验失败: ", err.Error()))
	}
	return CronCollect.AddFunc(spec, func() {
		n := repository.CleanOrphanPlaylists()
		log.Printf("执行一次孤儿数据清理任务，共删除 %d 条记录\n", n)
	})
}

// ClearCache 清理API接口数据缓存
func ClearCache() {
	repository.RemoveCache(config.IndexCacheKey)
	repository.RemoveCacheByPattern("MovieDetail:*")
	repository.RemoveCacheByPattern("MovieBasicInfo:*")
	repository.RemoveCacheByPattern("Cache:List:*")
	repository.RemoveCacheByPattern("Cache:Hot:*")
	repository.RemoveCacheByPattern("Cache:Search:*")
	repository.RemoveCacheByPattern("Cache:Tag:*")
}
