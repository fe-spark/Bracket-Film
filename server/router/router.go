package router

import (
	"server/config"
	"server/controller"
	"server/plugin/middleware"

	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {

	r := gin.Default()
	// 开启跨域
	r.Use(middleware.Cors())

	// 静态资源配置
	r.Static(config.FilmPictureUrlPath, config.FilmPictureUploadDir)

	r.GET(`/index`, controller.Index)
	r.GET(`/proxy/video`, controller.ProxyVideo)
	r.GET(`/cache/del`, controller.IndexCacheDel)
	r.GET(`/config/basic`, controller.SiteBasicConfig)
	r.GET(`/navCategory`, controller.CategoriesInfo)
	r.GET(`/filmDetail`, controller.FilmDetail)
	r.GET(`/filmPlayInfo`, controller.FilmPlayInfo)
	r.GET(`/searchFilm`, controller.SearchFilm)
	r.GET(`/filmClassify`, controller.FilmClassify)
	r.GET(`/filmClassifySearch`, controller.FilmTagSearch)
	//r.GET(`/filmCategory`, controller.FilmCategory) 弃用
	r.POST(`/login`, controller.Login)
	r.GET(`/logout`, middleware.AuthToken(), controller.Logout)

	// 管理员API路由组
	manageRoute := r.Group(`/manage`)
	manageRoute.Use(middleware.AuthToken())
	{
		manageRoute.GET(`/index`, controller.ManageIndex)

		// 系统相关
		sysConfig := manageRoute.Group(`/config`)
		{
			sysConfig.GET(`/basic`, controller.SiteBasicConfig)
			sysConfig.POST(`/basic/update`, controller.UpdateSiteBasic)
			sysConfig.GET(`/basic/reset`, controller.ResetSiteBasic)
		}

		// 轮播相关
		banner := manageRoute.Group(`banner`)
		{
			banner.GET(`/list`, controller.BannerList)
			banner.GET(`/find`, controller.BannerFind)
			banner.POST(`/add`, controller.BannerAdd)
			banner.POST(`/update`, controller.BannerUpdate)
			banner.GET(`/del`, controller.BannerDel)
		}

		// 用户相关
		userRoute := manageRoute.Group(`/user`)
		{
			userRoute.GET(`/info`, controller.UserInfo)
			userRoute.GET(`/list`, controller.UserListPage)
			userRoute.POST(`/add`, controller.UserAdd)
			userRoute.POST(`/update`, controller.UserUpdate)
			userRoute.GET(`/del`, controller.UserDelete)
		}

		// 采集路相关
		collect := manageRoute.Group(`/collect`)
		{
			collect.GET(`/list`, controller.FilmSourceList)
			collect.GET(`/find`, controller.FindFilmSource)
			collect.POST(`/test`, controller.FilmSourceTest)
			collect.POST(`/add`, controller.FilmSourceAdd)
			collect.POST(`/update`, controller.FilmSourceUpdate)
			collect.POST(`/change`, controller.FilmSourceChange)
			//collect.GET(`/star`, controller.CollectFilm)
			collect.GET(`/del`, controller.FilmSourceDel)
			collect.GET(`/options`, controller.GetNormalFilmSource)
			collect.GET(`/collecting/state`, controller.CollectingState)
			collect.GET(`/stop`, controller.StopCollect)

			collect.GET(`/record/list`, controller.FailureRecordList)
			collect.GET(`/record/retry`, controller.CollectRecover)
			collect.GET(`/record/retry/all`, controller.CollectRecoverAll)
			collect.GET(`/record/clear/done`, controller.ClearDoneRecord)
			collect.GET(`/record/clear/all`, controller.ClearAllRecord)

		}

		// 定时任务相关
		collectCron := manageRoute.Group(`/cron`)
		{
			collectCron.GET(`/list`, controller.FilmCronTaskList)
			collectCron.GET(`/find`, controller.GetFilmCronTask)
			//collectCron.GET(`/options`, controller.GetNormalFilmSource)
			collectCron.POST(`/add`, controller.FilmCronAdd)
			collectCron.POST(`/update`, controller.FilmCronUpdate)
			collectCron.POST(`/change`, controller.ChangeTaskState)
			collectCron.GET(`/del`, controller.DelFilmCron)
		}
		// spider 数据采集
		spiderRoute := manageRoute.Group(`/spider`)
		{
			spiderRoute.POST(`/start`, controller.StarSpider)
			spiderRoute.GET(`/zero`, controller.SpiderReset)
			spiderRoute.GET(`/clear`, controller.ClearAllFilm)
			spiderRoute.GET(`/update/single`, controller.SingleUpdateSpider)
			spiderRoute.GET(`/class/cover`, controller.CoverFilmClass)
		}
		// filmManage 影视管理
		filmRoute := manageRoute.Group(`/film`)
		{
			filmRoute.POST(`/add`, controller.FilmAdd)
			filmRoute.GET(`/search/list`, controller.FilmSearchPage)
			filmRoute.GET(`/search/del`, controller.FilmDelete)

			filmRoute.GET(`/class/tree`, controller.FilmClassTree)
			filmRoute.GET(`/class/find`, controller.FindFilmClass)
			filmRoute.POST(`/class/update`, controller.UpdateFilmClass)
			filmRoute.GET(`/class/del`, controller.DelFilmClass)
		}

		// 文件管理
		fileRoute := manageRoute.Group(`/file`)
		{
			fileRoute.POST(`/upload`, controller.SingleUpload)
			fileRoute.GET(`/upload/multiple`, controller.MultipleUpload)
			fileRoute.GET(`/del`, controller.DelFile)
			fileRoute.GET(`/list`, controller.PhotoWall)
		}

	}

	provideRoute := r.Group(`/provide`)
	{
		provideRoute.GET(`/vod`, controller.HandleProvide)          // CMS资源API
		provideRoute.GET(`/config`, controller.HandleProvideConfig) // TVBox/影视仓聚合网络配置API
	}

	return r
}
