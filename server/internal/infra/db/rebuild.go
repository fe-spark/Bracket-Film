package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// PhysicalRebuild 执行最彻底的物理删库重建
// 需要具有 DROP DATABASE 和 CREATE DATABASE 权限
func PhysicalRebuild(conn *sql.DB, dbName string) error {
	log.Printf("[Rebuild] 正在物理删除数据库 `%s`...\n", dbName)
	if _, err := conn.Exec(fmt.Sprintf("DROP DATABASE `%s`;", dbName)); err != nil {
		return err
	}

	log.Printf("[Rebuild] 正在重新创建数据库 `%s`...\n", dbName)
	query := fmt.Sprintf("CREATE DATABASE `%s` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;", dbName)
	if _, err := conn.Exec(query); err != nil {
		return err
	}

	// 给文件系统一点反应时间
	time.Sleep(500 * time.Millisecond)
	return nil
}

// LogicalWipe 获取所有表并 TRUNCATE，模拟删号重来的效果
// 在没有删库权限时的降级方案
func LogicalWipe(conn *sql.DB) {
	rows, err := conn.Query("SHOW TABLES")
	if err != nil {
		log.Printf("[Reset] 获取表列表失败: %v\n", err)
		return
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err == nil {
			tables = append(tables, table)
		}
	}

	if len(tables) == 0 {
		return
	}

	// 禁用外键检查并清空
	conn.Exec("SET FOREIGN_KEY_CHECKS = 0;")
	for _, table := range tables {
		if _, err := conn.Exec(fmt.Sprintf("TRUNCATE TABLE `%s`;", table)); err != nil {
			log.Printf("[Reset] TRUNCATE TABLE `%s` 失败: %v\n", table, err)
		}
	}
	conn.Exec("SET FOREIGN_KEY_CHECKS = 1;")
	log.Printf("[Reset] 数据库所有表已通过逻辑清空复位\n")
}
