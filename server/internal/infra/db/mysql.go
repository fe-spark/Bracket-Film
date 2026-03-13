package db

import (
	"database/sql"
	"fmt"
	"server/internal/config"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

var Mdb *gorm.DB

func InitMysql() (err error) {
	// 1. 探测与生命周期管理 (探测优先 -> 物理重建/逻辑清空 -> 按需创建)
	userDsn := config.MysqlDsn
	rootDsn := config.GetRootMysqlDsn()

	// 尝试先以普通用户连接
	db, err := sql.Open("mysql", userDsn)
	if err == nil && db.Ping() == nil {
		if config.IsDevMode {
			// 开发模式且库已存在：尝试彻底重置
			if PhysicalRebuild(db, config.MysqlDBName) != nil {
				LogicalWipe(db)
			}
		}
		db.Close()
	} else {
		// 库不存在或无权限，尝试 Root 权限创建
		if db != nil {
			db.Close()
		}
		if rootDb, err := sql.Open("mysql", rootDsn); err == nil {
			if err = rootDb.Ping(); err == nil {
				query := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;", config.MysqlDBName)
				_, _ = rootDb.Exec(query)
			}
			rootDb.Close()
		}
	}

	// 2. GORM 正式初始化连接池
	Mdb, err = gorm.Open(mysql.New(mysql.Config{
		DSN:                       userDsn,
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
	sqlDB.SetMaxIdleConns(10)                  // 最大空闲连接数
	sqlDB.SetMaxOpenConns(50)                  // 最大打开连接数
	sqlDB.SetConnMaxLifetime(time.Hour)        // 连接最大复用时间
	sqlDB.SetConnMaxIdleTime(time.Minute * 10) // 空闲连接最大存活时间

	return nil
}
