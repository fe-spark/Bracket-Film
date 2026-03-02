package controller

import (
	"log"
	"server/config"
	"server/logic"
	"server/model/system"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Login 管理员登录接口
func Login(c *gin.Context) {
	var u system.User
	if err := c.ShouldBindJSON(&u); err != nil {
		system.Failed("登录信息异常!!!", c)
		return
	}
	if len(u.UserName) <= 0 || len(u.Password) <= 0 {
		system.Failed("用户名和密码信息不能为空", c)
		return
	}
	token, err := logic.UL.UserLogin(u.UserName, u.Password)
	if err != nil {
		system.Failed(err.Error(), c)
		return
	}
	c.Header("new-token", token)
	system.SuccessOnlyMsg("登录成功!!!", c)
}

// Logout 退出登录
func Logout(c *gin.Context) {
	// 获取已登录的用户信息
	v, ok := c.Get(config.AuthUserClaims)
	if !ok {
		system.Failed("请求失败,登录信息获取异常!!!", c)
		return
	}
	// 清除redis中存储的对应token
	uc, ok := v.(*system.UserClaims)
	if !ok {
		system.Failed("注销失败, 身份信息格式化异常!!!", c)
		return
	}
	err := system.ClearUserToken(uc.UserID)
	if err != nil {
		log.Println("user logOut err: ", err)
	}
	system.SuccessOnlyMsg("已退出登录!!!", c)
}

func UserInfo(c *gin.Context) {
	// 从context中获取用户的相关信息
	v, ok := c.Get(config.AuthUserClaims)
	if !ok {
		system.Failed("用户信息获取失败, 未获取到用户授权信息", c)
		return
	}
	uc, ok := v.(*system.UserClaims)
	if !ok {
		system.Failed("用户信息获取失败, 户授权信息异常", c)
		return
	}
	// 通过用户ID获取用户基本信息
	info := logic.UL.GetUserInfo(uc.UserID)
	system.Success(info, "成功获取用户信息", c)
}

// UserListPage 用户列表分页
func UserListPage(c *gin.Context) {
	current, _ := strconv.Atoi(c.DefaultQuery("current", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	userName := c.DefaultQuery("userName", "")

	total, list := logic.UL.GetUserPage(current, pageSize, userName)
	system.Success(gin.H{
		"list":  list,
		"total": total,
	}, "用户列表获取成功", c)
}

// UserAdd 添加用户
func UserAdd(c *gin.Context) {
	var u system.User
	if err := c.ShouldBindJSON(&u); err != nil {
		system.Failed("参数校验失败!!!", c)
		return
	}
	if u.UserName == "" || u.Password == "" {
		system.Failed("用户名和密码必填!!!", c)
		return
	}
	if err := logic.UL.AddUser(u); err != nil {
		system.Failed(err.Error(), c)
		return
	}
	system.SuccessOnlyMsg("用户添加成功", c)
}

// UserUpdate 更新用户
func UserUpdate(c *gin.Context) {
	var u system.User
	if err := c.ShouldBindJSON(&u); err != nil {
		system.Failed("参数校验失败!!!", c)
		return
	}
	if u.ID == 0 {
		system.Failed("用户ID缺失!!!", c)
		return
	}
	if err := logic.UL.UpdateUser(u); err != nil {
		system.Failed(err.Error(), c)
		return
	}
	system.SuccessOnlyMsg("用户信息更新成功", c)
}

// UserDelete 删除用户
func UserDelete(c *gin.Context) {
	idStr := c.DefaultQuery("id", "")
	if idStr == "" {
		system.Failed("用户ID缺失!!!", c)
		return
	}
	id, _ := strconv.Atoi(idStr)
	// 不允许删除管理员账号 (或者是当前登录账号，这里先简单按ID排除1)
	if id == 1 {
		system.Failed("初始管理员账号不允许删除!!!", c)
		return
	}
	if err := logic.UL.DeleteUser(uint(id)); err != nil {
		system.Failed(err.Error(), c)
		return
	}
	system.SuccessOnlyMsg("用户删除成功", c)
}
