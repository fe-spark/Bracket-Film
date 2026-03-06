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

	// 2. 识别核心大类 (优先精准匹配，排除资讯类)
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

	rootIds := make(map[string]int64)
	// 第一遍：尝试精准匹配
	for _, c := range list {
		lowName := strings.ToLower(c.Name)
		for key, rule := range rules {
			for _, k := range rule.kw {
				if lowName == k {
					rootIds[key] = c.ID
					break
				}
			}
		}
	}
	// 第二遍：模糊匹配尚未识别的大类
	for _, c := range list {
		lowName := strings.ToLower(c.Name)
		for key, rule := range rules {
			if _, ok := rootIds[key]; !ok {
				if utils.ContainsAny(lowName, rule.kw) && !utils.ContainsAny(lowName, rule.exclude) {
					rootIds[key] = c.ID
					break
				}
			}
		}
	}

	// 3. 建立父子关系 (智能归位)
	// 顺序与规则优化：
	// - 先识别强特征分类（如 电影、电视剧）
	// - 后识别弱特征分类（如 爽剧、体育）
	subRules := []struct {
		key string
		kws []string
	}{
		{"anime", []string{"动漫", "动画", "新番"}},
		{"show", []string{"综艺", "访谈", "晚会"}},
		{"sports", []string{"足球", "篮球", "赛事", "斯诺克", "网球", "下注", "欧冠", "英超", "西甲", "德甲", "意甲", "法甲", "中超", "NBA", "CBA", "LPL", "WCBA", "竞技"}},
		{"doc", []string{"纪录", "记录"}},
		{"short", []string{"短剧", "爽剧", "重生", "穿越", "总裁", "都市", "虐恋", "逆袭", "甜宠", "短片"}},
		{"movie", []string{"片"}},
		{"tv", []string{"剧"}},
		{"other", []string{"伦理", "三级", "两性", "写真"}},
	}

	for _, c := range list {
		id, pid, name := c.ID, c.Pid, c.Name
		lowName := strings.ToLower(name)

		// 自动隐藏及过滤掉非影视资源的分类 (明星、资讯、解说等)
		show := true
		if utils.ContainsAny(lowName, []string{"资讯", "明星", "新闻", "解说", "站长", "教程"}) {
			show = false
		}
		nodes[id].Show = show

		if pid == 0 {
			matched := false
			// 特殊处理：如果包含“片”且不是“短片”，优先归位到电影
			if strings.Contains(lowName, "片") && !strings.Contains(lowName, "短片") {
				if rid, ok := rootIds["movie"]; ok && id != rid {
					pid = rid
					matched = true
				}
			} else if strings.Contains(lowName, "剧") && !strings.Contains(lowName, "短剧") {
				// 特殊处理：如果包含“剧”，优先归位到连续剧
				if rid, ok := rootIds["tv"]; ok && id != rid {
					pid = rid
					matched = true
				}
			}

			if !matched {
				for _, rule := range subRules {
					if utils.ContainsAny(lowName, rule.kws) {
						if rid, ok := rootIds[rule.key]; ok && id != rid {
							pid = rid
							matched = true
							break
						}
					}
				}
			}
		}
		parent := nodes[pid]
		if parent == nil {
			parent = root
		}
		parent.Children = append(parent.Children, nodes[id])
	}

	return root
}

// ConvertCategoryList 将分类树形数据转化为list类型
func ConvertCategoryList(tree model.CategoryTree) []model.Category {
	cl := []model.Category{{Id: tree.Id, Pid: tree.Pid, Name: tree.Name, Show: tree.Show}}
	for _, c := range tree.Children {
		cl = append(cl, model.Category{Id: c.Id, Pid: c.Pid, Name: c.Name, Show: c.Show})
		if len(c.Children) > 0 {
			for _, subC := range c.Children {
				cl = append(cl, model.Category{Id: subC.Id, Pid: subC.Pid, Name: subC.Name, Show: subC.Show})
			}
		}
	}
	return cl
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
