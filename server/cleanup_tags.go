package main

import (
	"fmt"
	"log"
	"server/internal/config"
	"server/internal/infra/db"
	"server/internal/model"
	"server/internal/repository"
)

func main() {
	// Set config directly
	config.MysqlDsn = "spark:jKccK4ZFPSTybKXP@(74.48.78.105:3306)/Bracket?charset=utf8mb4&parseTime=True&loc=Local"

	if err := db.InitMysql(); err != nil {
		log.Fatalf("Init DB Error: %v", err)
	}

	fmt.Println("Cleaning up SearchTagItem (Category & Year)...")
	db.Mdb.Where("tag_type IN ?", []string{"Category", "Year"}).Delete(&model.SearchTagItem{})

	fmt.Println("Re-populating tags from existing SearchInfo...")
	var infos []model.SearchInfo
	db.Mdb.Find(&infos)

	fmt.Printf("Processing %d movies...\n", len(infos))
	repository.BatchHandleSearchTag(infos...)

	fmt.Println("Done. Checking current Year tags...")
	var tags []model.SearchTagItem
	db.Mdb.Where("tag_type = 'Year'").Order("score DESC").Find(&tags)
	for _, t := range tags {
		fmt.Printf("Pid: %d, Name: %s, Value: %s, Score: %d\n", t.Pid, t.Name, t.Value, t.Score)
	}
}
