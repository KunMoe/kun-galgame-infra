package service

import (
	"context"
	"time"

	"api/internal/platform/settings/keys"

	"gorm.io/gorm"
)

type ReporterWeight struct {
	Weight float32
	Staff  bool
}

type Weigher interface {
	Weigh(ctx context.Context, reporterID int64) (ReporterWeight, error)
}

type DBWeigher struct{ mainDB *gorm.DB }

func NewDBWeigher(mainDB *gorm.DB) *DBWeigher { return &DBWeigher{mainDB: mainDB} }

var staffRoles = []string{"moderator", "admin", "ren"}

func (w *DBWeigher) Weigh(ctx context.Context, reporterID int64) (ReporterWeight, error) {
	db := w.mainDB.WithContext(ctx)

	var createdAt time.Time
	if err := db.Table("users").Select("created_at").
		Where("id = ?", reporterID).Scan(&createdAt).Error; err != nil {
		return ReporterWeight{}, err
	}

	var staffCount int64
	if err := db.Table("user_roles AS ur").
		Joins("JOIN roles r ON r.id = ur.role_id").
		Where("ur.user_id = ? AND r.name IN ?", reporterID, staffRoles).
		Count(&staffCount).Error; err != nil {
		return ReporterWeight{}, err
	}
	if staffCount > 0 {
		return ReporterWeight{Weight: 1.0, Staff: true}, nil
	}

	weight := float32(1.0)
	if !createdAt.IsZero() && time.Since(createdAt) < time.Duration(keys.TrustNewAccountAgeDays.Get())*24*time.Hour {
		weight = float32(keys.TrustNewAccountReporterWeight.Get())
	}
	return ReporterWeight{Weight: weight, Staff: false}, nil
}
