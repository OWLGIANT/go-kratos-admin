package handler

import (
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"

	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

// AlertHandler 告警处理器
type AlertHandler struct {
	manager *websocket.Manager
	log     *log.Helper
}

// NewAlertHandler 创建新的告警处理器
func NewAlertHandler(manager *websocket.Manager, logger log.Logger) *AlertHandler {
	return &AlertHandler{
		manager: manager,
		log:     log.NewHelper(log.With(logger, "module", "websocket/handler/alert")),
	}
}

// Handle 处理告警消息
func (h *AlertHandler) Handle(client *websocket.Client, cmd *protocol.Command) error {
	payload, ok := cmd.Payload.(*protocol.AlertSendCmd)
	if !ok || payload.Request == nil {
		h.log.Error("Invalid payload in alert")
		return client.SendError(cmd.RequestID, cmd.Seq, 400, "Invalid payload")
	}

	req := payload.Request
	h.log.Infof("Alert received: title=%s, level=%d, from=%s", req.Title, req.Level, client.Username)

	// 生成告警 ID
	alertID := uuid.New().String()

	// 构建告警通知命令
	notifyCmd := protocol.NewCommand(protocol.CommandTypeNotify)
	notifyCmd.Payload = &protocol.NotifyCmd{
		Request: &protocol.NotifyRequest{
			Topic:   "alert",
			Content: req.Content,
			Metadata: map[string]string{
				"alert_id": alertID,
				"title":    req.Title,
				"level":    string(rune(req.Level)),
				"robot_id": req.RobotID,
				"from":     client.Username,
			},
		},
	}

	// 广播给租户内所有客户端
	h.manager.BroadcastCommandToTenant(client.TenantID, notifyCmd)

	// 发送成功响应
	respPayload := &protocol.AlertSendCmd{
		Response: &protocol.AlertSendResponse{
			AlertID: alertID,
			Success: true,
		},
	}
	return client.SendResponse(protocol.CommandTypeAlertSend, cmd.RequestID, cmd.Seq, respPayload)
}
