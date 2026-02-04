package handler

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/log"

	"go-wind-admin/app/admin/service/internal/websocket"
	"go-wind-admin/app/admin/service/internal/websocket/protocol"
)

// HandlerFunc 消息处理函数类型
type HandlerFunc func(*websocket.Client, *protocol.Command) error

// Router 消息路由器
type Router struct {
	handlers map[protocol.CommandType]HandlerFunc
	log      *log.Helper
}

// NewRouter 创建新的消息路由器
func NewRouter(logger log.Logger) *Router {
	return &Router{
		handlers: make(map[protocol.CommandType]HandlerFunc),
		log:      log.NewHelper(log.With(logger, "module", "websocket/router")),
	}
}

// Register 注册命令处理器
func (r *Router) Register(cmdType protocol.CommandType, handler HandlerFunc) {
	r.handlers[cmdType] = handler
	r.log.Infof("Registered handler for command type: %s (%d)", protocol.CommandTypeToString[cmdType], cmdType)
}

// RegisterByName 通过命令名称注册处理器
func (r *Router) RegisterByName(name string, handler HandlerFunc) error {
	cmdType, ok := protocol.StringToCommandType[name]
	if !ok {
		return fmt.Errorf("unknown command type: %s", name)
	}
	r.Register(cmdType, handler)
	return nil
}

// HandleCommand 路由命令到对应的处理器
func (r *Router) HandleCommand(client *websocket.Client, cmd *protocol.Command) error {
	r.log.Infof("Received command: type=%s, seq=%d, request_id=%s, client=%s",
		cmd.GetTypeString(), cmd.Seq, cmd.RequestID, client.ID)

	handler, ok := r.handlers[cmd.Type]
	if !ok {
		r.log.Warnf("No handler found for command type: %s (%d)", cmd.GetTypeString(), cmd.Type)
		// 发送错误响应
		errCmd := protocol.NewErrorCommand(404, fmt.Sprintf("Unknown command type: %s", cmd.GetTypeString()))
		errCmd.RequestID = cmd.RequestID
		errCmd.Seq = cmd.Seq
		return client.SendCommand(errCmd)
	}

	r.log.Infof("Routing command to handler: type=%s, client=%s", cmd.GetTypeString(), client.ID)
	return handler(client, cmd)
}

// HasHandler 检查是否有对应的处理器
func (r *Router) HasHandler(cmdType protocol.CommandType) bool {
	_, ok := r.handlers[cmdType]
	return ok
}

// GetRegisteredTypes 获取所有已注册的命令类型
func (r *Router) GetRegisteredTypes() []protocol.CommandType {
	types := make([]protocol.CommandType, 0, len(r.handlers))
	for t := range r.handlers {
		types = append(types, t)
	}
	return types
}
