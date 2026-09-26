package constants

const (
	RepairStatusPending    = "pending"
	RepairStatusAssigned   = "assigned"
	RepairStatusProcessing = "processing"
	// RepairStatusAcceptance：物业提交完工后的待业主验收状态，关单权在提交人（业主）手中。
	RepairStatusAcceptance = "acceptance"
	// RepairStatusDone：历史状态，语义等同于待验收，仅用于兼容旧数据。
	RepairStatusDone   = "done"
	RepairStatusClosed = "closed"
)

var ValidRepairStatuses = map[string]bool{RepairStatusPending: true, RepairStatusAssigned: true, RepairStatusProcessing: true, RepairStatusAcceptance: true, RepairStatusDone: true, RepairStatusClosed: true}

// RepairStatusOrder 为物业进度更新允许的状态机走向（只能向前推进）。
var RepairStatusOrder = map[string]int{
	RepairStatusPending:    0,
	RepairStatusAssigned:   1,
	RepairStatusProcessing: 2,
	RepairStatusAcceptance: 3,
	RepairStatusDone:       3,
	RepairStatusClosed:     4,
}

// AcceptRatingPass：4~5 分视为验收通过，直接关单；1~3 分需填写返工原因并退回处理中。
const AcceptRatingPass = 4
