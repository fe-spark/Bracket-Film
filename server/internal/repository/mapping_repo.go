
package repository

import (
	"server/internal/infra/db"
	"server/internal/model"
	"sync"
)

var (
	cacheAreaMap sync.Map
	cacheLangMap sync.Map
	cacheBlackList sync.Map // key is the word, value is bool
)

// InitMappingEngine 从数据库加载映射规则并初始化内存缓存
func InitMappingEngine() {
	ReloadMappingRules()
}

// ReloadMappingRules 强制重新从数据库加载所有映射规则
func ReloadMappingRules() {
	var rules []model.MappingRule
	db.Mdb.Find(&rules)

	// 如果数据库为空，尝试从代码中的初始逻辑进行同步 (仅在首次运行或清库后触发)
	if len(rules) == 0 {
		syncInitialRulesToDB()
		db.Mdb.Find(&rules)
	}

	// 清理并更新缓存
	newArea := make(map[string]string)
	newLang := make(map[string]string)
	newBlack := make(map[string]bool)

	for _, r := range rules {
		switch r.Group {
		case "Area":
			newArea[r.Raw] = r.Target
		case "Language":
			newLang[r.Raw] = r.Target
		case "Blacklist":
			newBlack[r.Raw] = true
		}
	}

	// 批量更新 sync.Map (简单粗暴的替换)
	cacheAreaMap.Range(func(key, value interface{}) bool {
		cacheAreaMap.Delete(key)
		return true
	})
	for k, v := range newArea {
		cacheAreaMap.Store(k, v)
	}

	cacheLangMap.Range(func(key, value interface{}) bool {
		cacheLangMap.Delete(key)
		return true
	})
	for k, v := range newLang {
		cacheLangMap.Store(k, v)
	}

	cacheBlackList.Range(func(key, value interface{}) bool {
		cacheBlackList.Delete(key)
		return true
	})
	for k := range newBlack {
		cacheBlackList.Store(k, true)
	}
}

// syncInitialRulesToDB 将原本硬编码在代码里的基础映射同步到数据库
func syncInitialRulesToDB() {
	// 地区映射
	areas := map[string]string{
		"内地": "大陆", "中国": "大陆", "中国大陆": "大陆", "中国内地": "大陆",
		"韩国": "韩国", "南韩": "韩国",
		"日本": "日本",
		"台湾": "台湾", "中国台湾": "台湾",
		"香港": "香港", "中国香港": "香港",
		"美国": "美国", "欧美": "美国",
		"英国": "英国", "泰国": "泰国", "海外": "其他",
	}
	for k, v := range areas {
		db.Mdb.FirstOrCreate(&model.MappingRule{Group: "Area", Raw: k, Target: v})
	}

	// 语言映射
	langs := map[string]string{
		"普通话": "国语", "汉语普通话": "国语", "华语": "国语", "中文字幕": "国语",
		"粤语": "粤语", "坎语": "粤语",
		"韩语": "韩语", "韩国语": "韩语",
		"日语": "日语", "日本语": "日语",
		"英语": "英语", "英语中字": "英语",
	}
	for k, v := range langs {
		db.Mdb.FirstOrCreate(&model.MappingRule{Group: "Language", Raw: k, Target: v})
	}

	// 黑名单：涵盖大类名、通用描述、质量标识、采集站特殊占位符
	bl := []string{
		"动画", "动画片", "剧集", "影片", "电影", "视频", "动漫", "综艺", "短剧",
		"普通话", "国语", "粤语", "英语", "日语", "韩语", "泰语", "法语",
		"高清", "蓝光", "1080P", "4K", "HD", "BD", "TS", "TC", "DVD", "VCD",
		"其它", "其他", "全部", "剧情", "暂无", "简介", "正片", "完结", "更新中", "全集", "中字", "字幕",
		"国产", "日本", "韩国", "日韩", "欧美", "香港", "台湾", "泰国", "海外", "大陆", "内陆",
		"资源", "播放", "线路", "免费", "高速", "极速", "云播", "网盘", "在线",
	}
	for _, word := range bl {
		db.Mdb.FirstOrCreate(&model.MappingRule{Group: "Blacklist", Raw: word})
	}
}

func GetAreaMapping() map[string]string {
	res := make(map[string]string)
	cacheAreaMap.Range(func(k, v interface{}) bool {
		res[k.(string)] = v.(string)
		return true
	})
	return res
}

func GetLangMapping() map[string]string {
	res := make(map[string]string)
	cacheLangMap.Range(func(k, v interface{}) bool {
		res[k.(string)] = v.(string)
		return true
	})
	return res
}

func GetBlacklist() []string {
	res := make([]string, 0)
	cacheBlackList.Range(func(k, v interface{}) bool {
		res = append(res, k.(string))
		return true
	})
	return res
}
