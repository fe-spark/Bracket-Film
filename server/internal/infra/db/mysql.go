package db

import (
	"server/internal/config"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

var Mdb *gorm.DB

func InitMysql() (err error) {
	Mdb, err = gorm.Open(mysql.New(mysql.Config{
		DSN:                       config.MysqlDsn,
		DefaultStringSize:         255,   //string类型字段默认长度
		DisableDatetimePrecision:  true,  // 禁用 datetime 精度
		DontSupportRenameIndex:    true,  // 重命名索引时采用删除并新建的方式
		DontSupportRenameColumn:   true,  // 用change 重命名列
		SkipInitializeWithVersion: false, // 根据当前Mysql版本自动配置
	}), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true, //是否使用 结构体名称作为表名 (关闭自动变复数)
		},
		Logger: logger.Default.LogMode(logger.Error), //设置日志级别为Error, 避免采集时打印繁杂的 SQL 语句
	})

	if err != nil {
		return err
	}

	sqlDB, err := Mdb.DB()
	if err != nil {
		return err
	}

	// 设置连接池
	sqlDB.SetMaxIdleConns(10)           // 最大空闲连接数
	sqlDB.SetMaxOpenConns(50)           // 最大打开连接数
	sqlDB.SetConnMaxLifetime(time.Hour) // 连接最大复用时间

	return nil
}
