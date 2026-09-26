package repository

import (
	"errors"
	"github.com/smartestate/smartestate/internal/model"
	"gorm.io/gorm"
)

type RepairRepository struct{ DB *gorm.DB }

func NewRepairRepository(db *gorm.DB) *RepairRepository  { return &RepairRepository{db} }
func (r *RepairRepository) Create(v *model.Repair) error { return r.DB.Create(v).Error }
func (r *RepairRepository) List(status string) (out []model.Repair, e error) {
	q := r.DB.Preload("User").Preload("Handler").Order("created_at desc")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	e = q.Find(&out).Error
	return
}
func (r *RepairRepository) ByID(id uint) (v model.Repair, e error) {
	e = r.DB.Preload("User").Preload("Handler").First(&v, id).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		e = ErrNotFound
	}
	return
}
func (r *RepairRepository) Update(v *model.Repair) error { return r.DB.Save(v).Error }

// MigrateLegacyDone 把旧版本物业完工即关闭前留下的 done 工单归一成待验收，
// 让历史数据也走业主验收流程。
func (r *RepairRepository) MigrateLegacyDone() (int64, error) {
	res := r.DB.Model(&model.Repair{}).Where("status = ?", "done").Update("status", "acceptance")
	return res.RowsAffected, res.Error
}

// CountOpen 统计物业仍需跟进的工单；待验收工单等待业主操作，不计入物业待办。
func (r *RepairRepository) CountOpen() (int64, error) {
	var n int64
	e := r.DB.Model(&model.Repair{}).Where("status IN ?", []string{"pending", "assigned", "processing"}).Count(&n).Error
	return n, e
}
