package SystemInit

import "server/model/system"

// TableInIt 初始化 mysql 数据库相关数据
func TableInIt() {
	// 创建 User Table
	system.CreateUserTable()
	// 初始化管理员账户
	system.InitAdminAccount()
	// 创建 Search Table
	system.CreateSearchTable()
	// 创建图片信息管理表
	system.CreateFileTable()
	// 创建采集失效记录表
	system.CreateFailureRecordTable()
	// 创建影片详情持久化表
	system.CreateMovieDetailTable()
	// 创建分类持久化表
	system.CreateCategoryTable()
	// 创建多源播放列表表
	system.CreateMoviePlaylistTable()
	// 创建待同步图片队列表
	system.CreateVirtualPictureTable()
	// 创建采集源信息表
	system.CreateFilmSourceTable()
	// 创建影片检索标签持久化表
	system.CreateSearchTagTable()
	// 创建定时任务持久化表
	system.CreateCrontabTable()
	// 创建网站基础配置表
	system.CreateSiteConfigTable()
	// 创建轮播配置表
	system.CreateBannersTable()
}
