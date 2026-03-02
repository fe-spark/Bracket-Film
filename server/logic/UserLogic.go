package logic

import (
	"errors"
	"server/config"
	"server/model/system"
	"server/plugin/common/util"
)

type UserLogic struct {
}

var UL *UserLogic

// UserLogin 用户登录
func (ul *UserLogic) UserLogin(account, password string) (token string, err error) {
	// 根据 username 或 email 查询用户信息
	var u *system.User = system.GetUserByNameOrEmail(account)
	// 用户信息不存在则返回提示信息
	if u == nil {
		return "", errors.New(" 用户信息不存在!!!")
	}
	// 校验用户信息, 执行账号密码校验逻辑
	if util.PasswordEncrypt(password, u.Salt) != u.Password {
		return "", errors.New("用户名或密码错误")
	}
	// 密码校验成功后下发token
	token, err = system.GenToken(u.ID, u.UserName)
	err = system.SaveUserToken(token, u.ID)
	return
}

// UserLogout 用户退出登录 注销
func (ul *UserLogic) UserLogout() {
	// 通过用户ID清除Redis中的token信息

}

// GetUserInfo 获取用户基本信息
func (ul *UserLogic) GetUserInfo(id uint) system.UserInfoVo {
	// 通过用户ID查询对应的用户信息
	u := system.GetUserById(id)
	// 去除user信息中的不必要信息
	var vo = system.UserInfoVo{
		Id:       u.ID,
		UserName: u.UserName,
		Email:    u.Email,
		Gender:   u.Gender,
		NickName: u.NickName,
		Avatar:   u.Avatar,
		Status:   u.Status,
		IsAdmin:  u.ID == config.UserIdInitialVal,
	}
	return vo
}

// VerifyUserPassword 校验密码
func (ul *UserLogic) VerifyUserPassword(id uint, password string) bool {
	// 获取当前登录的用户全部信息
	u := system.GetUserById(id)
	// 校验密码是否正确
	return util.PasswordEncrypt(password, u.Salt) == u.Password
}

// GetUserPage 用户分页
func (ul *UserLogic) GetUserPage(current, pageSize int, userName string) (int64, []system.UserInfoVo) {
	total, list := system.GetUserPage(current, pageSize, userName)
	var voList []system.UserInfoVo
	for _, u := range list {
		voList = append(voList, system.UserInfoVo{
			Id:       u.ID,
			UserName: u.UserName,
			Email:    u.Email,
			Gender:   u.Gender,
			NickName: u.NickName,
			Avatar:   u.Avatar,
			Status:   u.Status,
			IsAdmin:  u.ID == config.UserIdInitialVal,
		})
	}
	return total, voList
}

// AddUser 添加用户
func (ul *UserLogic) AddUser(u system.User) error {
	// 检查用户名是否重复
	if exist := system.GetUserByNameOrEmail(u.UserName); exist != nil {
		return errors.New("用户名已存在")
	}
	// 密码加密
	u.Salt = util.GenerateSalt()
	u.Password = util.PasswordEncrypt(u.Password, u.Salt)
	return system.AddUser(&u)
}

// UpdateUser 更新用户
func (ul *UserLogic) UpdateUser(u system.User) error {
	// 超级管理员保护：禁止禁用默认用户
	if u.ID == config.UserIdInitialVal {
		u.Status = 0 // 强制设为正常状态
	}
	// 如果修改了密码，需要重新加密
	if u.Password != "" {
		// 先获取原用户信息拿到盐值
		oldUser := system.GetUserById(u.ID)
		if oldUser.ID == 0 {
			return errors.New("用户不存在")
		}
		u.Salt = oldUser.Salt
		u.Password = util.PasswordEncrypt(u.Password, u.Salt)
	}
	system.UpdateUserInfo(u)
	// 如果用户被禁用（Status == 1），强制清除其登录状态
	if u.Status == 1 {
		_ = system.ClearUserToken(u.ID)
	}
	return nil
}

// DeleteUser 删除用户
func (ul *UserLogic) DeleteUser(id uint) error {
	// 超级管理员保护：禁止删除默认用户
	if id == config.UserIdInitialVal {
		return errors.New("默认超级管理员不可删除")
	}
	err := system.DeleteUser(id)
	if err == nil {
		// 删除成功后，强制清除该用户的登录状态
		_ = system.ClearUserToken(id)
	}
	return err
}
