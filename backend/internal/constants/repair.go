package constants

const (
	RepairStatusPending    = "pending"
	RepairStatusAssigned   = "assigned"
	RepairStatusProcessing = "processing"
	RepairStatusDone       = "done"
	RepairStatusClosed     = "closed"
)

// RepairRatingPass 验收评分阈值：4-5 分验收通过直接关单，1-3 分退回返工
const RepairRatingPass = 4

var ValidRepairStatuses = map[string]bool{RepairStatusPending: true, RepairStatusAssigned: true, RepairStatusProcessing: true, RepairStatusDone: true, RepairStatusClosed: true}

// RepairStatusFlow 物业侧允许的状态流转；done 表示待验收，关单只能由提交人验收完成
var RepairStatusFlow = map[string][]string{
	RepairStatusPending:    {RepairStatusAssigned, RepairStatusProcessing},
	RepairStatusAssigned:   {RepairStatusProcessing},
	RepairStatusProcessing: {RepairStatusDone},
}

func CanTransit(from, to string) bool {
	for _, s := range RepairStatusFlow[from] {
		if s == to {
			return true
		}
	}
	return false
}
