package protocol

import (
	"encoding/json"
	"fmt"
)

// Codec 协议编解码器接口
type Codec interface {
	// Encode 编码命令为字节
	Encode(cmd *Command) ([]byte, error)
	// Decode 解码字节为命令
	Decode(data []byte) (*Command, error)
	// ContentType 返回内容类型
	ContentType() ContentType
}

// JSONCodec JSON 编解码器
type JSONCodec struct{}

// NewJSONCodec 创建 JSON 编解码器
func NewJSONCodec() *JSONCodec {
	return &JSONCodec{}
}

// ContentType 返回内容类型
func (c *JSONCodec) ContentType() ContentType {
	return ContentTypeJSON
}

// Encode 编码命令为 JSON
func (c *JSONCodec) Encode(cmd *Command) ([]byte, error) {
	return json.Marshal(cmd)
}

// Decode 解码 JSON 为命令
func (c *JSONCodec) Decode(data []byte) (*Command, error) {
	// 首先解析基础结构
	var rawCmd struct {
		Type      CommandType     `json:"type"`
		Seq       uint64          `json:"seq"`
		RequestID string          `json:"request_id,omitempty"`
		Error     *ErrorMessage   `json:"error,omitempty"`
		Events    []Event         `json:"events,omitempty"`
		Timestamp string          `json:"timestamp"`
		Payload   json.RawMessage `json:"payload,omitempty"`
	}

	if err := json.Unmarshal(data, &rawCmd); err != nil {
		return nil, fmt.Errorf("failed to unmarshal command: %w", err)
	}

	cmd := &Command{
		Type:      rawCmd.Type,
		Seq:       rawCmd.Seq,
		RequestID: rawCmd.RequestID,
		Error:     rawCmd.Error,
		Events:    rawCmd.Events,
	}

	// 根据命令类型解析 payload
	if len(rawCmd.Payload) > 0 {
		payload, err := c.decodePayload(rawCmd.Type, rawCmd.Payload)
		if err != nil {
			return nil, fmt.Errorf("failed to decode payload: %w", err)
		}
		cmd.Payload = payload
	}

	return cmd, nil
}

// decodePayload 根据命令类型解码 payload
func (c *JSONCodec) decodePayload(cmdType CommandType, data json.RawMessage) (interface{}, error) {
	switch cmdType {
	case CommandTypeInit:
		var payload InitCmd
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &payload, nil

	case CommandTypeEcho:
		var payload EchoCmd
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &payload, nil

	case CommandTypeNotify:
		var payload NotifyCmd
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &payload, nil

	case CommandTypeResync:
		var payload ResyncCmd
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &payload, nil

	case CommandTypeActorRegister:
		var payload ActorRegisterCmd
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &payload, nil

	case CommandTypeActorUnregister:
		var payload ActorUnregisterCmd
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &payload, nil

	case CommandTypeActorHeartbeat:
		var payload ActorHeartbeatCmd
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &payload, nil

	case CommandTypeActorStatus:
		var payload ActorStatusCmd
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &payload, nil

	case CommandTypeActorList:
		var payload ActorListCmd
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &payload, nil

	case CommandTypeRobotStart:
		var payload RobotStartCmd
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &payload, nil

	case CommandTypeRobotStop:
		var payload RobotStopCmd
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &payload, nil

	case CommandTypeRobotConfig:
		var payload RobotConfigCmd
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &payload, nil

	case CommandTypeRobotCommand:
		var payload RobotCommandCmd
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &payload, nil

	case CommandTypeRobotResult:
		var payload RobotResultCmd
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &payload, nil

	case CommandTypeServerSync:
		var payload ServerSyncCmd
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &payload, nil

	case CommandTypeServerStatus:
		var payload ServerStatusCmd
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &payload, nil

	case CommandTypeAlertSend:
		var payload AlertSendCmd
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &payload, nil

	case CommandTypeAlertAck:
		var payload AlertAckCmd
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &payload, nil

	case CommandTypeUserKick:
		var payload UserKickCmd
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &payload, nil

	case CommandTypeUserBroadcast:
		var payload UserBroadcastCmd
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &payload, nil

	case CommandTypeRobotSync:
		var payload RobotSyncCmd
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return &payload, nil

	default:
		// 未知类型，返回原始 JSON
		var payload map[string]interface{}
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	}
}

// LegacyCodec 兼容旧协议的编解码器
type LegacyCodec struct {
	jsonCodec *JSONCodec
}

// NewLegacyCodec 创建兼容旧协议的编解码器
func NewLegacyCodec() *LegacyCodec {
	return &LegacyCodec{
		jsonCodec: NewJSONCodec(),
	}
}

// ContentType 返回内容类型
func (c *LegacyCodec) ContentType() ContentType {
	return ContentTypeJSON
}

// LegacyMessage 旧协议消息格式
type LegacyMessage struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// LegacyData 旧协议数据格式
type LegacyData struct {
	ProtocolNumber int             `json:"protocol_number"`
	MessageBody    json.RawMessage `json:"message_body"`
}

// 旧协议号映射
var legacyProtocolToCommandType = map[int]CommandType{
	99999: CommandTypeEcho,
	10001: CommandTypeUserKick,
	10002: CommandTypeAlertSend,
	10003: CommandTypeRobotSync,
	10011: CommandTypeActorRegister,
	10012: CommandTypeActorUnregister,
	10013: CommandTypeActorHeartbeat,
	10014: CommandTypeActorStatus,
	10015: CommandTypeActorList,
	10021: CommandTypeRobotStart,
	10022: CommandTypeRobotStop,
	10023: CommandTypeRobotConfig,
	10024: CommandTypeRobotCommand,
	10025: CommandTypeRobotResult,
	10031: CommandTypeServerSync,
	10032: CommandTypeServerStatus,
}

var commandTypeToLegacyProtocol = map[CommandType]int{
	CommandTypeEcho:            99999,
	CommandTypeUserKick:        10001,
	CommandTypeAlertSend:       10002,
	CommandTypeRobotSync:       10003,
	CommandTypeActorRegister:   10011,
	CommandTypeActorUnregister: 10012,
	CommandTypeActorHeartbeat:  10013,
	CommandTypeActorStatus:     10014,
	CommandTypeActorList:       10015,
	CommandTypeRobotStart:      10021,
	CommandTypeRobotStop:       10022,
	CommandTypeRobotConfig:     10023,
	CommandTypeRobotCommand:    10024,
	CommandTypeRobotResult:     10025,
	CommandTypeServerSync:      10031,
	CommandTypeServerStatus:    10032,
}

// Encode 编码命令为旧协议格式
func (c *LegacyCodec) Encode(cmd *Command) ([]byte, error) {
	protocolNumber, ok := commandTypeToLegacyProtocol[cmd.Type]
	if !ok {
		protocolNumber = int(cmd.Type)
	}

	// 编码 payload
	var messageBody json.RawMessage
	if cmd.Payload != nil {
		data, err := json.Marshal(cmd.Payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload: %w", err)
		}
		messageBody = data
	}

	// 构建旧协议响应
	code := 0
	msg := "success"
	if cmd.Error != nil {
		code = int(cmd.Error.Code)
		msg = cmd.Error.Message
	}

	legacyData := LegacyData{
		ProtocolNumber: protocolNumber,
		MessageBody:    messageBody,
	}

	dataBytes, err := json.Marshal(legacyData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal legacy data: %w", err)
	}

	legacyMsg := LegacyMessage{
		Code: code,
		Msg:  msg,
		Data: dataBytes,
	}

	return json.Marshal(legacyMsg)
}

// Decode 解码旧协议格式为命令
func (c *LegacyCodec) Decode(data []byte) (*Command, error) {
	var legacyMsg LegacyMessage
	if err := json.Unmarshal(data, &legacyMsg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal legacy message: %w", err)
	}

	var legacyData LegacyData
	if err := json.Unmarshal(legacyMsg.Data, &legacyData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal legacy data: %w", err)
	}

	// 转换协议号为命令类型
	cmdType, ok := legacyProtocolToCommandType[legacyData.ProtocolNumber]
	if !ok {
		cmdType = CommandType(legacyData.ProtocolNumber)
	}

	cmd := &Command{
		Type: cmdType,
	}

	// 设置错误信息
	if legacyMsg.Code != 0 {
		cmd.Error = &ErrorMessage{
			Code:    int32(legacyMsg.Code),
			Message: legacyMsg.Msg,
		}
	}

	// 解码 payload
	if len(legacyData.MessageBody) > 0 {
		payload, err := c.jsonCodec.decodePayload(cmdType, legacyData.MessageBody)
		if err != nil {
			// 如果解析失败，保留原始数据
			var rawPayload map[string]interface{}
			if jsonErr := json.Unmarshal(legacyData.MessageBody, &rawPayload); jsonErr == nil {
				cmd.Payload = rawPayload
			}
		} else {
			cmd.Payload = payload
		}
	}

	return cmd, nil
}

// AutoCodec 自动检测协议格式的编解码器
type AutoCodec struct {
	jsonCodec   *JSONCodec
	legacyCodec *LegacyCodec
}

// NewAutoCodec 创建自动检测协议格式的编解码器
func NewAutoCodec() *AutoCodec {
	return &AutoCodec{
		jsonCodec:   NewJSONCodec(),
		legacyCodec: NewLegacyCodec(),
	}
}

// ContentType 返回内容类型
func (c *AutoCodec) ContentType() ContentType {
	return ContentTypeJSON
}

// Encode 编码命令（默认使用新协议）
func (c *AutoCodec) Encode(cmd *Command) ([]byte, error) {
	return c.jsonCodec.Encode(cmd)
}

// EncodeWithCodec 使用指定编解码器编码
func (c *AutoCodec) EncodeWithCodec(cmd *Command, codec Codec) ([]byte, error) {
	return codec.Encode(cmd)
}

// Decode 自动检测协议格式并解码
func (c *AutoCodec) Decode(data []byte) (*Command, error) {
	// 尝试检测协议格式
	if c.isNewProtocol(data) {
		return c.jsonCodec.Decode(data)
	}

	if c.isLegacyProtocol(data) {
		return c.legacyCodec.Decode(data)
	}

	return nil, fmt.Errorf("unknown protocol format")
}

// isNewProtocol 检测是否为新协议格式
func (c *AutoCodec) isNewProtocol(data []byte) bool {
	var probe struct {
		Type CommandType `json:"type"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return false
	}
	return probe.Type > 0
}

// isLegacyProtocol 检测是否为旧协议格式
func (c *AutoCodec) isLegacyProtocol(data []byte) bool {
	var probe struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return false
	}

	if len(probe.Data) == 0 {
		return false
	}

	var dataProbe struct {
		ProtocolNumber int `json:"protocol_number"`
	}
	if err := json.Unmarshal(probe.Data, &dataProbe); err != nil {
		return false
	}

	return dataProbe.ProtocolNumber > 0
}

// GetJSONCodec 获取 JSON 编解码器
func (c *AutoCodec) GetJSONCodec() *JSONCodec {
	return c.jsonCodec
}

// GetLegacyCodec 获取旧协议编解码器
func (c *AutoCodec) GetLegacyCodec() *LegacyCodec {
	return c.legacyCodec
}
