package service

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/smartestate/smartestate/internal/constants"
	"github.com/smartestate/smartestate/internal/model"
	"github.com/smartestate/smartestate/internal/repository"
)

// 验收流程的哨兵错误，handler 用 errors.Is 映射为对应 HTTP 状态码。
var (
	ErrRepairForbidden = errors.New("repair operation forbidden")
	ErrRepairConflict  = errors.New("repair state conflict")
	ErrRepairInvalid   = errors.New("repair request invalid")
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
	if v.Status == constants.RepairStatusClosed {
		return v, fmt.Errorf("%w: Repair[id=%d] assign failed: closed order cannot be reassigned, current role=%s", ErrRepairConflict, id, role)
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

// UpdateStatus 仅供物业推进进度。物业只能让工单在
// assigned → processing → acceptance(待验收) 之间向前流转：
// 完工提交进入待验收；closed 只能由业主验收通过产生，pending 不允许回退。
func (s *RepairService) UpdateStatus(id uint, status string, role string) (model.Repair, error) {
	if !constants.ValidRepairStatuses[status] {
		return model.Repair{}, fmt.Errorf("%w: Repair[id=%d] status failed: invalid status %q, current role=%s", ErrRepairInvalid, id, status, role)
	}
	v, e := s.repo.ByID(id)
	if e != nil {
		return v, e
	}
	// 历史 done 入参与库里的旧 done 数据一律按待验收处理。
	target := status
	if target == constants.RepairStatusDone {
		target = constants.RepairStatusAcceptance
	}
	switch {
	case v.Status == constants.RepairStatusClosed:
		return v, fmt.Errorf("%w: Repair[id=%d] status failed: closed order is finished, current role=%s", ErrRepairConflict, id, role)
	case target == constants.RepairStatusClosed:
		// 关单动作只属于提交人（业主验收通过）。
		return v, fmt.Errorf("%w: Repair[id=%d] status failed: closing requires owner acceptance, current role=%s", ErrRepairForbidden, id, role)
	case target == v.Status:
		// 重复提交同一结果不再改变状态。
		return v, nil
	case target == constants.RepairStatusProcessing && v.Status == constants.RepairStatusAcceptance:
		// 待验收工单退回处理中只能由业主低分返工触发。
		return v, fmt.Errorf("%w: Repair[id=%d] status failed: only owner rework can send acceptance back to processing, current role=%s", ErrRepairConflict, id, role)
	case constants.RepairStatusOrder[target] <= constants.RepairStatusOrder[v.Status]:
		return v, fmt.Errorf("%w: Repair[id=%d] status failed: %s cannot move backward to %s, current role=%s", ErrRepairConflict, id, v.Status, target, role)
	}
	v.Status = target
	if e = s.repo.Update(&v); e != nil {
		return v, fmt.Errorf("Repair[id=%d] status failed: %w", id, e)
	}
	return s.repo.ByID(id)
}

// Accept 处理提交人（业主）的完工验收评价。
//   - 只有工单提交人可以评价，其他人（含物业、管理员、其他业主）一律拒绝且不改单；
//   - 4~5 分：验收通过，工单直接关闭，旧返工原因清除；
//   - 1~3 分：必须填写返工原因，工单退回处理中，原处理人不变，继续跟进；
//   - 重复评价（工单已不在待验收）不会再次改变状态。
func (s *RepairService) Accept(id, userID uint, rating int, reworkReason, role string) (model.Repair, error) {
	if rating < 1 || rating > 5 {
		return model.Repair{}, fmt.Errorf("%w: Repair[id=%d] accept failed: rating must be 1-5, got %d, current user=%d", ErrRepairInvalid, id, rating, userID)
	}
	v, e := s.repo.ByID(id)
	if e != nil {
		return v, e
	}
	if v.UserID != userID {
		return v, fmt.Errorf("%w: %s", ErrRepairForbidden, fmt.Sprintf(constants.MessageRepairNotOwner, id, userID, role))
	}
	// 待验收之外的任何状态都不允许再评价，保证重复提交不改变工单。
	if v.Status != constants.RepairStatusAcceptance && v.Status != constants.RepairStatusDone {
		if v.Rating > 0 {
			return v, fmt.Errorf("%w: %s", ErrRepairConflict, fmt.Sprintf(constants.MessageRepairAlreadyRated, id, v.Rating, role))
		}
		return v, fmt.Errorf("%w: %s", ErrRepairConflict, fmt.Sprintf(constants.MessageRepairNotPending, id, v.Status, role))
	}
	reason := strings.TrimSpace(reworkReason)
	if rating < constants.AcceptRatingPass && reason == "" {
		return v, fmt.Errorf("%w: %s", ErrRepairInvalid, fmt.Sprintf(constants.MessageReworkReasonNeeded, id, userID))
	}
	v.Rating = rating
	if rating >= constants.AcceptRatingPass {
		v.Status = constants.RepairStatusClosed
		v.ReworkReason = ""
	} else {
		v.Status = constants.RepairStatusProcessing
		v.ReworkReason = reason
	}
	if e = s.repo.Update(&v); e != nil {
		return v, fmt.Errorf("Repair[id=%d] accept failed: %w", id, e)
	}
	return s.repo.ByID(id)
}

func (s *RepairService) OpenCount() (int64, error) { return s.repo.CountOpen() }
