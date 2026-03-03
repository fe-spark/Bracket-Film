package handler

import (
	"strconv"

	"server-v2/config"
	"server-v2/internal/model"
	"server-v2/internal/service"
	"server-v2/pkg/response"
	"server-v2/pkg/utils"

	"github.com/gin-gonic/gin"
)

type UserHandler struct{}

var UserHd = new(UserHandler)

// Login 管理员登录接口
func (h *UserHandler) Login(c *gin.Context) {
	var u model.User
	if err := c.ShouldBindJSON(&u); err != nil {
		response.Failed("登录信息异常!!!", c)
		return
	}
	if len(u.UserName) <= 0 || len(u.Password) <= 0 {
		response.Failed("用户名和密码信息不能为空", c)
		return
	}
	token, err := service.UserSvc.UserLogin(u.UserName, u.Password)
	if err != nil {
		response.Failed(err.Error(), c)
		return
	}
	c.Header("new-token", token)
	response.SuccessOnlyMsg("登录成功!!!", c)
}

// Logout 退出登录
func (h *UserHandler) Logout(c *gin.Context) {
	v, ok := c.Get(config.AuthUserClaims)
	if !ok {
		response.Failed("请求失败,登录信息获取异常!!!", c)
		return
	}
	uc, ok := v.(*utils.UserClaims)
	if !ok {
		response.Failed("注销失败, 身份信息格式化异常!!!", c)
		return
	}
	service.UserSvc.UserLogout(uc.UserID)
	response.SuccessOnlyMsg("已退出登录!!!", c)
}

// UserInfo 获取用户信息
func (h *UserHandler) UserInfo(c *gin.Context) {
	v, ok := c.Get(config.AuthUserClaims)
	if !ok {
		response.Failed("用户信息获取失败, 未获取到用户授权信息", c)
		return
	}
	uc, ok := v.(*utils.UserClaims)
	if !ok {
		response.Failed("用户信息获取失败, 户授权信息异常", c)
		return
	}
	info := service.UserSvc.GetUserInfo(uc.UserID)
	response.Success(info, "成功获取用户信息", c)
}

// UserListPage 用户列表分页
func (h *UserHandler) UserListPage(c *gin.Context) {
	current, _ := strconv.Atoi(c.DefaultQuery("current", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	userName := c.DefaultQuery("userName", "")

	total, list := service.UserSvc.GetUserPage(current, pageSize, userName)
	response.Success(gin.H{
		"list":  list,
		"total": total,
	}, "用户列表获取成功", c)
}

// UserAdd 添加用户
func (h *UserHandler) UserAdd(c *gin.Context) {
	var u model.User
	if err := c.ShouldBindJSON(&u); err != nil {
		response.Failed("参数校验失败!!!", c)
		return
	}
	if u.UserName == "" || u.Password == "" {
		response.Failed("用户名和密码必填!!!", c)
		return
	}
	if err := service.UserSvc.AddUser(u); err != nil {
		response.Failed(err.Error(), c)
		return
	}
	response.SuccessOnlyMsg("用户添加成功", c)
}

// UserUpdate 更新用户
func (h *UserHandler) UserUpdate(c *gin.Context) {
	var u model.User
	if err := c.ShouldBindJSON(&u); err != nil {
		response.Failed("参数校验失败!!!", c)
		return
	}
	if u.ID == 0 {
		response.Failed("用户ID缺失!!!", c)
		return
	}
	if err := service.UserSvc.UpdateUser(u); err != nil {
		response.Failed(err.Error(), c)
		return
	}
	response.SuccessOnlyMsg("用户信息更新成功", c)
}

// UserDelete 删除用户
func (h *UserHandler) UserDelete(c *gin.Context) {
	v, ok := c.Get(config.AuthUserClaims)
	if !ok {
		response.Failed("鉴权失败，请重新登录", c)
		return
	}
	uc, _ := v.(*utils.UserClaims)
	if uc.UserID != config.UserIdInitialVal {
		response.Failed("权限不足，仅超级管理员可删除用户", c)
		return
	}

	idStr := c.DefaultQuery("id", "")
	if idStr == "" {
		response.Failed("用户ID缺失!!!", c)
		return
	}
	id, _ := strconv.Atoi(idStr)
	if uint(id) == config.UserIdInitialVal {
		response.Failed("默认超级管理员账号不允许删除!!!", c)
		return
	}
	if err := service.UserSvc.DeleteUser(uint(id)); err != nil {
		response.Failed(err.Error(), c)
		return
	}
	response.SuccessOnlyMsg("用户删除成功", c)
}
