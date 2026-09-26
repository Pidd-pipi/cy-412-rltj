package service

import (
	"fmt"
	"github.com/smartestate/smartestate/internal/constants"
	"github.com/smartestate/smartestate/internal/model"
	"github.com/smartestate/smartestate/internal/repository"
	"log/slog"
	"strings"
)

type RepairService struct {
	repo   *repository.RepairRepository
	users  *repository.UserRepository
	logger *slog.Logger
}

func NewRepairService(r *repository.RepairRepository, u *repository.UserRepository, l *slog.Logger) *RepairService {
	return &RepairService{r, u, l}
}
func (s *RepairService) Create(uid uint, title, desc, typ, images string) (model.Repair, error) {
	v := model.Repair{UserID: uid, Title: title, Description: desc, Type: typ, Images: images, Status: constants.RepairStatusPending}
	if e := s.repo.Create(&v); e != nil {
		return v, fmt.Errorf("Repair[user_id=%d] create failed: %w", uid, e)
	}
	return s.repo.ByID(v.ID)
}
func (s *RepairService) List(status string) ([]model.Repair, error) { return s.repo.List(status) }
func (s *RepairService) Assign(id, handlerID uint, role string) (model.Repair, error) {
	handler, e := s.users.ByID(handlerID)
	if e != nil {
		return model.Repair{}, fmt.Errorf("Repair[id=%d] assign failed: staff not found, current role=%s: %w", id, role, e)
	}
	if handler.Role != constants.UserRoleStaff && handler.Role != constants.UserRoleAdmin {
		return model.Repair{}, fmt.Errorf("Repair[id=%d] assign failed: handler %d is not staff/admin, current role=%s", id, handlerID, role)
	}
	v, e := s.repo.ByID(id)
	if e != nil {
		return v, e
	}
	if v.Status == constants.RepairStatusDone || v.Status == constants.RepairStatusClosed {
		return v, fmt.Errorf("Repair[id=%d] assign failed: status=%s not assignable, current role=%s", id, v.Status, role)
	}
	v.HandlerID = &handlerID
	v.Handler = nil
	v.User = model.User{}
	v.Status = constants.RepairStatusAssigned
	if e = s.repo.Update(&v); e != nil {
		return v, fmt.Errorf("Repair[id=%d] assign failed: %w", id, e)
	}
	return s.repo.ByID(id)
}
func (s *RepairService) UpdateStatus(id uint, status string, role string) (model.Repair, error) {
	if !constants.ValidRepairStatuses[status] {
		return model.Repair{}, fmt.Errorf("Repair[id=%d] status failed: invalid status, current role=%s", id, role)
	}
	if status == constants.RepairStatusClosed {
		return model.Repair{}, fmt.Errorf("Repair[id=%d] status failed: close requires submitter evaluation, current role=%s", id, role)
	}
	v, e := s.repo.ByID(id)
	if e != nil {
		return v, e
	}
	if v.Status == status {
		return v, nil
	}
	if !constants.CanTransit(v.Status, status) {
		return model.Repair{}, fmt.Errorf("Repair[id=%d] status failed: cannot move %s -> %s, current role=%s", id, v.Status, status, role)
	}
	v.Status = status
	if e = s.repo.Update(&v); e != nil {
		return v, fmt.Errorf("Repair[id=%d] status failed: %w", id, e)
	}
	return s.repo.ByID(id)
}
func (s *RepairService) Evaluate(id, uid uint, rating int, reason, role string) (model.Repair, error) {
	v, e := s.repo.ByID(id)
	if e != nil {
		return v, e
	}
	if v.UserID != uid {
		return v, fmt.Errorf("Repair[id=%d] evaluate failed: only submitter can evaluate, current role=%s", id, role)
	}
	if v.Status != constants.RepairStatusDone {
		if v.Rating == rating {
			return v, nil
		}
		return v, fmt.Errorf("Repair[id=%d] evaluate failed: status=%s not awaiting acceptance, current role=%s", id, v.Status, role)
	}
	v.Rating = rating
	if rating >= constants.RepairRatingPass {
		v.Status = constants.RepairStatusClosed
		v.ReworkReason = ""
	} else {
		if strings.TrimSpace(reason) == "" {
			return v, fmt.Errorf("Repair[id=%d] evaluate failed: rework reason required for rating=%d, current role=%s", id, rating, role)
		}
		v.Status = constants.RepairStatusProcessing
		v.ReworkReason = reason
	}
	if e = s.repo.Update(&v); e != nil {
		return v, fmt.Errorf("Repair[id=%d] evaluate failed: %w", id, e)
	}
	return s.repo.ByID(id)
}
func (s *RepairService) OpenCount() (int64, error) { return s.repo.CountOpen() }
