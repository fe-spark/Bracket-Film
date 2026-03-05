package model

import (
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	UserName string `json:"userName"` // 用户名
	Password string `json:"password"` // 密码
	Salt     string `json:"salt"`     // 盐值
	Email    string `json:"email"`    // 邮箱
	Gender   int    `json:"gender"`   // 性别
	NickName string `json:"nickName"` // 昵称
	Avatar   string `json:"avatar"`   // 头像
	Status   int    `json:"status"`   // 状态
	Reserve1 string `json:"reserve1"` // 预留字段 3
	Reserve2 string `json:"reserve2"` // 预留字段 2
	Reserve3 string `json:"reserve3"` // 预留字段 1
}

// UserInfoVo 用户信息返回对象
type UserInfoVo struct {
	Id       uint   `json:"id"`
	UserName string `json:"userName"` // 用户名
	Email    string `json:"email"`    // 邮箱
	Gender   int    `json:"gender"`   // 性别
	NickName string `json:"nickName"` // 昵称
	Avatar   string `json:"avatar"`   // 头像
	Status   int    `json:"status"`   // 状态
	IsAdmin  bool   `json:"isAdmin"`  // 是否为超级管理员
}
