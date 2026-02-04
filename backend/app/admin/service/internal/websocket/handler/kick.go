package handler

import (
	"strconv"

	"github.com/go-kratos/kratos/v2/log"

	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

// KickHandler 踢出用户处理器
type KickHandler struct {
	manager *websocket.Manager
	log     *log.Helper
}

// NewKickHandler 创建新的踢出用户处理器
func NewKickHandler(manager *websocket.Manager, logger log.Logger) *KickHandler {
	return &KickHandler{
		manager: manager,
		log:     log.NewHelper(log.With(logger, "module", "websocket/handler/kick")),
	}
}

// Handle 处理踢出用户消息
func (h *KickHandler) Handle(client *websocket.Client, cmd *protocol.Command) error {
	payload, ok := cmd.Payload.(*protocol.UserKickCmd)
	if !ok || payload.Request == nil {
		h.log.Error("Invalid payload in kick")
		return client.SendError(cmd.RequestID, cmd.Seq, 400, "Invalid payload")
	}

	req := payload.Request

	// 解析用户 ID
	targetUserID, err := strconv.ParseUint(req.UserID, 10, 32)
	if err != nil {
		h.log.Errorf("Invalid user_id: %s", req.UserID)
		return client.SendError(cmd.RequestID, cmd.Seq, 400, "Invalid user_id")
	}

	reason := req.Reason
	if reason == "" {
		reason = "You have been logged out"
	}

	h.log.Infof("Kicking user %d: %s (requested by %s)", targetUserID, reason, client.Username)

	// 踢出用户（关闭所有连接）
	h.manager.KickUser(uint32(targetUserID), reason)

	// 发送成功响应
	respPayload := &protocol.UserKickCmd{
		Response: &protocol.UserKickResponse{
			Success: true,
		},
	}
	return client.SendResponse(protocol.CommandTypeUserKick, cmd.RequestID, cmd.Seq, respPayload)
}
