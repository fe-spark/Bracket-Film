package system

import (
	"encoding/json"
	"errors"
	"log"

	"server/plugin/db"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

/*
	定时任务持久化 (MySQL)
*/

// FilmCollectTask 影视采集任务
type FilmCollectTask struct {
	Id     string       `json:"id"`     // 唯一标识uid
	Ids    []string     `json:"ids"`    // 采集站id列表
	Cid    cron.EntryID `json:"cid"`    // 定时任务Id (运行时字段，不持久化)
	Time   int          `json:"time"`   // 采集时长, 最新x小时更新的内容
	Spec   string       `json:"spec"`   // 执行周期 cron表达式
	Model  int          `json:"model"`  // 任务类型, 0 - 自动更新已启用站点 || 1 - 更新Ids中的资源站数据 || 2 - 定期清理失败采集记录
	State  bool         `json:"state"`  // 状态 开启 | 禁用
	Remark string       `json:"remark"` // 任务备注信息
}

// CrontabRecord 定时任务持久化模型 (MySQL)
// Cid (cron.EntryID) 为运行时字段，由 cron 调度器动态分配，不做持久化
type CrontabRecord struct {
	gorm.Model
	TaskId    string `gorm:"uniqueIndex;size:64"`
	IdsJson   string `gorm:"type:text"` // JSON 序列化的 []string
	Time      int
	Spec      string `gorm:"size:64"`
	TaskModel int    `gorm:"column:task_model"` // 任务类型 (避免与 gorm.Model 嵌入名冲突)
	State     bool
	Remark    string `gorm:"size:256"`
}

// CreateCrontabTable 创建定时任务持久化表
func CreateCrontabTable() {
	if !db.Mdb.Migrator().HasTable(&CrontabRecord{}) {
		_ = db.Mdb.AutoMigrate(&CrontabRecord{})
	}
}

func toCrontabRecord(t FilmCollectTask) CrontabRecord {
	idsJson, _ := json.Marshal(t.Ids)
	return CrontabRecord{
		TaskId:    t.Id,
		IdsJson:   string(idsJson),
		Time:      t.Time,
		Spec:      t.Spec,
		TaskModel: t.Model,
		State:     t.State,
		Remark:    t.Remark,
	}
}

func fromCrontabRecord(r CrontabRecord) FilmCollectTask {
	var ids []string
	_ = json.Unmarshal([]byte(r.IdsJson), &ids)
	return FilmCollectTask{
		Id:     r.TaskId,
		Ids:    ids,
		Time:   r.Time,
		Spec:   r.Spec,
		Model:  r.TaskModel,
		State:  r.State,
		Remark: r.Remark,
	}
}

// SaveFilmTask 保存影视采集任务信息
func SaveFilmTask(t FilmCollectTask) {
	rec := toCrontabRecord(t)
	if err := db.Mdb.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "task_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"ids_json", "time", "spec", "model", "state", "remark", "updated_at"}),
	}).Create(&rec).Error; err != nil {
		log.Println("SaveFilmTask Error:", err)
	}
}

// GetAllFilmTask 获取所有的任务信息
func GetAllFilmTask() []FilmCollectTask {
	var records []CrontabRecord
	if err := db.Mdb.Find(&records).Error; err != nil {
		log.Println("GetAllFilmTask Error:", err)
		return nil
	}
	var tl []FilmCollectTask
	for _, r := range records {
		tl = append(tl, fromCrontabRecord(r))
	}
	return tl
}

// GetFilmTaskById 通过Id获取当前任务信息
func GetFilmTaskById(id string) (FilmCollectTask, error) {
	var rec CrontabRecord
	if err := db.Mdb.Where("task_id = ?", id).First(&rec).Error; err != nil {
		return FilmCollectTask{}, errors.New(" The task does not exist ")
	}
	return fromCrontabRecord(rec), nil
}

// UpdateFilmTask 更新定时任务信息(直接覆盖Id对应的定时任务信息)
func UpdateFilmTask(t FilmCollectTask) {
	SaveFilmTask(t)
}

// DelFilmTask 通过Id删除对应的定时任务信息
func DelFilmTask(id string) {
	if err := db.Mdb.Where("task_id = ?", id).Delete(&CrontabRecord{}).Error; err != nil {
		log.Println("DelFilmTask Error:", err)
	}
}

// ExistTask 是否存在定时任务相关信息
func ExistTask() bool {
	var count int64
	db.Mdb.Model(&CrontabRecord{}).Count(&count)
	return count > 0
}
