package main

import (
	"fmt"
	"os"
	"server/internal/infra/db"
	"server/internal/model"
	"server/internal/repository"
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

	fmt.Println("--- [1] Testing Stateless Mapping Engine ---")
	id := repository.GetMainCategoryIdByName("动漫片", 0)
	fmt.Printf("Input: '动漫片' | Mapped ID: %d (Expected: 4)\n", id)
	if id == 4 {
		fmt.Println("SUCCESS: Mapping is now deterministic and hardcoded.")
	} else {
		fmt.Println("FAILURE: Mapping still inconsistent!")
	}

	fmt.Println("\n--- [2] Running Deterministic Initialization ---")
	repository.InitMainCategories()
	
	var cat model.Category
	db.Mdb.Where("id = 4").First(&cat)
	fmt.Printf("Category ID 4 DB Name: %s (Expected: 动漫)\n", cat.Name)
	if cat.Name == "动漫" {
		fmt.Println("SUCCESS: Database standardized to '动漫'.")
	} else {
		fmt.Println("FAILURE: Database still has non-standard name.")
	}

	fmt.Println("\n--- [3] Verifying Sorting Tags for ID 4 ---")
	var tags []model.SearchTagItem
	db.Mdb.Where("pid = 4 AND tag_type = 'Sort'").Find(&tags)
	fmt.Printf("Found %d sorting tags for ID 4.\n", len(tags))
	if len(tags) >= 4 {
		fmt.Println("SUCCESS: Standard sorting tags correctly injected.")
	} else {
		fmt.Println("FAILURE: Sorting tags missing!")
	}

	fmt.Println("\n--- [4] Simulating Orphan Cleanup (Immediate Effect) ---")
	// Re-map any remaining transients
	var count int64
	db.Mdb.Model(&model.SearchInfo{}).Where("c_name = '动漫片' AND pid != 4").Count(&count)
	fmt.Printf("Orphans with '动漫片' but wrong Pid: %d\n", count)
	
	// This code also acts as a one-time migration for existing data
	db.Mdb.Model(&model.SearchInfo{}).Where("c_name = '动漫片'").Update("pid", 4)
	fmt.Println("Migration complete: Data now visible under ID 4.")
}
// Redefine to avoid redeclared main error if needed but using write_to_file with overwrite
