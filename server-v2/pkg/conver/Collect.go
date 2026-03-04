package conver

import (
	"fmt"
	"strings"

	"server-v2/config"
	"server-v2/internal/model"
	"server-v2/internal/model/collect"
)

/*
	处理 不同结构体数据之间的转化
	统一转化为内部结构体
*/

// GenCategoryTree 解析处理 filmListPage数据 生成分类树形数据
func GenCategoryTree(list []collect.FilmClass) *model.CategoryTree {
	// 遍历所有分类进行树形结构组装
	tree := &model.CategoryTree{Category: &model.Category{Id: 0, Pid: -1, Name: "分类信息", Show: true}}
	temp := make(map[int64]*model.CategoryTree)
	temp[tree.Id] = tree
	for _, c := range list {
		// 判断当前节点ID是否存在于 temp中
		category, ok := temp[c.TypeID]
		if ok {
			// 将当前节点信息保存
			category.Category = &model.Category{Id: c.TypeID, Pid: c.TypePid, Name: c.TypeName, Show: true}
		} else {
			// 如果不存在则将当前分类存放到 temp中
			category = &model.CategoryTree{Category: &model.Category{Id: c.TypeID, Pid: c.TypePid, Name: c.TypeName, Show: true}}
			temp[c.TypeID] = category
		}
		// 根据 pid获取父节点信息
		parent, ok := temp[category.Pid]
		if !ok {
			// 如果不存在父节点存在, 则将父节点存放到temp中
			temp[c.TypePid] = parent
		}
		// 将当前节点存放到父节点的Children中
		parent.Children = append(parent.Children, category)
	}

	return tree
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
func ConvertFilmDetails(details []collect.FilmDetail) []model.MovieDetail {
	var dl []model.MovieDetail
	for _, d := range details {
		dl = append(dl, ConvertFilmDetail(d))
	}
	return dl
}

// ConvertFilmDetail 将影片详情数据处理转化为 model.MovieDetail
func ConvertFilmDetail(detail collect.FilmDetail) model.MovieDetail {
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
func DetailCovertList(details []collect.FilmDetail) []collect.FilmList {
	var l []collect.FilmList
	for _, d := range details {
		fl := collect.FilmList{
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
func DetailCovertXml(details []collect.FilmDetail) []collect.VideoDetail {
	var vl []collect.VideoDetail
	for _, d := range details {
		vl = append(vl, collect.VideoDetail{
			Last:     d.VodTime,
			ID:       d.VodID,
			Tid:      d.TypeID,
			Name:     collect.CDATA{Text: d.VodName},
			Type:     d.TypeName,
			Pic:      d.VodPic,
			Lang:     d.VodLang,
			Area:     d.VodArea,
			Year:     d.VodYear,
			State:    d.VodState,
			Note:     collect.CDATA{Text: d.VodRemarks},
			Actor:    collect.CDATA{Text: d.VodActor},
			Director: collect.CDATA{Text: d.VodDirector},
			DL:       collect.DL{DD: []collect.DD{{Flag: d.VodPlayFrom, Value: d.VodPlayURL}}},
			Des:      collect.CDATA{Text: d.VodContent},
		})
	}
	return vl
}

// DetailCovertListXml 将影片详情信息转化为Xml格式FilmList的对象
func DetailCovertListXml(details []collect.FilmDetail) []collect.VideoList {
	var vl []collect.VideoList
	for _, d := range details {
		vl = append(vl, collect.VideoList{
			Last: d.VodTime,
			ID:   d.VodID,
			Tid:  d.TypeID,
			Name: collect.CDATA{Text: d.VodName},
			Type: d.TypeName,
			Dt:   d.VodPlayFrom,
			Note: collect.CDATA{Text: d.VodRemarks},
		})
	}
	return vl
}

// ClassListCovertXml 将影片分类列表转化为XML格式
func ClassListCovertXml(cl []collect.FilmClass) collect.ClassXL {
	var l collect.ClassXL
	for _, c := range cl {
		l.ClassX = append(l.ClassX, collect.ClassX{ID: c.TypeID, Value: c.TypeName})
	}
	return l
}

// FilterFilmDetail 对影片详情数据进行处理, t 修饰类型 0-返回m3u8,mp4 | 1 返回 云播链接 | 2 返回全部
func FilterFilmDetail(fd collect.FilmDetail, t int64) collect.FilmDetail {
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
