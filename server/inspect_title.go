package main

import (
	"encoding/json"
	"fmt"
	"os"
	"server/internal/infra/db"
	"server/internal/service"
)

func main() {
	os.Setenv("MYSQL_HOST", "74.48.78.105")
	os.Setenv("MYSQL_PORT", "3306")
	os.Setenv("MYSQL_USER", "spark")
	os.Setenv("MYSQL_PASSWORD", "jKccK4ZFPSTybKXP")
	os.Setenv("MYSQL_DBNAME", "bracket")
	
	os.Setenv("REDIS_HOST", "74.48.78.105")
	os.Setenv("REDIS_PORT", "6379")
	os.Setenv("REDIS_PASSWORD", "redis_r3Dp6C")
	os.Setenv("REDIS_DB", "0")

	db.InitMysql()
	db.InitRedisConn()

	// 1. Check GetPidCategory result structure
	fmt.Println("--- [1] GetPidCategory Structure ---")
	cat := service.IndexSvc.GetPidCategory(4)
	if cat != nil {
		data, _ := json.MarshalIndent(cat, "", "  ")
		fmt.Printf("CategoryTree JSON:\n%s\n", string(data))
		
		fmt.Println("\n--- [2] Flattened Structure ---")
		fmt.Printf("ID: %d, Name: %s, Children Count: %d\n", cat.Id, cat.Name, len(cat.Children))
	} else {
		fmt.Println("Category 4 not found.")
	}
}
