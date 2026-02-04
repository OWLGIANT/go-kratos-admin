package protocol

import (
	"time"
)

// ContentType 内容类型
type ContentType int

const (
	ContentTypeJSON     ContentType = 1
	ContentTypeProtobuf ContentType = 2
)

// CommandType 命令类型枚举 (不同组路由步距 600)
type CommandType int32

const (
	CommandTypeUnknown CommandType = 0

	// 基础命令 (1-599)
	CommandTypeInit   CommandType = 1
	CommandTypeEcho   CommandType = 2
	CommandTypeNotify CommandType = 3
	CommandTypeResync CommandType = 4
	CommandTypeError  CommandType = 5

	// Actor 生命周期命令 (600-1199) - Actor 相当于服务器
	CommandTypeActorRegister   CommandType = 600
	CommandTypeActorUnregister CommandType = 601
	CommandTypeActorHeartbeat  CommandType = 602
	CommandTypeActorStatus     CommandType = 603
	CommandTypeActorList       CommandType = 604

	// Robot 控制命令 (1200-1799) - Robot 是机器人
	CommandTypeRobotStart   CommandType = 1200
	CommandTypeRobotStop    CommandType = 1201
	CommandTypeRobotConfig  CommandType = 1202
	CommandTypeRobotCommand CommandType = 1203
	CommandTypeRobotResult  CommandType = 1204

	// 服务器信息命令 (1800-2399)
	CommandTypeServerSync   CommandType = 1800
	CommandTypeServerStatus CommandType = 1801

	// 告警命令 (2400-2999)
	CommandTypeAlertSend CommandType = 2400
	CommandTypeAlertAck  CommandType = 2401

	// 用户命令 (3000-3599)
	CommandTypeUserKick      CommandType = 3000
	CommandTypeUserBroadcast CommandType = 3001

	// 机器人同步命令 (3600-4199)
	CommandTypeRobotSync CommandType = 3600
)

// CommandTypeToString 命令类型转字符串
var CommandTypeToString = map[CommandType]string{
	CommandTypeUnknown:         "unknown",
	CommandTypeInit:            "init",
	CommandTypeEcho:            "echo",
	CommandTypeNotify:          "notify",
	CommandTypeResync:          "resync",
	CommandTypeError:           "error",
	CommandTypeActorRegister:   "actor.register",
	CommandTypeActorUnregister: "actor.unregister",
	CommandTypeActorHeartbeat:  "actor.heartbeat",
	CommandTypeActorStatus:     "actor.status",
	CommandTypeActorList:       "actor.list",
	CommandTypeRobotStart:      "robot.start",
	CommandTypeRobotStop:       "robot.stop",
	CommandTypeRobotConfig:     "robot.config",
	CommandTypeRobotCommand:    "robot.command",
	CommandTypeRobotResult:     "robot.result",
	CommandTypeServerSync:      "server.sync",
	CommandTypeServerStatus:    "server.status",
	CommandTypeAlertSend:       "alert.send",
	CommandTypeAlertAck:        "alert.ack",
	CommandTypeUserKick:        "user.kick",
	CommandTypeUserBroadcast:   "user.broadcast",
	CommandTypeRobotSync:       "robot.sync",
}

// StringToCommandType 字符串转命令类型
var StringToCommandType = map[string]CommandType{
	"unknown":          CommandTypeUnknown,
	"init":             CommandTypeInit,
	"echo":             CommandTypeEcho,
	"notify":           CommandTypeNotify,
	"resync":           CommandTypeResync,
	"error":            CommandTypeError,
	"actor.register":   CommandTypeActorRegister,
	"actor.unregister": CommandTypeActorUnregister,
	"actor.heartbeat":  CommandTypeActorHeartbeat,
	"actor.status":     CommandTypeActorStatus,
	"actor.list":       CommandTypeActorList,
	"robot.start":      CommandTypeRobotStart,
	"robot.stop":       CommandTypeRobotStop,
	"robot.config":     CommandTypeRobotConfig,
	"robot.command":    CommandTypeRobotCommand,
	"robot.result":     CommandTypeRobotResult,
	"server.sync":      CommandTypeServerSync,
	"server.status":    CommandTypeServerStatus,
	"alert.send":       CommandTypeAlertSend,
	"alert.ack":        CommandTypeAlertAck,
	"user.kick":        CommandTypeUserKick,
	"user.broadcast":   CommandTypeUserBroadcast,
	"robot.sync":       CommandTypeRobotSync,
}

// EventType 事件类型
type EventType int32

const (
	EventTypeUnknown             EventType = 0
	EventTypeActorConnected      EventType = 1
	EventTypeActorDisconnected   EventType = 2
	EventTypeActorStatusChanged  EventType = 3
	EventTypeServerStatusChanged EventType = 4
	EventTypeAlert               EventType = 5
)

// AlertLevel 告警级别
type AlertLevel int32

const (
	AlertLevelUnknown  AlertLevel = 0
	AlertLevelInfo     AlertLevel = 1
	AlertLevelWarning  AlertLevel = 2
	AlertLevelError    AlertLevel = 3
	AlertLevelCritical AlertLevel = 4
)

// ErrorMessage 错误信息
type ErrorMessage struct {
	Code    int32  `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// Event 事件
type Event struct {
	Type      EventType         `json:"type"`
	Timestamp time.Time         `json:"timestamp"`
	Data      map[string]string `json:"data,omitempty"`
}

// Command 主命令消息
type Command struct {
	Type      CommandType   `json:"type"`
	Seq       uint64        `json:"seq"`
	RequestID string        `json:"request_id,omitempty"`
	Error     *ErrorMessage `json:"error,omitempty"`
	Events    []Event       `json:"events,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	Payload   interface{}   `json:"payload,omitempty"`
}

// ServerStatusInfo 服务器状态信息
type ServerStatusInfo struct {
	CPU               string            `json:"cpu,omitempty"`
	IPPool            float64           `json:"ip_pool,omitempty"`
	Mem               float64           `json:"mem,omitempty"`
	MemPct            string            `json:"mem_pct,omitempty"`
	DiskPct           string            `json:"disk_pct,omitempty"`
	TaskNum           int32             `json:"task_num,omitempty"`
	StraVersion       bool              `json:"stra_version,omitempty"`
	StraVersionDetail map[string]string `json:"stra_version_detail,omitempty"`
	AwsAcct           string            `json:"aws_acct,omitempty"`
	AwsZone           string            `json:"aws_zone,omitempty"`
}

// ActorInfo Actor 信息
type ActorInfo struct {
	ClientID      string            `json:"client_id"`
	RobotID       string            `json:"robot_id"`
	Exchange      string            `json:"exchange"`
	Version       string            `json:"version"`
	TenantID      uint32            `json:"tenant_id"`
	Status        string            `json:"status"`
	Balance       float64           `json:"balance"`
	RegisteredAt  time.Time         `json:"registered_at"`
	LastHeartbeat time.Time         `json:"last_heartbeat"`
	ServerInfo    *ServerStatusInfo `json:"server_info,omitempty"`
	IP            string            `json:"ip,omitempty"`
	InnerIP       string            `json:"inner_ip,omitempty"`
	Port          string            `json:"port,omitempty"`
	MachineID     string            `json:"machine_id,omitempty"`
	Nickname      string            `json:"nickname,omitempty"`
}

// RobotInfo 机器人信息
type RobotInfo struct {
	RobotID  string            `json:"robot_id"`
	Name     string            `json:"name"`
	Exchange string            `json:"exchange"`
	Status   string            `json:"status"`
	Config   map[string]string `json:"config,omitempty"`
}

// ==================== 命令载荷定义 ====================

// InitRequest 初始化请求
type InitRequest struct {
	ClientType string `json:"client_type"`
	Version    string `json:"version"`
	TenantID   uint32 `json:"tenant_id"`
	Token      string `json:"token"`
}

// InitResponse 初始化响应
type InitResponse struct {
	ClientID      string `json:"client_id"`
	ServerVersion string `json:"server_version"`
	ServerTime    int64  `json:"server_time"`
}

// InitCmd 初始化命令
type InitCmd struct {
	Request  *InitRequest  `json:"request,omitempty"`
	Response *InitResponse `json:"response,omitempty"`
}

// EchoRequest 心跳请求
type EchoRequest struct {
	Message    string `json:"message"`
	ClientTime int64  `json:"client_time"`
}

// EchoResponse 心跳响应
type EchoResponse struct {
	Message    string `json:"message"`
	ServerTime int64  `json:"server_time"`
}

// EchoCmd 心跳命令
type EchoCmd struct {
	Request  *EchoRequest  `json:"request,omitempty"`
	Response *EchoResponse `json:"response,omitempty"`
}

// NotifyRequest 通知请求
type NotifyRequest struct {
	Topic    string            `json:"topic"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// NotifyCmd 通知命令
type NotifyCmd struct {
	Request *NotifyRequest `json:"request,omitempty"`
}

// ResyncRequest 重新同步请求
type ResyncRequest struct {
	LastSeq uint64 `json:"last_seq"`
}

// ResyncCmd 重新同步命令
type ResyncCmd struct {
	Request *ResyncRequest `json:"request,omitempty"`
}

// ActorRegisterRequest Actor 注册请求
type ActorRegisterRequest struct {
	RobotID   string `json:"robot_id"`
	Exchange  string `json:"exchange"`
	Version   string `json:"version"`
	TenantID  uint32 `json:"tenant_id"`
	IP        string `json:"ip,omitempty"`
	InnerIP   string `json:"inner_ip,omitempty"`
	Port      string `json:"port,omitempty"`
	MachineID string `json:"machine_id,omitempty"`
	Nickname  string `json:"nickname,omitempty"`
}

// ActorRegisterResponse Actor 注册响应
type ActorRegisterResponse struct {
	Registered bool   `json:"registered"`
	ClientID   string `json:"client_id"`
}

// ActorRegisterCmd Actor 注册命令
type ActorRegisterCmd struct {
	Request  *ActorRegisterRequest  `json:"request,omitempty"`
	Response *ActorRegisterResponse `json:"response,omitempty"`
}

// ActorUnregisterRequest Actor 注销请求
type ActorUnregisterRequest struct {
	RobotID string `json:"robot_id"`
	Reason  string `json:"reason,omitempty"`
}

// ActorUnregisterResponse Actor 注销响应
type ActorUnregisterResponse struct {
	Success bool `json:"success"`
}

// ActorUnregisterCmd Actor 注销命令
type ActorUnregisterCmd struct {
	Request  *ActorUnregisterRequest  `json:"request,omitempty"`
	Response *ActorUnregisterResponse `json:"response,omitempty"`
}

// ActorHeartbeatRequest Actor 心跳请求
type ActorHeartbeatRequest struct {
	RobotID    string `json:"robot_id"`
	ClientTime int64  `json:"client_time"`
}

// ActorHeartbeatResponse Actor 心跳响应
type ActorHeartbeatResponse struct {
	ServerTime int64 `json:"server_time"`
}

// ActorHeartbeatCmd Actor 心跳命令
type ActorHeartbeatCmd struct {
	Request  *ActorHeartbeatRequest  `json:"request,omitempty"`
	Response *ActorHeartbeatResponse `json:"response,omitempty"`
}

// ActorStatusRequest Actor 状态请求
type ActorStatusRequest struct {
	RobotID    string            `json:"robot_id"`
	Status     string            `json:"status"`
	Balance    float64           `json:"balance"`
	ServerInfo *ServerStatusInfo `json:"server_info,omitempty"`
}

// ActorStatusResponse Actor 状态响应
type ActorStatusResponse struct {
	Acknowledged bool `json:"acknowledged"`
}

// ActorStatusCmd Actor 状态命令
type ActorStatusCmd struct {
	Request  *ActorStatusRequest  `json:"request,omitempty"`
	Response *ActorStatusResponse `json:"response,omitempty"`
}

// ActorListRequest Actor 列表请求
type ActorListRequest struct {
	TenantID uint32 `json:"tenant_id,omitempty"`
	Status   string `json:"status,omitempty"`
	Page     int32  `json:"page,omitempty"`
	PageSize int32  `json:"page_size,omitempty"`
}

// ActorListResponse Actor 列表响应
type ActorListResponse struct {
	Actors []*ActorInfo `json:"actors"`
	Total  int32        `json:"total"`
}

// ActorListCmd Actor 列表命令
type ActorListCmd struct {
	Request  *ActorListRequest  `json:"request,omitempty"`
	Response *ActorListResponse `json:"response,omitempty"`
}

// RobotStartRequest Robot 启动请求
type RobotStartRequest struct {
	RobotID string            `json:"robot_id"`
	Config  map[string]string `json:"config,omitempty"`
}

// RobotStartResponse Robot 启动响应
type RobotStartResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// RobotStartCmd Robot 启动命令
type RobotStartCmd struct {
	Request  *RobotStartRequest  `json:"request,omitempty"`
	Response *RobotStartResponse `json:"response,omitempty"`
}

// RobotStopRequest Robot 停止请求
type RobotStopRequest struct {
	RobotID  string `json:"robot_id"`
	Graceful bool   `json:"graceful"`
	Reason   string `json:"reason,omitempty"`
}

// RobotStopResponse Robot 停止响应
type RobotStopResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// RobotStopCmd Robot 停止命令
type RobotStopCmd struct {
	Request  *RobotStopRequest  `json:"request,omitempty"`
	Response *RobotStopResponse `json:"response,omitempty"`
}

// RobotConfigRequest Robot 配置请求
type RobotConfigRequest struct {
	RobotID string            `json:"robot_id"`
	Config  map[string]string `json:"config"`
}

// RobotConfigResponse Robot 配置响应
type RobotConfigResponse struct {
	Success       bool              `json:"success"`
	CurrentConfig map[string]string `json:"current_config,omitempty"`
}

// RobotConfigCmd Robot 配置命令
type RobotConfigCmd struct {
	Request  *RobotConfigRequest  `json:"request,omitempty"`
	Response *RobotConfigResponse `json:"response,omitempty"`
}

// RobotCommandRequest Robot 通用命令请求
type RobotCommandRequest struct {
	RobotID   string `json:"robot_id"`
	Action    string `json:"action"`
	Payload   []byte `json:"payload,omitempty"`
	TimeoutMs int32  `json:"timeout_ms,omitempty"`
}

// RobotCommandResponse Robot 通用命令响应
type RobotCommandResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Result  []byte `json:"result,omitempty"`
}

// RobotCommandCmd Robot 通用命令
type RobotCommandCmd struct {
	Request  *RobotCommandRequest  `json:"request,omitempty"`
	Response *RobotCommandResponse `json:"response,omitempty"`
}

// RobotResultRequest Robot 命令结果请求
type RobotResultRequest struct {
	RequestID string `json:"request_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	Result    []byte `json:"result,omitempty"`
}

// RobotResultCmd Robot 命令结果
type RobotResultCmd struct {
	Request *RobotResultRequest `json:"request,omitempty"`
}

// ServerSyncRequest 服务器同步请求
type ServerSyncRequest struct {
	RobotID    string            `json:"robot_id"`
	ServerInfo *ServerStatusInfo `json:"server_info"`
	IP         string            `json:"ip,omitempty"`
	InnerIP    string            `json:"inner_ip,omitempty"`
	Port       string            `json:"port,omitempty"`
	MachineID  string            `json:"machine_id,omitempty"`
	Nickname   string            `json:"nickname,omitempty"`
}

// ServerSyncResponse 服务器同步响应
type ServerSyncResponse struct {
	Success bool `json:"success"`
}

// ServerSyncCmd 服务器同步命令
type ServerSyncCmd struct {
	Request  *ServerSyncRequest  `json:"request,omitempty"`
	Response *ServerSyncResponse `json:"response,omitempty"`
}

// ServerStatusRequest 服务器状态请求
type ServerStatusRequest struct {
	RobotID string `json:"robot_id"`
}

// ServerStatusResponse 服务器状态响应
type ServerStatusResponse struct {
	ServerInfo *ServerStatusInfo `json:"server_info"`
}

// ServerStatusCmd 服务器状态命令
type ServerStatusCmd struct {
	Request  *ServerStatusRequest  `json:"request,omitempty"`
	Response *ServerStatusResponse `json:"response,omitempty"`
}

// AlertSendRequest 发送告警请求
type AlertSendRequest struct {
	RobotID  string            `json:"robot_id"`
	Level    AlertLevel        `json:"level"`
	Title    string            `json:"title"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// AlertSendResponse 发送告警响应
type AlertSendResponse struct {
	AlertID string `json:"alert_id"`
	Success bool   `json:"success"`
}

// AlertSendCmd 发送告警命令
type AlertSendCmd struct {
	Request  *AlertSendRequest  `json:"request,omitempty"`
	Response *AlertSendResponse `json:"response,omitempty"`
}

// AlertAckRequest 确认告警请求
type AlertAckRequest struct {
	AlertID string `json:"alert_id"`
	AckBy   string `json:"ack_by"`
	Comment string `json:"comment,omitempty"`
}

// AlertAckResponse 确认告警响应
type AlertAckResponse struct {
	Success bool `json:"success"`
}

// AlertAckCmd 确认告警命令
type AlertAckCmd struct {
	Request  *AlertAckRequest  `json:"request,omitempty"`
	Response *AlertAckResponse `json:"response,omitempty"`
}

// UserKickRequest 踢出用户请求
type UserKickRequest struct {
	UserID string `json:"user_id"`
	Reason string `json:"reason"`
}

// UserKickResponse 踢出用户响应
type UserKickResponse struct {
	Success bool `json:"success"`
}

// UserKickCmd 踢出用户命令
type UserKickCmd struct {
	Request  *UserKickRequest  `json:"request,omitempty"`
	Response *UserKickResponse `json:"response,omitempty"`
}

// UserBroadcastRequest 广播消息请求
type UserBroadcastRequest struct {
	UserIDs  []string          `json:"user_ids,omitempty"`
	Topic    string            `json:"topic"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// UserBroadcastResponse 广播消息响应
type UserBroadcastResponse struct {
	SentCount int32 `json:"sent_count"`
}

// UserBroadcastCmd 广播消息命令
type UserBroadcastCmd struct {
	Request  *UserBroadcastRequest  `json:"request,omitempty"`
	Response *UserBroadcastResponse `json:"response,omitempty"`
}

// RobotSyncRequest 机器人同步请求
type RobotSyncRequest struct {
	Robots []*RobotInfo `json:"robots"`
}

// RobotSyncResponse 机器人同步响应
type RobotSyncResponse struct {
	Success     bool  `json:"success"`
	SyncedCount int32 `json:"synced_count"`
}

// RobotSyncCmd 机器人同步命令
type RobotSyncCmd struct {
	Request  *RobotSyncRequest  `json:"request,omitempty"`
	Response *RobotSyncResponse `json:"response,omitempty"`
}

// ==================== 辅助函数 ====================

// NewCommand 创建新命令
func NewCommand(cmdType CommandType) *Command {
	return &Command{
		Type:      cmdType,
		Timestamp: time.Now(),
	}
}

// NewCommandWithSeq 创建带序列号的命令
func NewCommandWithSeq(cmdType CommandType, seq uint64) *Command {
	return &Command{
		Type:      cmdType,
		Seq:       seq,
		Timestamp: time.Now(),
	}
}

// NewCommandWithRequestID 创建带请求ID的命令
func NewCommandWithRequestID(cmdType CommandType, requestID string) *Command {
	return &Command{
		Type:      cmdType,
		RequestID: requestID,
		Timestamp: time.Now(),
	}
}

// NewErrorCommand 创建错误命令
func NewErrorCommand(code int32, message string) *Command {
	return &Command{
		Type:      CommandTypeError,
		Timestamp: time.Now(),
		Error: &ErrorMessage{
			Code:    code,
			Message: message,
		},
	}
}

// SetError 设置错误信息
func (c *Command) SetError(code int32, message string, details string) {
	c.Error = &ErrorMessage{
		Code:    code,
		Message: message,
		Details: details,
	}
}

// AddEvent 添加事件
func (c *Command) AddEvent(eventType EventType, data map[string]string) {
	c.Events = append(c.Events, Event{
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      data,
	})
}

// GetTypeString 获取命令类型字符串
func (c *Command) GetTypeString() string {
	if s, ok := CommandTypeToString[c.Type]; ok {
		return s
	}
	return "unknown"
}
