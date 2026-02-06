package backend

import (
	"encoding/json"
	"sync/atomic"
	"time"

	websocketpb "actor/api/gen/go/websocket/service/v1"
)

// CommandType 命令类型枚举
type CommandType = websocketpb.CommandType

// CommandType 常量
const (
	CommandTypeUnknown         = websocketpb.CommandType_COMMAND_TYPE_UNKNOWN
	CommandTypeInit            = websocketpb.CommandType_COMMAND_TYPE_INIT
	CommandTypeEcho            = websocketpb.CommandType_COMMAND_TYPE_ECHO
	CommandTypeNotify          = websocketpb.CommandType_COMMAND_TYPE_NOTIFY
	CommandTypeResync          = websocketpb.CommandType_COMMAND_TYPE_RESYNC
	CommandTypeError           = websocketpb.CommandType_COMMAND_TYPE_ERROR
	CommandTypeActorRegister   = websocketpb.CommandType_COMMAND_TYPE_ACTOR_REGISTER
	CommandTypeActorUnregister = websocketpb.CommandType_COMMAND_TYPE_ACTOR_UNREGISTER
	CommandTypeActorHeartbeat  = websocketpb.CommandType_COMMAND_TYPE_ACTOR_HEARTBEAT
	CommandTypeActorStatus     = websocketpb.CommandType_COMMAND_TYPE_ACTOR_STATUS
	CommandTypeActorList       = websocketpb.CommandType_COMMAND_TYPE_ACTOR_LIST
	CommandTypeRobotStart      = websocketpb.CommandType_COMMAND_TYPE_ROBOT_START
	CommandTypeRobotStop       = websocketpb.CommandType_COMMAND_TYPE_ROBOT_STOP
	CommandTypeRobotConfig     = websocketpb.CommandType_COMMAND_TYPE_ROBOT_CONFIG
	CommandTypeRobotCommand    = websocketpb.CommandType_COMMAND_TYPE_ROBOT_COMMAND
	CommandTypeRobotResult     = websocketpb.CommandType_COMMAND_TYPE_ROBOT_RESULT
	CommandTypeServerSync      = websocketpb.CommandType_COMMAND_TYPE_SERVER_SYNC
	CommandTypeServerStatus    = websocketpb.CommandType_COMMAND_TYPE_SERVER_STATUS
	CommandTypeAlertSend       = websocketpb.CommandType_COMMAND_TYPE_ALERT_SEND
	CommandTypeAlertAck        = websocketpb.CommandType_COMMAND_TYPE_ALERT_ACK
	CommandTypeUserKick        = websocketpb.CommandType_COMMAND_TYPE_USER_KICK
	CommandTypeUserBroadcast   = websocketpb.CommandType_COMMAND_TYPE_USER_BROADCAST
	CommandTypeRobotSync       = websocketpb.CommandType_COMMAND_TYPE_ROBOT_SYNC
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

// ErrorMessage 错误信息
type ErrorMessage struct {
	Code    int32  `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// Command 主命令消息
type Command struct {
	Type      CommandType   `json:"type"`
	Seq       uint64        `json:"seq"`
	RequestID string        `json:"request_id,omitempty"`
	Error     *ErrorMessage `json:"error,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	Payload   interface{}   `json:"payload,omitempty"`
}

// GetTypeString 获取命令类型字符串
func (c *Command) GetTypeString() string {
	if s, ok := CommandTypeToString[c.Type]; ok {
		return s
	}
	return "unknown"
}

// 全局序列号生成器
var globalSeq uint64

// NewCommand 创建新命令
func NewCommand(cmdType CommandType) *Command {
	return &Command{
		Type:      cmdType,
		Seq:       atomic.AddUint64(&globalSeq, 1),
		Timestamp: time.Now(),
	}
}

// NewCommandWithRequestID 创建带请求ID的命令
func NewCommandWithRequestID(cmdType CommandType, requestID string) *Command {
	return &Command{
		Type:      cmdType,
		Seq:       atomic.AddUint64(&globalSeq, 1),
		RequestID: requestID,
		Timestamp: time.Now(),
	}
}

// ToJSON 转换为 JSON
func (c *Command) ToJSON() ([]byte, error) {
	return json.Marshal(c)
}

// ==================== 命令载荷定义 ====================

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
// 必须字段: IP, InnerIP, Port, Nickname
type ServerSyncRequest struct {
	RobotID    string            `json:"robot_id"`
	IP         string            `json:"ip"`        // 外网IP (必须)
	InnerIP    string            `json:"inner_ip"`  // 内网IP (必须)
	Port       string            `json:"port"`      // 端口 (必须)
	Nickname   string            `json:"nickname"`  // 托管者昵称 (必须)
	MachineID  string            `json:"machine_id,omitempty"`
	ServerInfo *ServerStatusInfo `json:"server_info,omitempty"` // 服务器状态信息
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

// ==================== 辅助函数 ====================

// NewRegisterCommand 创建注册命令
func NewRegisterCommand(robotID, exchange, version string, tenantID uint32) *Command {
	cmd := NewCommand(CommandTypeActorRegister)
	cmd.Payload = &ActorRegisterCmd{
		Request: &ActorRegisterRequest{
			RobotID:  robotID,
			Exchange: exchange,
			Version:  version,
			TenantID: tenantID,
		},
	}
	return cmd
}

// NewUnregisterCommand 创建注销命令
func NewUnregisterCommand(robotID, reason string) *Command {
	cmd := NewCommand(CommandTypeActorUnregister)
	cmd.Payload = &ActorUnregisterCmd{
		Request: &ActorUnregisterRequest{
			RobotID: robotID,
			Reason:  reason,
		},
	}
	return cmd
}

// NewHeartbeatCommand 创建心跳命令
func NewHeartbeatCommand(robotID string) *Command {
	cmd := NewCommand(CommandTypeActorHeartbeat)
	cmd.Payload = &ActorHeartbeatCmd{
		Request: &ActorHeartbeatRequest{
			RobotID:    robotID,
			ClientTime: time.Now().UnixMilli(),
		},
	}
	return cmd
}

// NewStatusCommand 创建状态更新命令
func NewStatusCommand(robotID, status string, balance float64) *Command {
	cmd := NewCommand(CommandTypeActorStatus)
	cmd.Payload = &ActorStatusCmd{
		Request: &ActorStatusRequest{
			RobotID: robotID,
			Status:  status,
			Balance: balance,
		},
	}
	return cmd
}

// NewServerSyncCommand 创建服务器同步命令
func NewServerSyncCommand(data *ServerSyncData) *Command {
	cmd := NewCommand(CommandTypeServerSync)

	var serverInfo *ServerStatusInfo
	if data.ServerInfo != nil {
		serverInfo = &ServerStatusInfo{
			CPU:               data.ServerInfo.CPU,
			IPPool:            data.ServerInfo.IPPool,
			Mem:               data.ServerInfo.Mem,
			MemPct:            data.ServerInfo.MemPct,
			DiskPct:           data.ServerInfo.DiskPct,
			TaskNum:           data.ServerInfo.TaskNum,
			StraVersion:       data.ServerInfo.StraVersion,
			StraVersionDetail: data.ServerInfo.StraVersionDetail,
			AwsAcct:           data.ServerInfo.AwsAcct,
			AwsZone:           data.ServerInfo.AwsZone,
		}
	}

	cmd.Payload = &ServerSyncCmd{
		Request: &ServerSyncRequest{
			RobotID:    data.RobotID,
			IP:         data.IP,
			InnerIP:    data.InnerIP,
			Port:       data.Port,
			MachineID:  data.MachineID,
			Nickname:   data.Nickname,
			ServerInfo: serverInfo,
		},
	}
	return cmd
}

// NewRobotResultCommand 创建命令结果命令
func NewRobotResultCommand(requestID string, success bool, result []byte, errMsg string) *Command {
	cmd := NewCommandWithRequestID(CommandTypeRobotResult, requestID)
	cmd.Payload = &RobotResultCmd{
		Request: &RobotResultRequest{
			RequestID: requestID,
			Success:   success,
			Error:     errMsg,
			Result:    result,
		},
	}
	return cmd
}

// ==================== 兼容旧协议的类型定义 ====================

// ServerInfo 服务器状态信息 (兼容旧协议)
type ServerInfo = ServerStatusInfo

// ServerSyncData 服务器同步数据 (兼容旧协议)
// 必须字段: IP, InnerIP, Port, Nickname
type ServerSyncData struct {
	RobotID    string      `json:"robot_id"`
	IP         string      `json:"ip"`        // 外网IP (必须)
	InnerIP    string      `json:"inner_ip"`  // 内网IP (必须)
	Port       string      `json:"port"`      // 端口 (必须)
	Nickname   string      `json:"nickname"`  // 托管者昵称 (必须)
	MachineID  string      `json:"machine_id,omitempty"`
	ServerInfo *ServerInfo `json:"server_info,omitempty"` // 服务器状态信息
}

// CommandHandler handles commands from backend
type CommandHandler interface {
	// HandleCommand processes a command and returns a result
	HandleCommand(cmd *IncomingCommand) *CommandResult
}

// IncomingCommand 来自后端的命令
type IncomingCommand struct {
	Type      CommandType
	RequestID string
	RobotID   string
	Action    string
	Data      map[string]interface{}
	Payload   []byte
	TimeoutMs int32
}

// CommandResult represents the result of a command execution
type CommandResult struct {
	RequestID string
	Success   bool
	Error     string
	Result    interface{}
}
