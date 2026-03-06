package service

import (
	"errors"
	"server/internal/config"
	"server/internal/model"
	"server/internal/repository"
	"server/internal/utils"
)

type UserService struct{}

func NewUserService() *UserService {
	return &UserService{}
}

var UserSvc = new(UserService)

// UserLogin 用户登录
func (s *UserService) UserLogin(account, password string) (token string, err error) {
	// 根据 username 或 email 查询用户信息
	var u *model.User = repository.GetUserByNameOrEmail(account)
	// 用户信息不存在则返回提示信息
	if u == nil {
		return "", errors.New("用户信息不存在!!!")
	}
	// 校验用户信息, 执行账号密码校验逻辑
	if utils.PasswordEncrypt(password, u.Salt) != u.Password {
		return "", errors.New("用户名或密码错误")
	}
	// 密码校验成功后下发token
	token, err = utils.GenToken(u.ID, u.UserName)
	if err != nil {
		return "", err
	}
	err = repository.SaveUserToken(token, u.ID)
	return
}

// UserLogout 用户退出登录 注销
func (s *UserService) UserLogout(id uint) {
	// 通过用户ID清除Redis中的token信息
	_ = repository.ClearUserToken(id)
}

// GetUserInfo 获取用户基本信息
func (s *UserService) GetUserInfo(id uint) model.UserInfoVo {
	// 通过用户ID查询对应的用户信息
	u := repository.GetUserById(id)
	// 去除user信息中的不必要信息
	var vo = model.UserInfoVo{
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
func (s *UserService) VerifyUserPassword(id uint, password string) bool {
	// 获取当前登录的用户全部信息
	u := repository.GetUserById(id)
	// 校验密码是否正确
	return utils.PasswordEncrypt(password, u.Salt) == u.Password
}

// GetUserPage 用户分页
func (s *UserService) GetUserPage(current, pageSize int, userName string) (int64, []model.UserInfoVo) {
	total, list := repository.GetUserPage(current, pageSize, userName)
	var voList []model.UserInfoVo
	for _, u := range list {
		voList = append(voList, model.UserInfoVo{
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
func (s *UserService) AddUser(u model.User) error {
	// 检查用户名是否重复
	if exist := repository.GetUserByNameOrEmail(u.UserName); exist != nil {
		return errors.New("用户名已存在")
	}
	// 密码加密
	u.Salt = utils.GenerateSalt()
	u.Password = utils.PasswordEncrypt(u.Password, u.Salt)
	return repository.AddUser(&u)
}

// UpdateUser 更新用户
func (s *UserService) UpdateUser(u model.User) error {
	// 超级管理员保护：禁止禁用默认用户
	if u.ID == config.UserIdInitialVal {
		u.Status = 0 // 强制设为正常状态
	}
	// 如果修改了密码，需要重新加密
	if u.Password != "" {
		// 先获取原用户信息拿到盐值
		oldUser := repository.GetUserById(u.ID)
		if oldUser.ID == 0 {
			return errors.New("用户不存在")
		}
		u.Salt = oldUser.Salt
		u.Password = utils.PasswordEncrypt(u.Password, u.Salt)
	}
	repository.UpdateUserInfo(u)
	// 如果用户被禁用（Status == 1），强制清除其登录状态
	if u.Status == 1 {
		_ = repository.ClearUserToken(u.ID)
	}
	return nil
}

// DeleteUser 删除用户
func (s *UserService) DeleteUser(id uint) error {
	// 超级管理员保护：禁止删除默认用户
	if id == config.UserIdInitialVal {
		return errors.New("默认超级管理员不可删除")
	}
	err := repository.DeleteUser(id)
	if err == nil {
		// 删除成功后，强制清除该用户的登录状态
		_ = repository.ClearUserToken(id)
	}
	return err
}
