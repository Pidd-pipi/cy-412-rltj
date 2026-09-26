package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/smartestate/smartestate/internal/constants"
	"github.com/smartestate/smartestate/internal/dto"
	"github.com/smartestate/smartestate/internal/repository"
	"github.com/smartestate/smartestate/internal/service"
)

type RepairHandler struct {
	Handler
	svc *service.RepairService
}

func NewRepairHandler(s *service.RepairService, h *Handler) *RepairHandler {
	return &RepairHandler{Handler: *h, svc: s}
}
func (h *RepairHandler) List(c *gin.Context) {
	v, e := h.svc.List(c.Query("status"))
	if e != nil {
		Fail(c, 500, 50001, e.Error())
		return
	}
	OK(c, v)
}
func (h *RepairHandler) Create(c *gin.Context) {
	var r dto.CreateRepairRequest
	if !Bind(c, &r, h.Validate) {
		return
	}
	v, e := h.svc.Create(c.GetUint("userID"), r.Title, r.Description, r.Type, r.Images)
	if e != nil {
		Fail(c, 500, 50001, e.Error())
		return
	}
	OK(c, v)
}
func (h *RepairHandler) Assign(c *gin.Context) {
	var r dto.AssignRepairRequest
	if !Bind(c, &r, h.Validate) {
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	v, e := h.svc.Assign(uint(id), r.HandlerID, c.GetString("role"))
	if e != nil {
		h.failRepair(c, e)
		return
	}
	OK(c, v)
}
func (h *RepairHandler) Status(c *gin.Context) {
	var r dto.UpdateRepairStatusRequest
	if !Bind(c, &r, h.Validate) {
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	v, e := h.svc.UpdateStatus(uint(id), r.Status, c.GetString("role"))
	if e != nil {
		h.failRepair(c, e)
		return
	}
	OK(c, v)
}

// Accept 由工单提交人（业主）完成完工验收评价；物业不持有该动作。
func (h *RepairHandler) Accept(c *gin.Context) {
	var r dto.AcceptRepairRequest
	if !Bind(c, &r, h.Validate) {
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	v, e := h.svc.Accept(uint(id), c.GetUint("userID"), r.Rating, r.ReworkReason, c.GetString("role"))
	if e != nil {
		h.failRepair(c, e)
		return
	}
	OK(c, v)
}

// failRepair 把 service 哨兵错误映射为统一 JSON 错误响应，其余错误按 500 处理。
func (h *RepairHandler) failRepair(c *gin.Context, e error) {
	switch {
	case errors.Is(e, service.ErrRepairForbidden):
		Fail(c, http.StatusForbidden, constants.CodeForbidden, e.Error())
	case errors.Is(e, service.ErrRepairConflict):
		Fail(c, http.StatusConflict, constants.CodeConflict, e.Error())
	case errors.Is(e, service.ErrRepairInvalid):
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, e.Error())
	case errors.Is(e, repository.ErrNotFound):
		Fail(c, http.StatusNotFound, constants.CodeNotFound, constants.MessageNotFound)
	default:
		Fail(c, http.StatusInternalServerError, constants.CodeInternal, e.Error())
	}
}
