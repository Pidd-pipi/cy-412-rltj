package constants

const (
	MessageOK             = "ok"
	MessageUnauthorized   = "登录已失效"
	MessageForbidden      = "无权限执行此操作"
	MessageValidation     = "请求参数不合法"
	MessageNotFound       = "资源不存在"
	MessagePaymentSuccess = "支付宝沙箱支付成功"
	MessageRepairCreated  = "报修工单已提交"
)

// 完工验收流程提示文案（前端展示、后端返回与日志内容共用，改动需三处同步）。
const (
	MessageRepairNotOwner     = "Repair[id=%d] accept failed: only the submitter can verify, current user=%d role=%s"
	MessageRepairNotPending   = "Repair[id=%d] accept failed: status %s is not pending acceptance, current role=%s"
	MessageRepairAlreadyRated = "Repair[id=%d] accept failed: already rated %d, current role=%s"
	MessageReworkReasonNeeded = "Repair[id=%d] accept failed: rework reason is required when rating 1-3, current user=%d"
)
