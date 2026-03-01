package main

import (
	"fmt"
	"log"
	"time"

	"server/config"
	"server/model/system"
	"server/plugin/SystemInit"
	"server/plugin/db"
	"server/router"
)

func init() {
	// 等待 Redis 就绪（最多重试 30 次，每次间隔 2s）
	if err := waitForRedis(30, 2*time.Second); err != nil {
		panic(err)
	}
	// 等待 MySQL 就绪
	if err := waitForMySQL(30, 2*time.Second); err != nil {
		panic(err)
	}
}

func waitForRedis(maxRetries int, interval time.Duration) error {
	var err error
	for i := 1; i <= maxRetries; i++ {
		err = db.InitRedisConn()
		if err == nil {
			log.Printf("[Init] Redis 连接成功 (第 %d 次尝试)", i)
			return nil
		}
		log.Printf("[Init] Redis 连接失败 (%d/%d): %v", i, maxRetries, err)
		time.Sleep(interval)
	}
	return fmt.Errorf("Redis 连接失败，已重试 %d 次: %w", maxRetries, err)
}

func waitForMySQL(maxRetries int, interval time.Duration) error {
	var err error
	for i := 1; i <= maxRetries; i++ {
		err = db.InitMysql()
		if err == nil {
			log.Printf("[Init] MySQL 连接成功 (第 %d 次尝试)", i)
			return nil
		}
		log.Printf("[Init] MySQL 连接失败 (%d/%d): %v", i, maxRetries, err)
		time.Sleep(interval)
	}
	return fmt.Errorf("MySQL 连接失败，已重试 %d 次: %w", maxRetries, err)
}

func main() {
	start()
}

func start() {
	// 启动前先执行数据库内容的初始化工作
	DefaultDataInit()
	// 开启路由监听
	r := router.SetupRouter()
	_ = r.Run(fmt.Sprintf(":%s", config.ListenerPort))
}

func DefaultDataInit() {
	// 1. 数据库表结构初始化 (仅在用户表不存在时执行)
	if !system.ExistUserTable() {
		SystemInit.TableInIt()
	}

	// 2. 网站基础配置和轮播图 (改为检查 Redis Key 是否存在，确保清空 Redis 后能自动恢复)
	SystemInit.BasicConfigInit()
	SystemInit.BannersInit()

	// 3. 初始化影视来源和定时任务 (内部已带有存在性检查)
	SystemInit.SpiderInit()
}
