package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"server-v2/internal/model"
	"server-v2/pkg/db"
	"server-v2/pkg/response"
	"server-v2/pkg/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// --------- Crontab Tasks -----------

// CreateCrontabTable 创建定时任务持久化表
func CreateCrontabTable() {
	if !db.Mdb.Migrator().HasTable(&model.CrontabRecord{}) {
		_ = db.Mdb.AutoMigrate(&model.CrontabRecord{})
	}
}

func toCrontabRecord(t model.FilmCollectTask) model.CrontabRecord {
	idsJson, _ := json.Marshal(t.Ids)
	return model.CrontabRecord{
		TaskId:    t.Id,
		IdsJson:   string(idsJson),
		Time:      t.Time,
		Spec:      t.Spec,
		TaskModel: t.Model,
		State:     t.State,
		Remark:    t.Remark,
	}
}

func fromCrontabRecord(r model.CrontabRecord) model.FilmCollectTask {
	var ids []string
	_ = json.Unmarshal([]byte(r.IdsJson), &ids)
	return model.FilmCollectTask{
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
func SaveFilmTask(t model.FilmCollectTask) {
	rec := toCrontabRecord(t)
	if err := db.Mdb.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "task_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"ids_json", "time", "spec", "task_model", "state", "remark", "updated_at"}),
	}).Create(&rec).Error; err != nil {
		log.Println("SaveFilmTask Error:", err)
	}
}

// GetAllFilmTask 获取所有的任务信息
func GetAllFilmTask() []model.FilmCollectTask {
	var records []model.CrontabRecord
	if err := db.Mdb.Find(&records).Error; err != nil {
		log.Println("GetAllFilmTask Error:", err)
		return nil
	}
	var tl []model.FilmCollectTask
	for _, r := range records {
		tl = append(tl, fromCrontabRecord(r))
	}
	return tl
}

// GetFilmTaskById 通过Id获取当前任务信息
func GetFilmTaskById(id string) (model.FilmCollectTask, error) {
	var rec model.CrontabRecord
	if err := db.Mdb.Where("task_id = ?", id).First(&rec).Error; err != nil {
		return model.FilmCollectTask{}, errors.New(" The task does not exist ")
	}
	return fromCrontabRecord(rec), nil
}

// UpdateFilmTask 更新定时任务信息(直接覆盖Id对应的定时任务信息)
func UpdateFilmTask(t model.FilmCollectTask) {
	SaveFilmTask(t)
}

// DelFilmTask 通过Id删除对应的定时任务信息
func DelFilmTask(id string) {
	if err := db.Mdb.Where("task_id = ?", id).Delete(&model.CrontabRecord{}).Error; err != nil {
		log.Println("DelFilmTask Error:", err)
	}
}

// ExistTask 是否存在定时任务相关信息
func ExistTask() bool {
	var count int64
	db.Mdb.Model(&model.CrontabRecord{}).Count(&count)
	return count > 0
}

// --------- Collect Source -----------

// GetCollectSourceList 获取采集站API列表
func GetCollectSourceList() []model.FilmSource {
	var list []model.FilmSource
	if err := db.Mdb.Order("grade ASC").Find(&list).Error; err != nil {
		log.Println("GetCollectSourceList Error:", err)
		return nil
	}
	return list
}

// GetCollectSourceListByGrade 返回指定类型的采集Api信息 Master | Slave
func GetCollectSourceListByGrade(grade model.SourceGrade) []model.FilmSource {
	var list []model.FilmSource
	if err := db.Mdb.Where("grade = ?", grade).Find(&list).Error; err != nil {
		log.Println("GetCollectSourceListByGrade Error:", err)
		return nil
	}
	return list
}

// FindCollectSourceById 通过Id标识获取对应的资源站信息
func FindCollectSourceById(id string) *model.FilmSource {
	var fs model.FilmSource
	if err := db.Mdb.Where("id = ?", id).First(&fs).Error; err != nil {
		return nil
	}
	return &fs
}

// DelCollectResource 通过Id删除对应的采集站点信息
func DelCollectResource(id string) {
	db.Mdb.Where("id = ?", id).Delete(&model.FilmSource{})
}

// AddCollectSource 添加采集站信息
func AddCollectSource(s model.FilmSource) error {
	var count int64
	db.Mdb.Model(&model.FilmSource{}).Where("uri = ?", s.Uri).Count(&count)
	if count > 0 {
		return errors.New("当前采集站点信息已存在, 请勿重复添加")
	}
	// 生成一个短uuid
	if s.Id == "" {
		s.Id = utils.GenerateSalt()
	}
	return db.Mdb.Create(&s).Error
}

// BatchAddCollectSource 批量添加采集站信息
func BatchAddCollectSource(list []model.FilmSource) error {
	return db.Mdb.Create(list).Error
}

// UpdateCollectSource 更新采集站信息
func UpdateCollectSource(s model.FilmSource) error {
	var count int64
	db.Mdb.Model(&model.FilmSource{}).Where("id != ? AND uri = ?", s.Id, s.Uri).Count(&count)
	if count > 0 {
		return errors.New("当前采集站链接已存在其他站点中, 请勿重复添加")
	}
	return db.Mdb.Save(&s).Error
}

// ClearAllCollectSource 删除所有采集站信息
func ClearAllCollectSource() {
	db.Mdb.Exec("TRUNCATE table film_sources")
}

// ExistCollectSourceList 查询是否已经存在站点list相关数据
func ExistCollectSourceList() bool {
	var count int64
	db.Mdb.Model(&model.FilmSource{}).Count(&count)
	return count > 0
}

// CreateFilmSourceTable 创建采集源信息表
func CreateFilmSourceTable() {
	if !db.Mdb.Migrator().HasTable(&model.FilmSource{}) {
		err := db.Mdb.AutoMigrate(&model.FilmSource{})
		if err != nil {
			log.Println("Create Table FilmSource Failed: ", err)
		}
	}
}

// --------- Failure Record -----------

func pendingFailureScope(tx *gorm.DB, fl model.FailureRecord) *gorm.DB {
	return tx.Where("origin_id = ? AND collect_type = ? AND page_number = ? AND hour = ? AND status = 1",
		fl.OriginId, fl.CollectType, fl.PageNumber, fl.Hour,
	)
}

func findPendingFailure(tx *gorm.DB, fl model.FailureRecord) (*model.FailureRecord, error) {
	var current model.FailureRecord
	err := pendingFailureScope(tx, fl).First(&current).Error
	if err != nil {
		return nil, err
	}
	return &current, nil
}

// CreateFailureRecordTable 创建失效记录表
func CreateFailureRecordTable() {
	fl := &model.FailureRecord{}
	// 不存在则创建FailureRecord表
	if !db.Mdb.Migrator().HasTable(fl) {
		if err := db.Mdb.AutoMigrate(fl); err != nil {
			log.Println("Create Table failure_record failed:", err)
		}
	}
}

// SaveFailureRecord 添加采集失效记录
func SaveFailureRecord(fl model.FailureRecord) {
	if fl.RetryCount <= 0 {
		fl.RetryCount = 1
	}
	// 数据量不多但存在并发问题, 开启事务
	err := db.Mdb.Transaction(func(tx *gorm.DB) error {
		current, err := findPendingFailure(tx, fl)
		if err == nil {
			if err = tx.Model(&model.FailureRecord{}).Where("id = ?", current.ID).Updates(map[string]any{
				"origin_name": fl.OriginName,
				"uri":         fl.Uri,
				"cause":       fl.Cause,
				"retry_count": gorm.Expr("retry_count + ?", 1),
			}).Error; err != nil {
				log.Println("Update failure record failed:", err)
				return err
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Println("Query failure record failed:", err)
			return err
		}

		if err = tx.Create(&fl).Error; err != nil {
			log.Println("Add failure record failed:", err)
			return err
		}
		return nil
	})
	// 如果事务提交失败, 则输出相应信息, (存一份数据到Redis??)
	if err != nil {
		log.Println("Save failure record affairs failed:", err)
	}
}

// FailureRecordList 获取所有的采集失效记录
func FailureRecordList(vo model.RecordRequestVo) []model.FailureRecord {
	// 通过RecordRequestVo,生成查询条件
	qw := db.Mdb.Model(&model.FailureRecord{})
	if vo.OriginId != "" {
		qw.Where("origin_id = ?", vo.OriginId)
	}
	if !vo.BeginTime.IsZero() && !vo.EndTime.IsZero() {
		qw.Where("created_at BETWEEN ? AND ? ", vo.BeginTime, vo.EndTime)
	}
	if vo.Status >= 0 {
		qw.Where("status = ?", vo.Status)
	}

	// 获取分页数据
	response.GetPage(qw, vo.Paging)
	// 获取分页查询的数据
	var list []model.FailureRecord
	if err := qw.Limit(vo.Paging.PageSize).Offset((vo.Paging.Current - 1) * vo.Paging.PageSize).Order("updated_at DESC").Find(&list).Error; err != nil {
		log.Println(err)
		return nil
	}
	return list
}

// FindRecordById 获取id对应的失效记录
func FindRecordById(id uint) *model.FailureRecord {
	var fr model.FailureRecord
	// 通过ID查询对应的数据
	if err := db.Mdb.First(&fr, id).Error; err != nil {
		return nil
	}
	return &fr
}

// PendingRecord 查询所有待处理的记录信息
func PendingRecord() []model.FailureRecord {
	var list []model.FailureRecord
	// 1. 获取 hour > 4320 || hour < 0  && status = 1 的影片信息
	db.Mdb.Where("(hour > 4320 OR hour < 0) AND status = 1").Find(&list)
	// 2. 获取 hour > 0 && hour < 4320 && status = 1 的影片信息(只获取最早的一条记录)
	var fr model.FailureRecord
	if err := db.Mdb.Where("hour > 0 AND hour < 4320 AND status = 1").Order("hour DESC, created_at ASC").First(&fr).Error; err == nil {
		// 3. 将 fr 添加到 list中
		list = append(list, fr)
	}
	return list
}

// ChangeRecord 修改已完成二次采集的记录状态
func ChangeRecord(fr *model.FailureRecord, status int) {
	if fr == nil || fr.ID == 0 {
		return
	}
	db.Mdb.Model(&model.FailureRecord{}).Where("id = ?", fr.ID).Update("status", status)
}

// RetryRecord 修改重试采集成功的记录
func RetryRecord(id uint, status int) error {
	// 查询id对应的失败记录
	fr := FindRecordById(id)
	if fr == nil {
		return errors.New("failure record not found")
	}
	// 将本次更新成功的记录数据状态修改为成功 0
	return db.Mdb.Model(&model.FailureRecord{}).Where("updated_at > ?", fr.UpdatedAt).Update("status", status).Error
}

// DelDoneRecord 删除已处理的记录信息 -- 逻辑删除
func DelDoneRecord() {
	if err := db.Mdb.Where("status = ?", 0).Delete(&model.FailureRecord{}).Error; err != nil {
		log.Println("Delete failure record failed:", err)
	}
}

// TruncateRecordTable  截断 record table
func TruncateRecordTable() {
	var s model.FailureRecord
	err := db.Mdb.Exec(fmt.Sprintf("TRUNCATE Table %s", s.TableName())).Error
	if err != nil {
		log.Println("TRUNCATE TABLE Error: ", err)
	}
}
