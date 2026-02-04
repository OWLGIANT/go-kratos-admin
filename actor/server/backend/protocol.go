package backend

import (
	"encoding/json"
	"time"
)

// Message types
const (
	// Actor -> Backend
	ActionRegister      = "actor.register"
	ActionUnregister    = "actor.unregister"
	ActionStatusUpdate  = "actor.status_update"
	ActionHeartbeat     = "actor.heartbeat"
	ActionCommandResult = "actor.command_result"
	ActionServerSync    = "actor.server_sync"

	// Backend -> Actor
	ActionStart  = "actor.start"
	ActionStop   = "actor.stop"
	ActionStatus = "actor.status"
	ActionConfig = "actor.config"
	ActionCreate = "actor.create"
	ActionDelete = "actor.delete"
)

// Message represents a message sent to/from backend
type Message struct {
	Action    string                 `json:"action"`
	Data      map[string]interface{} `json:"data,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
	Timestamp int64                  `json:"timestamp,omitempty"`
}

// Response represents a response from backend
type Response struct {
	Success   bool                   `json:"success"`
	Code      int                    `json:"code"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
	Timestamp int64                  `json:"timestamp,omitempty"`
}

// Command represents a command from backend to actor
type Command struct {
	Action    string                 `json:"action"`
	Data      map[string]interface{} `json:"data,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
}

// CommandResult represents the result of a command execution
type CommandResult struct {
	RequestID string      `json:"request_id"`
	Success   bool        `json:"success"`
	Error     string      `json:"error,omitempty"`
	Result    interface{} `json:"result,omitempty"`
}

// RegisterData contains actor registration information
type RegisterData struct {
	RobotID   string `json:"robot_id"`
	Exchange  string `json:"exchange"`
	Version   string `json:"version"`
	TenantID  uint32 `json:"tenant_id,omitempty"`
}

// StatusData contains actor status information
type StatusData struct {
	RobotID   string   `json:"robot_id"`
	Status    string   `json:"status"` // running, stopped, error
	Balance   float64  `json:"balance,omitempty"`
	Positions []string `json:"positions,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// NewMessage creates a new message
func NewMessage(action string, data map[string]interface{}) *Message {
	return &Message{
		Action:    action,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}
}

// NewMessageWithRequestID creates a new message with request ID
func NewMessageWithRequestID(action string, data map[string]interface{}, requestID string) *Message {
	return &Message{
		Action:    action,
		Data:      data,
		RequestID: requestID,
		Timestamp: time.Now().Unix(),
	}
}

// ToJSON converts message to JSON bytes
func (m *Message) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// ParseMessage parses JSON bytes to message
func ParseMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// ParseResponse parses JSON bytes to response
func ParseResponse(data []byte) (*Response, error) {
	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// NewRegisterMessage creates a registration message
func NewRegisterMessage(robotID, exchange, version string, tenantID uint32) *Message {
	return NewMessage(ActionRegister, map[string]interface{}{
		"robot_id":  robotID,
		"exchange":  exchange,
		"version":   version,
		"tenant_id": tenantID,
	})
}

// NewStatusUpdateMessage creates a status update message
func NewStatusUpdateMessage(robotID, status string, balance float64) *Message {
	return NewMessage(ActionStatusUpdate, map[string]interface{}{
		"robot_id": robotID,
		"status":   status,
		"balance":  balance,
	})
}

// NewHeartbeatMessage creates a heartbeat message
func NewHeartbeatMessage(robotID string) *Message {
	return NewMessage(ActionHeartbeat, map[string]interface{}{
		"robot_id": robotID,
	})
}

// NewCommandResultMessage creates a command result message
func NewCommandResultMessage(requestID string, success bool, result interface{}, errMsg string) *Message {
	data := map[string]interface{}{
		"request_id": requestID,
		"success":    success,
	}
	if result != nil {
		data["result"] = result
	}
	if errMsg != "" {
		data["error"] = errMsg
	}
	return NewMessage(ActionCommandResult, data)
}

// ServerInfo 服务器状态信息
type ServerInfo struct {
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

// ServerSyncData 服务器同步数据
type ServerSyncData struct {
	RobotID    string      `json:"robot_id"`
	ServerInfo *ServerInfo `json:"server_info,omitempty"`
	IP         string      `json:"ip,omitempty"`
	InnerIP    string      `json:"inner_ip,omitempty"`
	Port       string      `json:"port,omitempty"`
	MachineID  string      `json:"machine_id,omitempty"`
	Nickname   string      `json:"nickname,omitempty"`
}

// NewServerSyncMessage creates a server sync message
func NewServerSyncMessage(data *ServerSyncData) *Message {
	msgData := map[string]interface{}{
		"robot_id": data.RobotID,
	}

	if data.ServerInfo != nil {
		serverInfo := map[string]interface{}{}
		if data.ServerInfo.CPU != "" {
			serverInfo["cpu"] = data.ServerInfo.CPU
		}
		if data.ServerInfo.IPPool != 0 {
			serverInfo["ip_pool"] = data.ServerInfo.IPPool
		}
		if data.ServerInfo.Mem != 0 {
			serverInfo["mem"] = data.ServerInfo.Mem
		}
		if data.ServerInfo.MemPct != "" {
			serverInfo["mem_pct"] = data.ServerInfo.MemPct
		}
		if data.ServerInfo.DiskPct != "" {
			serverInfo["disk_pct"] = data.ServerInfo.DiskPct
		}
		if data.ServerInfo.TaskNum != 0 {
			serverInfo["task_num"] = data.ServerInfo.TaskNum
		}
		serverInfo["stra_version"] = data.ServerInfo.StraVersion
		if data.ServerInfo.StraVersionDetail != nil {
			serverInfo["stra_version_detail"] = data.ServerInfo.StraVersionDetail
		}
		if data.ServerInfo.AwsAcct != "" {
			serverInfo["aws_acct"] = data.ServerInfo.AwsAcct
		}
		if data.ServerInfo.AwsZone != "" {
			serverInfo["aws_zone"] = data.ServerInfo.AwsZone
		}
		if len(serverInfo) > 0 {
			msgData["server_info"] = serverInfo
		}
	}

	if data.IP != "" {
		msgData["ip"] = data.IP
	}
	if data.InnerIP != "" {
		msgData["inner_ip"] = data.InnerIP
	}
	if data.Port != "" {
		msgData["port"] = data.Port
	}
	if data.MachineID != "" {
		msgData["machine_id"] = data.MachineID
	}
	if data.Nickname != "" {
		msgData["nickname"] = data.Nickname
	}

	return NewMessage(ActionServerSync, msgData)
}
