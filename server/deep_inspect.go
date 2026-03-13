package main

import (
	"fmt"
	"os"
	"server/internal/infra/db"
	"server/internal/model"
	"server/internal/repository"
)

func main() {
	os.Setenv("PORT", "3601")
	os.Setenv("LISTENER_PORT", "3601")
	os.Setenv("MYSQL_HOST", "74.48.78.105")
	os.Setenv("MYSQL_PORT", "3306")
	os.Setenv("MYSQL_USER", "spark")
	os.Setenv("MYSQL_PASSWORD", "jKccK4ZFPSTybKXP")
	os.Setenv("MYSQL_DBNAME", "bracket")
	
	db.InitMysql()
	db.InitRedisConn()

	fmt.Println("--- [1] Triggering Standardization ---")
	repository.InitMainCategories()

	fmt.Println("\n--- [2] Root Categories After Standardization ---")
	var mains []model.Category
	db.Mdb.Where("pid = 0").Order("id asc").Find(&mains)
	for _, m := range mains {
		var tagCount int64
		db.Mdb.Model(&model.SearchTagItem{}).Where("pid = ?", m.Id).Count(&tagCount)
		
		fmt.Printf("ID: %d | Name: %-10s | Tags: %-2d | Alias: %s\n", 
			m.Id, m.Name, tagCount, m.Alias)
	}

	fmt.Println("\n--- [3] Sub-categories of ID 4 ---")
	var subs []model.Category
	db.Mdb.Where("pid = 4").Find(&subs)
	for _, s := range subs {
		fmt.Printf("ID: %d | Name: %-10s | Pid: %d\n", s.Id, s.Name, s.Pid)
	}

	fmt.Println("\n--- [4] Test Mapping Engine ---")
	testPid := repository.GetMainCategoryIdByName("动漫片", 4)
	fmt.Printf("Mapping '动漫片' result Pid: %d\n", testPid)
}
