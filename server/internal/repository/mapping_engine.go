package repository

import (
	"regexp"
	"server/internal/infra/db"
	"server/internal/model"
	"strings"
)

// StandardMapping 现在已迁移至数据库驱动 (mapping_rules 表)
// 内存中维护 sync.Map 以保证高性能

// GetCategoryBucketRole 根据名称推断其属于哪一个预设标准大类 (正则匹配版)
func GetCategoryBucketRole(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return ""
	}

	// 优先级正则表达式字典（从细到粗排布，避免交叉误判）
	patterns := []struct {
		Role  string
		Regex string
	}{
		{"短剧", `短剧|微短剧|爽剧|竖屏剧`},
		{"纪录片", `纪录片|记录片|纪实|专题`},
		{"动漫", `动漫|动画|番剧|二次元|BD|OVA`},
		{"综艺", `综艺|真人秀|脱口秀|晚会|访谈|晚$`},
		{"伦理片", `伦理|非法|福利|写真|X级`},
		{"电影", `电影|影片|片$|剧场版|蓝光|动作片|喜剧片|爱情片|科幻片|恐怖片|剧情片|战争片|动作|喜剧|爱情|科幻|惊悚|恐怖`},
		{"电视剧", `电视剧|连续剧|系列剧|剧集|国产剧|港剧|台剧|泰剧|美剧|韩剧|日剧|欧美剧|海外剧|剧$`},
	}

	for _, p := range patterns {
		re := regexp.MustCompile("(?i)" + p.Regex)
		if re.MatchString(typeName) {
			return p.Role
		}
	}

	return ""
}

// GetStandardIdByRole 返回标准大类的固定 ID (硬編碼確保部署即用，不受數據庫初始化順序影響)
func GetStandardIdByRole(role string) int64 {
	switch role {
	case "电影":
		return 1
	case "电视剧":
		return 2
	case "综艺":
		return 3
	case "动漫":
		return 4
	case "纪录片":
		return 5
	case "短剧":
		return 6
	case "伦理片":
		return 7
	case "其他":
		return 8
	default:
		return 0
	}
}

// GetMainCategoryIdByName 根据采集站的分类名识别标准大类 ID (無狀態確定性版本)
func GetMainCategoryIdByName(typeName string, typePid int64) int64 {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return GetStandardIdByRole("其他")
	}

	// 1. 優先級 A：通過智能分類引擎判定角色，並返回硬編碼的標準 ID
	role := GetCategoryBucketRole(typeName)
	if role != "" {
		return GetStandardIdByRole(role)
	}

	// 2. 優先級 B：根據採集站常用 Pid 推斷 (MacCMS 常用約定)
	switch typePid {
	case 1:
		return GetStandardIdByRole("电影")
	case 2:
		return GetStandardIdByRole("电视剧")
	case 3:
		return GetStandardIdByRole("综艺")
	case 4:
		return GetStandardIdByRole("动漫")
	case 37:
		return GetStandardIdByRole("短剧")
	}

	// 3. 兜底回退：歸類到標準的“其他”桶 (ID 8)
	return GetStandardIdByRole("其他")
}

func findIdByName(mains []model.Category, name string) int64 {
	for _, m := range mains {
		if m.Name == name {
			return m.Id
		}
	}
	return 0
}

// GetMainCategoryName 根据 ID 获取标准大类名称
func GetMainCategoryName(pid int64) string {
	if pid <= 0 {
		return ""
	}
	var m model.Category
	if err := db.Mdb.Where("pid = 0 AND id = ?", pid).First(&m).Error; err != nil {
		return ""
	}
	return m.Name
}

// NormalizeArea 标准化地区名称
func NormalizeArea(rawArea string) string {
	if rawArea == "" {
		return "其他"
	}
	// 多地区处理 (支持全部标准化并保留)
	rawArea = regexp.MustCompile(`[/,，、\s\.\+\|]`).ReplaceAllString(rawArea, ",")
	areas := strings.Split(rawArea, ",")
	var result []string
	seen := make(map[string]bool)

	mapping := GetAreaMapping()
	blacklist := GetBlacklist()

	for _, a := range areas {
		a = strings.TrimSpace(a)
		if a == "" || a == "其他" || a == "其它" {
			continue
		}

		// 黑名单过滤
		isBlack := false
		for _, b := range blacklist {
			if b != "" && (a == b || strings.Contains(a, b)) {
				isBlack = true
				break
			}
		}
		if isBlack {
			continue
		}

		if mapped, ok := mapping[a]; ok {
			a = mapped
		}
		if !seen[a] {
			result = append(result, a)
			seen[a] = true
		}
	}

	if len(result) == 0 {
		return "其他"
	}
	return strings.Join(result, ",")
}

// NormalizeLanguage 标准化语言名称
func NormalizeLanguage(rawLang string) string {
	if rawLang == "" {
		return "其他"
	}
	rawLang = regexp.MustCompile(`[/,，、\s]`).ReplaceAllString(rawLang, ",")
	langs := strings.Split(rawLang, ",")
	var result []string
	seen := make(map[string]bool)

	mapping := GetLangMapping()
	areaMapping := GetAreaMapping()
	blacklist := GetBlacklist()

	for _, l := range langs {
		l = strings.TrimSpace(l)
		if l == "" || l == "其他" || l == "其它" || l == "普通话" {
			continue
		}

		// 黑名单过滤
		isBlack := false
		for _, b := range blacklist {
			if b != "" && (l == b || strings.Contains(l, b)) {
				isBlack = true
				break
			}
		}
		if isBlack {
			continue
		}

		// 维度清洗：如果语言名在地区映射里，说明这个词其实是地区，剔除
		if _, isArea := areaMapping[l]; isArea {
			continue
		}
		// 映射
		if mapped, ok := mapping[l]; ok {
			l = mapped
		}
		if !seen[l] {
			result = append(result, l)
			seen[l] = true
		}
	}

	if len(result) == 0 {
		return "其他"
	}
	return strings.Join(result, ",")
}

// MapAttributesFromTypeName 从分类名中提取隐含的属性 (如 "国产剧" -> "剧集" + "大陆")
func MapAttributesFromTypeName(typeName string) (cleanTypeName string, area string) {
	cleanTypeName = typeName
	// 常见的地区词（包含在分类名中）
	keywords := map[string]string{
		"国产": "大陆", "大陆": "大陆", "内地": "大陆",
		"香港": "香港", "港片": "香港", "港剧": "香港",
		"台湾": "台湾", "台剧": "台湾",
		"欧美": "美国", "美剧": "美国",
		"韩国": "韩国", "韩剧": "韩国",
		"日本": "日本", "日剧": "日本", "日漫": "日本",
		"泰国": "泰国", "泰剧": "泰国",
		"海外": "其他",
	}

	for k, v := range keywords {
		if strings.Contains(typeName, k) {
			area = v
			// 剥离地区词，使分类名更纯粹 (如 "国产动漫" -> "动漫")
			// 注意：这里使用 Replacer 避免破坏词根
			replacer := strings.NewReplacer(
				"国产", "", "内地", "", "大陆", "",
				"韩国", "", "韩剧", "剧集",
				"日本", "", "日剧", "剧集", "日漫", "动漫",
				"欧美", "", "美剧", "剧集",
				"香港", "", "港片", "电影", "港剧", "剧集",
				"台湾", "", "台剧", "剧集",
				"泰国", "", "泰剧", "剧集",
			)
			cleanTypeName = replacer.Replace(typeName)
			// 特殊处理：如果剥离后剩下了通用的后缀（如“片”、“播”、“资源”），进一步精简
			cleanTypeName = strings.TrimSuffix(cleanTypeName, "片")
			cleanTypeName = strings.TrimSuffix(cleanTypeName, "播")
			cleanTypeName = strings.TrimSuffix(cleanTypeName, "资源")
			break
		}
	}

	cleanTypeName = strings.TrimSpace(cleanTypeName)
	if cleanTypeName == "" {
		cleanTypeName = typeName // 无法剥离则保留原样
	}
	return
}

// CleanPlotTags 用于清洗“剧情”标签，去除冗余词汇，并拆解胶水标签，确保维度纯净
func CleanPlotTags(tags string, area string, mainCategory string, category string) string {
	if tags == "" {
		return ""
	}

	// 1. 初始化过滤器与关键字列表
	blackList := GetBlacklist()
	keywords := []string{
		"动作", "喜剧", "爱情", "科幻", "悬疑", "惊悚", "恐怖", "奇幻", "冒险", "战争",
		"犯罪", "动画", "纪录", "剧情", "伦理", "传记", "历史", "古装", "武侠", "西部",
		"玄幻", "魔幻", "都市", "言情", "热血", "搞笑", "穿越", "职场", "励志", "校园",
		"竞技", "运动", "励志", "生活", "歌舞", "传记", "末日", "恐怖", "神怪", "少女", "少儿",
	}

	// 2. 预处理分隔符
	tags = regexp.MustCompile(`[/,，、\s\|+]`).ReplaceAllString(tags, ",")
	parts := strings.Split(tags, ",")

	var res []string
	seen := make(map[string]bool)

	// 获取地区简化词
	shortArea := area
	if strings.Contains(area, "中国") {
		shortArea = strings.ReplaceAll(area, "中国", "")
	}

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == category || p == "其他" || p == "其它" {
			continue
		}

		// A. 处理“胶水标签”（如“动作动画奇幻”）
		pLen := len([]rune(p))
		if pLen >= 3 && pLen <= 6 {
			foundGlue := false
			matchCount := 0
			for _, kw := range keywords {
				if strings.Contains(p, kw) {
					matchCount++
					if p != kw {
						foundGlue = true
					}
				}
			}
			if foundGlue && matchCount >= 1 {
				tempP := p
				for _, kw := range keywords {
					if strings.Contains(tempP, kw) {
						if !seen[kw] && !IsInSlice(blackList, kw) && kw != category && kw != mainCategory {
							res = append(res, kw)
							seen[kw] = true
						}
						tempP = strings.ReplaceAll(tempP, kw, "")
					}
				}
				continue
			}
		}

		// B. 剥离冗余后缀
		p = strings.TrimSuffix(p, "片")
		p = strings.TrimSuffix(p, "剧")
		p = strings.TrimSuffix(p, "类")
		p = strings.TrimSuffix(p, "题材")

		// C. 拦截黑名单与维度偏移词
		isBlack := IsInSlice(blackList, p)
		if !isBlack && mainCategory != "" {
			if strings.Contains(p, mainCategory) || (mainCategory == "动漫" && p == "动画") {
				isBlack = true
			}
		}
		if !isBlack && (strings.Contains(p, area) || (shortArea != "" && strings.Contains(p, shortArea))) {
			isBlack = true
		}

		if isBlack || len([]rune(p)) > 4 || len([]rune(p)) < 2 {
			continue
		}

		if !seen[p] {
			res = append(res, p)
			seen[p] = true
		}
	}

	return strings.Join(res, ",")
}

// IsInSlice 检查字符串是否在切片中
func IsInSlice(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
