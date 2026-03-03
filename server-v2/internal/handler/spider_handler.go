package handler

import (
	"fmt"
	"strconv"

	"server-v2/config"
	"server-v2/internal/model"
	"server-v2/internal/service"
	"server-v2/pkg/response"
	"server-v2/pkg/utils"

	"github.com/gin-gonic/gin"
)

type SpiderHandler struct{}

var SpiderHd = new(SpiderHandler)

// CollectFilm 开启ID对应的资源站的采集任务
func (h *SpiderHandler) CollectFilm(c *gin.Context) {
	id := c.DefaultQuery("id", "")
	hourStr := c.DefaultQuery("h", "0")
	if id == "" || hourStr == "0" {
		response.Failed("采集任务开启失败, 缺乏必要参数", c)
		return
	}
	hr, err := strconv.Atoi(hourStr)
	if err != nil {
		response.Failed("采集任务开启失败, 采集(时长)不符合规范", c)
		return
	}
	if err = service.SpiderSvc.StartCollect(id, hr); err != nil {
		response.Failed(fmt.Sprint("采集任务开启失败: ", err.Error()), c)
		return
	}
	response.SuccessOnlyMsg("采集任务已成功开启!!!", c)
}

// StarSpider 开启并执行采集任务
func (h *SpiderHandler) StarSpider(c *gin.Context) {
	var cp model.CollectParams
	if err := c.ShouldBindJSON(&cp); err != nil {
		response.Failed("请求参数异常!!!", c)
		return
	}
	if cp.Time == 0 {
		response.Failed("采集开启失败,采集时长不能为0", c)
		return
	}

	if cp.Batch {
		if len(cp.Ids) <= 0 {
			response.Failed("批量采集开启失败, 关联的资源站信息为空", c)
			return
		}
		if err := service.SpiderSvc.BatchCollect(cp.Time, cp.Ids); err != nil {
			response.Failed(err.Error(), c)
			return
		}
	} else {
		if len(cp.Id) <= 0 {
			response.Failed("批量采集开启失败, 资源站Id获取失败", c)
			return
		}
		if err := service.SpiderSvc.StartCollect(cp.Id, cp.Time); err != nil {
			response.Failed(fmt.Sprint("采集任务开启失败: ", err.Error()), c)
			return
		}
	}
	response.SuccessOnlyMsg("采集任务已成功开启!!!", c)
}

// ClearAllFilm 删除所有film信息
func (h *SpiderHandler) ClearAllFilm(c *gin.Context) {
	pwd := c.DefaultQuery("password", "")
	if !verifyPassword(c, pwd) {
		response.Failed("重置失败, 密钥校验失败!!!", c)
		return
	}
	service.SpiderSvc.ClearFilms()
	response.SuccessOnlyMsg("影视数据已删除!!!", c)
}

// SpiderReset 重置影视数据, 清空库存, 从零开始
func (h *SpiderHandler) SpiderReset(c *gin.Context) {
	pwd := c.DefaultQuery("password", "")
	if !verifyPassword(c, pwd) {
		response.Failed("重置失败, 密码校验失败!!!", c)
		return
	}
	service.SpiderSvc.ZeroCollect(-1)
	response.SuccessOnlyMsg("影视数据已重置, 请耐心等待采集完成!!!", c)
}

// CoverFilmClass 重置覆盖影片分类信息
func (h *SpiderHandler) CoverFilmClass(c *gin.Context) {
	if err := service.SpiderSvc.FilmClassCollect(); err != nil {
		response.Failed(err.Error(), c)
		return
	}
	response.SuccessOnlyMsg("影视分类信息重置成功, 请稍等片刻后刷新页面", c)
}

// DirectedSpider 采集指定的影片
func (h *SpiderHandler) DirectedSpider(c *gin.Context) {
	// Not Implemented
}

// SingleUpdateSpider 单一影片更新采集
func (h *SpiderHandler) SingleUpdateSpider(c *gin.Context) {
	ids := c.Query("ids")
	if ids == "" {
		response.Failed("参数异常, 资源标识ID信息缺失", c)
		return
	}
	service.SpiderSvc.SyncCollect(ids)
	response.SuccessOnlyMsg("影片更新任务已成功开启!!!", c)
}

// MasterDataStatus 查询主站是否已有采集数据
func (h *SpiderHandler) MasterDataStatus(c *gin.Context) {
	response.Success(service.SpiderSvc.HasMasterData(), "获取成功", c)
}

func verifyPassword(c *gin.Context, password string) bool {
	v, ok := c.Get(config.AuthUserClaims)
	if !ok {
		response.Failed("操作失败,登录信息异常!!!", c)
		return false
	}
	uc := v.(*utils.UserClaims)
	return service.UserSvc.VerifyUserPassword(uc.UserID, password)
}
