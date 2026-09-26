package service

import (
	"github.com/smartestate/smartestate/internal/constants"
	"github.com/smartestate/smartestate/internal/model"
	"github.com/smartestate/smartestate/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log/slog"
	"testing"
)

func newRepairSvc(t *testing.T) (*RepairService, *gorm.DB, uint, uint, uint) {
	t.Helper()
	db, e := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	if e = db.AutoMigrate(&model.User{}, &model.Repair{}); e != nil {
		t.Fatal(e)
	}
	owner := model.User{Phone: "1", Nickname: "业主", Role: constants.UserRoleResident}
	other := model.User{Phone: "2", Nickname: "邻居", Role: constants.UserRoleResident}
	staff := model.User{Phone: "3", Nickname: "物业", Role: constants.UserRoleStaff}
	db.Create(&owner)
	db.Create(&other)
	db.Create(&staff)
	return NewRepairService(repository.NewRepairRepository(db), repository.NewUserRepository(db), slog.Default()), db, owner.ID, other.ID, staff.ID
}

func doneRepair(t *testing.T, db *gorm.DB, ownerID uint) model.Repair {
	t.Helper()
	v := model.Repair{UserID: ownerID, Title: "t", Description: "d", Type: "水电", Status: constants.RepairStatusDone}
	if e := db.Create(&v).Error; e != nil {
		t.Fatal(e)
	}
	return v
}

func TestEvaluateHighRatingCloses(t *testing.T) {
	s, db, owner, _, _ := newRepairSvc(t)
	v := doneRepair(t, db, owner)
	got, e := s.Evaluate(v.ID, owner, 5, "", constants.UserRoleResident)
	if e != nil || got.Status != constants.RepairStatusClosed || got.Rating != 5 {
		t.Fatalf("got status=%s rating=%d err=%v", got.Status, got.Rating, e)
	}
}

func TestEvaluateLowRatingReworks(t *testing.T) {
	s, db, owner, _, staffID := newRepairSvc(t)
	v := doneRepair(t, db, owner)
	db.Model(&v).Update("handler_id", staffID)
	got, e := s.Evaluate(v.ID, owner, 2, "灯具仍然闪烁", constants.UserRoleResident)
	if e != nil {
		t.Fatal(e)
	}
	if got.Status != constants.RepairStatusProcessing || got.Rating != 2 || got.ReworkReason != "灯具仍然闪烁" {
		t.Fatalf("got status=%s rating=%d reason=%q", got.Status, got.Rating, got.ReworkReason)
	}
	if got.HandlerID == nil || *got.HandlerID != staffID {
		t.Fatalf("handler should stay, got %v", got.HandlerID)
	}
}

func TestEvaluateLowRatingNeedsReason(t *testing.T) {
	s, db, owner, _, _ := newRepairSvc(t)
	v := doneRepair(t, db, owner)
	if _, e := s.Evaluate(v.ID, owner, 3, "  ", constants.UserRoleResident); e == nil {
		t.Fatal("rating 1-3 without reason should fail")
	}
	got, _ := s.repo.ByID(v.ID)
	if got.Status != constants.RepairStatusDone {
		t.Fatalf("status changed to %s", got.Status)
	}
}

func TestEvaluateOnlySubmitter(t *testing.T) {
	s, db, owner, other, _ := newRepairSvc(t)
	v := doneRepair(t, db, owner)
	if _, e := s.Evaluate(v.ID, other, 5, "", constants.UserRoleResident); e == nil {
		t.Fatal("non-submitter evaluate should fail")
	}
	got, _ := s.repo.ByID(v.ID)
	if got.Status != constants.RepairStatusDone || got.Rating != 0 {
		t.Fatalf("repair changed by other: status=%s rating=%d", got.Status, got.Rating)
	}
}

func TestEvaluateIdempotentSameResult(t *testing.T) {
	s, db, owner, _, _ := newRepairSvc(t)
	for _, tt := range []struct {
		rating int
		reason string
		want   string
	}{{5, "", constants.RepairStatusClosed}, {2, "没修好", constants.RepairStatusProcessing}} {
		fresh := doneRepair(t, db, owner)
		if _, e := s.Evaluate(fresh.ID, owner, tt.rating, tt.reason, constants.UserRoleResident); e != nil {
			t.Fatal(e)
		}
		got, e := s.Evaluate(fresh.ID, owner, tt.rating, tt.reason, constants.UserRoleResident)
		if e != nil {
			t.Fatalf("repeat same result should not error: %v", e)
		}
		if got.Status != tt.want || got.Rating != tt.rating {
			t.Fatalf("rating %d: got status=%s rating=%d", tt.rating, got.Status, got.Rating)
		}
	}
}

func TestEvaluateRequiresAwaitingAcceptance(t *testing.T) {
	s, db, owner, _, _ := newRepairSvc(t)
	v := model.Repair{UserID: owner, Title: "t", Description: "d", Type: "水电", Status: constants.RepairStatusProcessing}
	db.Create(&v)
	if _, e := s.Evaluate(v.ID, owner, 4, "", constants.UserRoleResident); e == nil {
		t.Fatal("evaluate on processing repair should fail")
	}
}

func TestUpdateStatusFlow(t *testing.T) {
	s, db, owner, _, _ := newRepairSvc(t)
	v := model.Repair{UserID: owner, Title: "t", Description: "d", Type: "水电", Status: constants.RepairStatusProcessing}
	db.Create(&v)
	if _, e := s.UpdateStatus(v.ID, constants.RepairStatusClosed, constants.UserRoleStaff); e == nil {
		t.Fatal("staff should not close directly")
	}
	if _, e := s.UpdateStatus(v.ID, constants.RepairStatusDone, constants.UserRoleStaff); e != nil {
		t.Fatalf("processing -> done should pass: %v", e)
	}
	if _, e := s.UpdateStatus(v.ID, constants.RepairStatusProcessing, constants.UserRoleStaff); e == nil {
		t.Fatal("done -> processing by staff should fail, rework only via evaluation")
	}
	got, e := s.UpdateStatus(v.ID, constants.RepairStatusDone, constants.UserRoleStaff)
	if e != nil || got.Status != constants.RepairStatusDone {
		t.Fatalf("repeat same status should be a no-op, got %s err %v", got.Status, e)
	}
}
