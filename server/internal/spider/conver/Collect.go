package conver

import (
	"fmt"
	"strings"

	"server/internal/config"
	"server/internal/model"
	"server/internal/utils"
)

/*
	处理 不同结构体数据之间的转化
	统一转化为内部结构体
*/

// GenCategoryTree 解析处理 filmListPage数据 生成分类树形数据
func GenCategoryTree(list []model.FilmClass) *model.CategoryTree {
	root := &model.CategoryTree{
		Category: &model.Category{Id: 0, Pid: -1, Name: "分类信息", Show: true},
		Children: make([]*model.CategoryTree, 0),
	}
	nodes := make(map[int64]*model.CategoryTree)
	nodes[0] = root

	// 1. 初始化节点
	for _, c := range list {
		id, name := c.ID, c.Name
		nodes[id] = &model.CategoryTree{
			Category: &model.Category{Id: id, Pid: c.Pid, Name: name, Show: true},
			Children: make([]*model.CategoryTree, 0),
		}
	}

	// 2. 定义分类识别规则 (仅作为逻辑参考，不再预先创建 ID)
	rules := map[string]struct{ kw, exclude []string }{
		"movie":  {[]string{"电影", "影院"}, []string{"片", "资讯", "新闻", "站长", "解说"}},
		"tv":     {[]string{"电视剧", "剧集", "连续剧"}, []string{"短剧", "资讯", "新闻", "站长", "解说"}},
		"anime":  {[]string{"动漫", "动画", "漫剧"}, []string{"资讯", "新闻", "站长", "解说"}},
		"show":   {[]string{"综艺"}, []string{"资讯", "新闻", "站长", "解说"}},
		"sports": {[]string{"体育", "赛事"}, []string{"资讯", "新闻", "站长", "解说"}},
		"short":  {[]string{"短剧", "爽剧"}, []string{"反转", "资讯", "新闻", "站长", "解说"}},
		"doc":    {[]string{"纪录", "记录", "纪录片"}, []string{"资讯", "新闻", "站长", "解说"}},
		"other":  {[]string{"其他", "福利", "伦理", "三级"}, nil},
	}

	// 用于存储最终确定的“大类”节点与其类型 Key 的映射
	rootIds := make(map[string]int64)
	rootNames := map[string]string{
		"movie": "电影", "tv": "电视剧", "anime": "动漫", "show": "综艺",
		"sports": "体育", "short": "短剧", "doc": "纪录片", "other": "其他",
	}
	// virtualRoots 记录按需生成的虚拟根节点指针
	virtualRoots := make(map[string]*model.CategoryTree)

	// 第一遍：初步扫描识别“真根” (采集站原始已归好的类)
	for _, c := range list {
		lowName := strings.ToLower(c.Name)
		for key, rule := range rules {
			if c.Pid == 0 && utils.ContainsAny(lowName, rule.kw) && !utils.ContainsAny(lowName, rule.exclude) {
				if _, ok := rootIds[key]; !ok {
					rootIds[key] = c.ID
				}
			}
		}
	}

	// 辅助逻辑：按需获取或创建根节点
	getOrCreateRoot := func(key string) *model.CategoryTree {
		if rid, ok := rootIds[key]; ok {
			return nodes[rid]
		}
		if vr, ok := virtualRoots[key]; ok {
			return vr
		}
		// 创建无 ID 的纯逻辑根节点，入库时由存取层自动根据 Name + 树位置对齐
		vr := &model.CategoryTree{
			Category: &model.Category{Name: rootNames[key], Pid: 0, Show: true},
			Children: make([]*model.CategoryTree, 0),
		}
		virtualRoots[key] = vr
		root.Children = append(root.Children, vr)
		return vr
	}

	// 3. 建立层级关系 (智能归并)
	subRules := []struct {
		key string
		kws []string
	}{
		{"anime", []string{"动漫", "动画", "新番"}},
		{"show", []string{"综艺", "访谈", "晚会", "各色演唱会", "演唱会"}},
		{"sports", []string{"足球", "篮球", "赛事", "斯诺克", "网球", "下注", "欧冠", "英超", "西甲", "德甲", "意甲", "法甲", "中超", "nba", "cba", "lpl", "wcba", "竞技", "奥运", "亚运", "lol"}},
		{"doc", []string{"纪录", "记录", "科普", "学习"}},
		{"short", []string{"短剧", "爽剧", "重生", "穿越", "总裁", "都市", "虐恋", "逆袭", "甜宠", "短片"}},
		{"movie", []string{"电影", "影片", "片", "影院", "蓝光", "4k", "仙侠", "古装", "悬疑", "烧脑", "惊悚"}},
		{"tv", []string{"剧", "剧集", "电视剧", "连续剧"}},
		{"other", []string{"伦理", "三级", "两性", "写真"}},
	}

	for _, c := range list {
		id, pid, name := c.ID, c.Pid, c.Name
		lowName := strings.ToLower(name)

		// 忽略资讯、明星等
		nodes[id].Show = !utils.ContainsAny(lowName, []string{"资讯", "明星", "新闻", "解说", "站长", "教程"})

		if pid == 0 {
			isRoot := false
			for _, rid := range rootIds {
				if id == rid {
					isRoot = true
					break
				}
			}

			if !isRoot {
				var target *model.CategoryTree
				// 电影/电视剧优先
				if utils.ContainsAny(lowName, []string{"电影", "影片", "影院", "蓝光", "4k", "仙侠", "古装", "悬疑", "烧脑", "惊悚"}) || (strings.Contains(lowName, "片") && !strings.Contains(lowName, "短片")) {
					target = getOrCreateRoot("movie")
				} else if utils.ContainsAny(lowName, []string{"剧集", "电视剧", "连续剧"}) || (strings.Contains(lowName, "剧") && !strings.Contains(lowName, "短剧")) {
					target = getOrCreateRoot("tv")
				} else {
					for _, rule := range subRules {
						if utils.ContainsAny(lowName, rule.kws) {
							target = getOrCreateRoot(rule.key)
							break
						}
					}
				}
				if target == nil {
					target = getOrCreateRoot("other")
				}
				// 挂载并设置 Pid 语义
				target.Children = append(target.Children, nodes[id])
				// 注意：这里保留 nodes[id].Category.Pid = 0 不变，或者入库层处理，
				// 为了让保存逻辑识别它是“被重新分配了父类”，我们可以手动干预：
				// nodes[id].Category.Pid = target.Id // 如果 target 有 Id
				continue
			}
		}

		// 默认归位逻辑
		parent := nodes[pid]
		if parent == nil {
			parent = root
		}
		parent.Children = append(parent.Children, nodes[id])
	}

	return root
}

// ConvertCategoryList 将分类树形数据平滑展开为列表，支持深度嵌套
func ConvertCategoryList(tree *model.CategoryTree) []model.Category {
	var list []model.Category
	if tree == nil {
		return list
	}
	// 不保存虚拟根节点 0 本身到列表（通常数据库不需要这个占位符）
	if tree.Id != 0 {
		list = append(list, *tree.Category)
	}
	for _, child := range tree.Children {
		list = append(list, ConvertCategoryList(child)...)
	}
	return list
}

// ConvertFilmDetails 批量处理影片详情信息
func ConvertFilmDetails(details []model.FilmDetail) []model.MovieDetail {
	var dl []model.MovieDetail
	for _, d := range details {
		// 跳过片名为空的无效数据，防止数据库出现空记录
		if strings.TrimSpace(d.VodName) == "" {
			continue
		}
		dl = append(dl, ConvertFilmDetail(d))
	}
	return dl
}

// ConvertFilmDetail 将影片详情数据处理转化为 model.MovieDetail
func ConvertFilmDetail(detail model.FilmDetail) model.MovieDetail {
	md := model.MovieDetail{
		Id:       detail.VodID,
		Cid:      detail.TypeID,
		Pid:      detail.TypeID1,
		Name:     detail.VodName,
		Picture:  detail.VodPic,
		DownFrom: detail.VodDownFrom,
		MovieDescriptor: model.MovieDescriptor{
			SubTitle:    detail.VodSub,
			CName:       detail.TypeName,
			EnName:      detail.VodEn,
			Initial:     detail.VodLetter,
			ClassTag:    detail.VodClass,
			Actor:       detail.VodActor,
			Director:    detail.VodDirector,
			Writer:      detail.VodWriter,
			Blurb:       detail.VodBlurb,
			Remarks:     detail.VodRemarks,
			ReleaseDate: detail.VodPubDate,
			Area:        detail.VodArea,
			Language:    detail.VodLang,
			Year:        detail.VodYear,
			State:       detail.VodState,
			UpdateTime:  detail.VodTime,
			AddTime:     detail.VodTimeAdd,
			DbId:        detail.VodDouBanID,
			DbScore:     detail.VodDouBanScore,
			Hits:        detail.VodHits,
			Content:     detail.VodContent,
		},
	}
	// 通过分割符切分播放源信息  PlaySeparator $$$
	md.PlayFrom = strings.Split(detail.VodPlayFrom, detail.VodPlayNote)
	// v2 只保留m3u8播放源
	md.PlayList = GenFilmPlayList(detail.VodPlayURL, detail.VodPlayNote)
	md.DownloadList = GenFilmPlayList(detail.VodDownURL, detail.VodPlayNote)

	return md
}

// GenFilmPlayList 处理影片播放地址数据, 保留播放链接,生成playList
// 只 append 有效（非空）的播放列表，防止 ConvertPlayUrl("") 产生 nil inner slice → JSON [null]
func GenFilmPlayList(playUrl, separator string) [][]model.MovieUrlInfo {
	var res [][]model.MovieUrlInfo
	if separator != "" {
		// 1. 通过分隔符切分播放源地址
		for _, l := range strings.Split(playUrl, separator) {
			// 只保留解析出有效链接的播放源
			if pl := ConvertPlayUrl(l); len(pl) > 0 {
				res = append(res, pl)
			}
		}
	} else {
		if pl := ConvertPlayUrl(playUrl); len(pl) > 0 {
			res = append(res, pl)
		}
	}
	return res
}

// GenAllFilmPlayList 处理影片播放地址数据, 保留全部播放链接,生成playList
func GenAllFilmPlayList(playUrl, separator string) [][]model.MovieUrlInfo {
	var res [][]model.MovieUrlInfo
	if separator != "" {
		// 1. 通过分隔符切分播放源地址
		for _, l := range strings.Split(playUrl, separator) {
			if pl := ConvertPlayUrl(l); len(pl) > 0 {
				res = append(res, pl)
			}
		}
		return res
	}
	if pl := ConvertPlayUrl(playUrl); len(pl) > 0 {
		res = append(res, pl)
	}
	return res
}

// parseEpisode 从单个片段解析集数名和播放链接，支持以下格式：
//
//	"集名$URL"  → episode=集名, link=URL
//	"URL"       → episode="",  link=URL  (无集名，调用方自动补全)
//	"$URL"      → episode="",  link=URL  (部分采集站数据以 $ 开头)
//	"集名$"     → ok=false              (link 缺失，无效)
func parseEpisode(seg string) (episode, link string, ok bool) {
	ep, lk, hasDollar := strings.Cut(seg, "$")
	ep, lk = strings.TrimSpace(ep), strings.TrimSpace(lk)
	switch {
	case !hasDollar:
		return "", ep, ep != "" // 整条是 URL
	case lk != "":
		return ep, lk, true // 正常 "集名$URL"
	case strings.HasPrefix(ep, "http"):
		return "", ep, true // "$URL" 形式，ep 实为 URL
	default:
		return "", "", false // "集名$"，link 为空
	}
}

// isVideoURL 判断是否为视频直链，过滤 share/ 等网页链接
func isVideoURL(link string) bool {
	lower := strings.ToLower(link)
	return strings.Contains(lower, ".m3u8") ||
		strings.Contains(lower, ".mp4") ||
		strings.Contains(lower, ".flv")
}

// ConvertPlayUrl 将单条 playFrom 地址字符串解析为播放列表
// 片段格式：集名$URL，多集以 # 分隔
func ConvertPlayUrl(playUrl string) []model.MovieUrlInfo {
	var result []model.MovieUrlInfo
	for _, seg := range strings.Split(playUrl, "#") {
		episode, link, ok := parseEpisode(strings.TrimSpace(seg))
		if !ok || !isVideoURL(link) {
			continue
		}
		if episode == "" {
			episode = fmt.Sprintf("第%d集", len(result)+1)
		}
		result = append(result, model.MovieUrlInfo{Episode: episode, Link: link})
	}
	return result
}

// ConvertVirtualPicture 将影片详情信息转化为虚拟图片信息
func ConvertVirtualPicture(details []model.MovieDetail) []model.VirtualPicture {
	var l []model.VirtualPicture
	for _, d := range details {
		if len(d.Picture) > 0 {
			l = append(l, model.VirtualPicture{Id: d.Id, Link: d.Picture})
		}
	}
	return l
}

// ----------------------------------Provide API---------------------------------------------------

// DetailCovertList 将影视详情信息转化为列表信息
func DetailCovertList(details []model.FilmDetail) []model.FilmList {
	var l []model.FilmList
	for _, d := range details {
		fl := model.FilmList{
			VodID:       d.VodID,
			VodName:     d.VodName,
			TypeID:      d.TypeID,
			TypeName:    d.TypeName,
			VodEn:       d.VodEn,
			VodTime:     d.VodTime,
			VodRemarks:  d.VodRemarks,
			VodPlayFrom: d.VodPlayFrom,
		}
		l = append(l, fl)
	}
	return l
}

// DetailCovertXml 将影片详情信息转化为Xml格式的对象
func DetailCovertXml(details []model.FilmDetail) []model.VideoDetail {
	var vl []model.VideoDetail
	for _, d := range details {
		vl = append(vl, model.VideoDetail{
			Last:     d.VodTime,
			ID:       d.VodID,
			Tid:      d.TypeID,
			Name:     model.CDATA{Text: d.VodName},
			Type:     d.TypeName,
			Pic:      d.VodPic,
			Lang:     d.VodLang,
			Area:     d.VodArea,
			Year:     d.VodYear,
			State:    d.VodState,
			Note:     model.CDATA{Text: d.VodRemarks},
			Actor:    model.CDATA{Text: d.VodActor},
			Director: model.CDATA{Text: d.VodDirector},
			DL:       model.DL{DD: []model.DD{{Flag: d.VodPlayFrom, Value: d.VodPlayURL}}},
			Des:      model.CDATA{Text: d.VodContent},
		})
	}
	return vl
}

// DetailCovertListXml 将影片详情信息转化为Xml格式FilmList的对象
func DetailCovertListXml(details []model.FilmDetail) []model.VideoList {
	var vl []model.VideoList
	for _, d := range details {
		vl = append(vl, model.VideoList{
			Last: d.VodTime,
			ID:   d.VodID,
			Tid:  d.TypeID,
			Name: model.CDATA{Text: d.VodName},
			Type: d.TypeName,
			Dt:   d.VodPlayFrom,
			Note: model.CDATA{Text: d.VodRemarks},
		})
	}
	return vl
}

// ClassListCovertXml 将影片分类列表转化为XML格式
func ClassListCovertXml(cl []model.FilmClass) model.ClassXL {
	var l model.ClassXL
	for _, c := range cl {
		l.ClassX = append(l.ClassX, model.ClassX{ID: c.ID, Value: c.Name})
	}
	return l
}

// FilterFilmDetail 对影片详情数据进行处理, t 修饰类型 0-返回m3u8,mp4 | 1 返回 云播链接 | 2 返回全部
func FilterFilmDetail(fd model.FilmDetail, t int64) model.FilmDetail {
	// 只保留 mu38 | mp4 格式的播放源, 如果包含多种格式的播放数据
	if strings.Contains(fd.VodPlayURL, fd.VodPlayNote) {
		switch t {
		case 2:
			fd.VodPlayFrom = config.PlayFormAll
		case 1, 0:
			for _, v := range strings.Split(fd.VodPlayURL, fd.VodPlayNote) {
				if t == 0 && (strings.Contains(v, ".m3u8") || strings.Contains(v, ".mp4")) {
					fd.VodPlayFrom = config.PlayForm
					fd.VodPlayURL = v
				} else if t == 1 && !strings.Contains(v, ".m3u8") && !strings.Contains(v, ".mp4") {
					fd.VodPlayFrom = config.PlayFormCloud
					fd.VodPlayURL = v
				}
			}

		}
	} else {
		// 如果只有一种类型的播放链,则默认为m3u8  修改 VodPlayFrom 信息
		fd.VodPlayFrom = config.PlayForm
	}

	return fd
}
