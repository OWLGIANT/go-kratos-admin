package protocol

import (
	"fmt"
)

// Adapter handles protocol detection and conversion
type Adapter struct {
	legacyAdapter *LegacyAdapter
	actorAdapter  *ActorAdapter
}

// NewAdapter creates a new protocol adapter
func NewAdapter() *Adapter {
	return &Adapter{
		legacyAdapter: NewLegacyAdapter(),
		actorAdapter:  NewActorAdapter(),
	}
}

// DetectAndConvert detects the protocol type and converts to unified message
func (a *Adapter) DetectAndConvert(data []byte) (*UnifiedMessage, ProtocolType, error) {
	// Try Actor protocol first (more specific fields)
	if IsActorProtocol(data) {
		msg, err := a.actorAdapter.ToUnified(data)
		if err != nil {
			return nil, "", fmt.Errorf("actor protocol conversion failed: %w", err)
		}
		return msg, ProtocolTypeActor, nil
	}

	// Try Legacy protocol
	if IsLegacyProtocol(data) {
		msg, err := a.legacyAdapter.ToUnified(data)
		if err != nil {
			return nil, "", fmt.Errorf("legacy protocol conversion failed: %w", err)
		}
		return msg, ProtocolTypeLegacy, nil
	}

	return nil, "", fmt.Errorf("unknown protocol format")
}

// ConvertResponse converts a unified response to the appropriate protocol format
func (a *Adapter) ConvertResponse(resp *UnifiedResponse, protocolType ProtocolType, action string) ([]byte, error) {
	switch protocolType {
	case ProtocolTypeActor:
		return a.actorAdapter.FromUnified(resp, action)
	case ProtocolTypeLegacy:
		return a.legacyAdapter.FromUnified(resp, action)
	default:
		return nil, fmt.Errorf("unsupported protocol type: %s", protocolType)
	}
}
