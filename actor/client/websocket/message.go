package websocket

import (
	"encoding/json"
	"time"
)

// MessageType 消息类型
type MessageType string

const (
	MessageTypeText   MessageType = "text"
	MessageTypeBinary MessageType = "binary"
	MessageTypePing   MessageType = "ping"
	MessageTypePong   MessageType = "pong"
)

// Message 通用消息结构
type Message struct {
	Type      MessageType            `json:"type"`
	Action    string                 `json:"action,omitempty"`
	Data      interface{}            `json:"data,omitempty"`
	Timestamp int64                  `json:"timestamp"`
	RequestID string                 `json:"request_id,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// NewMessage 创建新消息
func NewMessage(action string, data interface{}) *Message {
	return &Message{
		Type:      MessageTypeText,
		Action:    action,
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
	}
}

// ToJSON 转换为 JSON
func (m *Message) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// FromJSON 从 JSON 解析
func (m *Message) FromJSON(data []byte) error {
	return json.Unmarshal(data, m)
}

// SetRequestID 设置请求ID
func (m *Message) SetRequestID(id string) *Message {
	m.RequestID = id
	return m
}

// SetMetadata 设置元数据
func (m *Message) SetMetadata(key string, value interface{}) *Message {
	if m.Metadata == nil {
		m.Metadata = make(map[string]interface{})
	}
	m.Metadata[key] = value
	return m
}

// Response 响应消息结构
type Response struct {
	Success   bool        `json:"success"`
	Code      int         `json:"code"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
	Timestamp int64       `json:"timestamp"`
}

// NewResponse 创建新响应
func NewResponse(success bool, code int, message string, data interface{}) *Response {
	return &Response{
		Success:   success,
		Code:      code,
		Message:   message,
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
	}
}

// ToJSON 转换为 JSON
func (r *Response) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// FromJSON 从 JSON 解析
func (r *Response) FromJSON(data []byte) error {
	return json.Unmarshal(data, r)
}

// SetRequestID 设置请求ID
func (r *Response) SetRequestID(id string) *Response {
	r.RequestID = id
	return r
}
