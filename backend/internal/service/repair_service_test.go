package service

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/smartestate/smartestate/internal/constants"
	"github.com/smartestate/smartestate/internal/model"
	"github.com/smartestate/smartestate/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newRepairServiceFixture(t *testing.T) (*RepairService, *gorm.DB, model.User, model.User) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err = db.AutoMigrate(&model.User{}, &model.Repair{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	owner := model.User{Phone: "1", Nickname: "owner", Role: "resident"}
	staff := model.User{Phone: "2", Nickname: "staff", Role: "staff"}
	if err = db.Create(&owner).Error; err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if err = db.Create(&staff).Error; err != nil {
		t.Fatalf("create staff: %v", err)
	}
	svc := NewRepairService(repository.NewRepairRepository(db), repository.NewUserRepository(db), slog.New(slog.NewTextHandler(io.Discard, nil)))
	return svc, db, owner, staff
}

func createRepairAt(t *testing.T, db *gorm.DB, ownerID uint, status string) model.Repair {
	t.Helper()
	r := model.Repair{UserID: ownerID, Title: "灯坏了", Description: "客厅灯具闪烁", Type: "水电", Status: status}
	if err := db.Create(&r).Error; err != nil {
		t.Fatalf("create repair: %v", err)
	}
	return r
}

func TestUpdateStatusStaffFlowTable(t *testing.T) {
	for _, tt := range []struct {
		name    string
		start   string
		target  string
		wantErr bool
		want    string
	}{
		{"assigned to processing", constants.RepairStatusAssigned, constants.RepairStatusProcessing, false, constants.RepairStatusProcessing},
		{"processing to acceptance", constants.RepairStatusProcessing, constants.RepairStatusAcceptance, false, constants.RepairStatusAcceptance},
		{"legacy done maps to acceptance", constants.RepairStatusProcessing, constants.RepairStatusDone, false, constants.RepairStatusAcceptance},
		{"idempotent same status", constants.RepairStatusProcessing, constants.RepairStatusProcessing, false, constants.RepairStatusProcessing},
		{"staff cannot close", constants.RepairStatusProcessing, constants.RepairStatusClosed, true, constants.RepairStatusProcessing},
		{"cannot move backward", constants.RepairStatusProcessing, constants.RepairStatusAssigned, true, constants.RepairStatusProcessing},
		{"closed is terminal", constants.RepairStatusClosed, constants.RepairStatusProcessing, true, constants.RepairStatusClosed},
		{"acceptance only reopens via owner rework", constants.RepairStatusAcceptance, constants.RepairStatusProcessing, true, constants.RepairStatusAcceptance},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, db, owner, _ := newRepairServiceFixture(t)
			r := createRepairAt(t, db, owner.ID, tt.start)
			got, err := svc.UpdateStatus(r.ID, tt.target, "staff")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got status %s", got.Status)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got, _ = svc.repo.ByID(r.ID)
			if got.Status != tt.want {
				t.Fatalf("status = %s, want %s", got.Status, tt.want)
			}
		})
	}
}

func TestAcceptFlowTable(t *testing.T) {
	for _, tt := range []struct {
		name         string
		start        string
		rating       int
		reason       string
		actorOffset  uint // 0 = 提交人本人，其他值为伪造的用户 ID
		wantErr      bool
		wantStatus   string
		wantReason   string
		wantRating   int
		wantSentinel error
	}{
		{"4 stars closes", constants.RepairStatusAcceptance, 4, "", 0, false, constants.RepairStatusClosed, "", 4, nil},
		{"5 stars closes", constants.RepairStatusAcceptance, 5, "", 0, false, constants.RepairStatusClosed, "", 5, nil},
		{"3 stars needs reason", constants.RepairStatusAcceptance, 3, "", 0, true, constants.RepairStatusAcceptance, "", 0, ErrRepairInvalid},
		{"1 star reworks to processing", constants.RepairStatusAcceptance, 1, "仍然漏水", 0, false, constants.RepairStatusProcessing, "仍然漏水", 1, nil},
		{"2 stars reworks to processing", constants.RepairStatusAcceptance, 2, "灯还是闪", 0, false, constants.RepairStatusProcessing, "灯还是闪", 2, nil},
		{"other user cannot rate", constants.RepairStatusAcceptance, 5, "", 999, true, constants.RepairStatusAcceptance, "", 0, ErrRepairForbidden},
		{"rating not in acceptance", constants.RepairStatusProcessing, 5, "", 0, true, constants.RepairStatusProcessing, "", 0, ErrRepairConflict},
		{"duplicate pass keeps closed", constants.RepairStatusClosed, 5, "", 0, true, constants.RepairStatusClosed, "", 5, ErrRepairConflict},
		{"duplicate rework keeps processing", constants.RepairStatusProcessing, 2, "又坏了", 0, true, constants.RepairStatusProcessing, "仍然漏水", 2, ErrRepairConflict},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, db, owner, _ := newRepairServiceFixture(t)
			r := createRepairAt(t, db, owner.ID, tt.start)
			if tt.name == "duplicate pass keeps closed" {
				if err := db.Model(&model.Repair{}).Where("id = ?", r.ID).Update("rating", 5).Error; err != nil {
					t.Fatalf("seed rating: %v", err)
				}
			}
			if tt.name == "duplicate rework keeps processing" {
				if err := db.Model(&model.Repair{}).Where("id = ?", r.ID).Updates(map[string]any{"rating": 2, "rework_reason": "仍然漏水"}).Error; err != nil {
					t.Fatalf("seed rating: %v", err)
				}
			}
			actor := owner.ID + tt.actorOffset
			got, err := svc.Accept(r.ID, actor, tt.rating, tt.reason, "resident")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got status %s", got.Status)
				}
				if tt.wantSentinel != nil && !errors.Is(err, tt.wantSentinel) {
					t.Fatalf("err = %v, want sentinel %v", err, tt.wantSentinel)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got, _ = svc.repo.ByID(r.ID)
			if got.Status != tt.wantStatus {
				t.Fatalf("status = %s, want %s", got.Status, tt.wantStatus)
			}
			if got.Rating != tt.wantRating {
				t.Fatalf("rating = %d, want %d", got.Rating, tt.wantRating)
			}
			if got.ReworkReason != tt.wantReason {
				t.Fatalf("rework_reason = %q, want %q", got.ReworkReason, tt.wantReason)
			}
		})
	}
}

func TestReworkThenRepairThenAcceptKeepsOriginalHandler(t *testing.T) {
	svc, db, owner, staff := newRepairServiceFixture(t)
	r := createRepairAt(t, db, owner.ID, constants.RepairStatusProcessing)
	if err := db.Model(&model.Repair{}).Where("id = ?", r.ID).Update("handler_id", staff.ID).Error; err != nil {
		t.Fatalf("assign handler: %v", err)
	}
	// 物业提交完工 -> 待验收。
	if _, err := svc.UpdateStatus(r.ID, constants.RepairStatusAcceptance, "staff"); err != nil {
		t.Fatalf("submit done: %v", err)
	}
	// 业主评 2 分返工 -> 处理中，原处理人保留。
	if _, err := svc.Accept(r.ID, owner.ID, 2, "没修好", "resident"); err != nil {
		t.Fatalf("rework: %v", err)
	}
	r, _ = svc.repo.ByID(r.ID)
	if r.Status != constants.RepairStatusProcessing || r.HandlerID == nil || *r.HandlerID != staff.ID {
		t.Fatalf("rework should keep processing and original handler, got %+v", r)
	}
	// 原处理人继续跟进，再次完工 -> 待验收。
	if _, err := svc.UpdateStatus(r.ID, constants.RepairStatusAcceptance, "staff"); err != nil {
		t.Fatalf("resubmit done: %v", err)
	}
	// 业主评 5 分 -> 关闭。
	got, err := svc.Accept(r.ID, owner.ID, 5, "", "resident")
	if err != nil || got.Status != constants.RepairStatusClosed || got.Rating != 5 {
		t.Fatalf("final accept = %+v, err %v", got, err)
	}
	if got.ReworkReason != "" {
		t.Fatalf("passing acceptance should clear old rework reason, got %q", got.ReworkReason)
	}
}
