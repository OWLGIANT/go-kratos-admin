package handler

import (
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

// HeartbeatHandler 心跳处理器
type HeartbeatHandler struct {
	log *log.Helper
}

// NewHeartbeatHandler 创建新的心跳处理器
func NewHeartbeatHandler(logger log.Logger) *HeartbeatHandler {
	return &HeartbeatHandler{
		log: log.NewHelper(log.With(logger, "module", "websocket/handler/heartbeat")),
	}
}

// Handle 处理心跳消息
func (h *HeartbeatHandler) Handle(client *websocket.Client, cmd *protocol.Command) error {
	// 更新客户端活动时间
	client.UpdateActivity()

	// 发送心跳响应
	respPayload := &protocol.EchoCmd{
		Response: &protocol.EchoResponse{
			Message:    "pong",
			ServerTime: time.Now().UnixMilli(),
		},
	}

	if err := client.SendResponse(protocol.CommandTypeEcho, cmd.RequestID, cmd.Seq, respPayload); err != nil {
		h.log.Errorf("Failed to send heartbeat response to client %s: %v", client.ID, err)
		return err
	}

	h.log.Debugf("Heartbeat processed for client %s", client.ID)
	return nil
}
