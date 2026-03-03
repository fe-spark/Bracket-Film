package repository

import (
	"fmt"
	"log"
	"server-v2/config"
	"server-v2/internal/model"
	"server-v2/pkg/db"
	"server-v2/pkg/utils"

	"gorm.io/gorm"
)

// ExistUserTable 判断表中是否存在User表
func ExistUserTable() bool {
	return db.Mdb.Migrator().HasTable(&model.User{})
}

// CreateUserTable 创建存储检索信息的数据表
func CreateUserTable() {
	var u = &model.User{}
	// 如果不存在则创建表 并设置自增ID初始值为10000
	if !ExistUserTable() {
		err := db.Mdb.AutoMigrate(u)
		db.Mdb.Exec(fmt.Sprintf("alter table %s auto_Increment = %d", u.TableName(), config.UserIdInitialVal))
		if err != nil {
			log.Println("Create Table User Failed: ", err)
		}
	}
}

// InitAdminAccount 初始化admin用户密码
func InitAdminAccount() {
	// 先查询是否已经存在admin用户信息, 存在则直接退出
	user := GetUserByNameOrEmail("admin")
	if user != nil {
		return
	}
	// 不存在管理员用户则进行初始化创建
	u := &model.User{
		UserName: "admin",
		Password: "admin",
		Salt:     utils.GenerateSalt(),
		Email:    "administrator@gmail.com",
		Gender:   2,
		NickName: "Spark",
		Avatar:   "empty",
		Status:   0,
	}

	u.Password = utils.PasswordEncrypt(u.Password, u.Salt)
	db.Mdb.Create(u)
}

// GetUserByNameOrEmail 查询 username || email 对应的账户信息
func GetUserByNameOrEmail(userName string) *model.User {
	var u *model.User
	if err := db.Mdb.Where("user_name = ? OR email = ?", userName, userName).First(&u).Error; err != nil {
		log.Println(err)
		return nil
	}
	return u
}

// GetUserById 通过id获取对应的用户信息
func GetUserById(id uint) model.User {
	var user = model.User{Model: gorm.Model{ID: id}}
	db.Mdb.First(&user)
	return user
}

// UpdateUserInfo 更新用户信息
func UpdateUserInfo(u model.User) {
	// 值更新允许修改的部分字段, 零值会在更新时被自动忽略
	db.Mdb.Model(&u).Updates(model.User{Password: u.Password, Email: u.Email, NickName: u.NickName, Status: u.Status, Gender: u.Gender, Avatar: u.Avatar})
}

// GetUserPage 分页获取用户信息
func GetUserPage(current, pageSize int, userName string) (int64, []model.User) {
	var list []model.User
	var total int64
	query := db.Mdb.Model(&model.User{})
	if userName != "" {
		query = query.Where("user_name LIKE ?", "%"+userName+"%")
	}
	query.Count(&total)
	query.Offset((current - 1) * pageSize).Limit(pageSize).Find(&list)
	return total, list
}

// AddUser 添加新用户
func AddUser(u *model.User) error {
	return db.Mdb.Create(u).Error
}

// DeleteUser 删除用户
func DeleteUser(id uint) error {
	return db.Mdb.Delete(&model.User{}, id).Error
}
