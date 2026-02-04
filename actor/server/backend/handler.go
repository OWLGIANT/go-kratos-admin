package backend

// DefaultHandler provides a default implementation of CommandHandler
type DefaultHandler struct {
	startFunc   func(data map[string]interface{}) error
	stopFunc    func(data map[string]interface{}) error
	statusFunc  func() (interface{}, error)
	configFunc  func(data map[string]interface{}) error
	createFunc  func(data map[string]interface{}) error
	deleteFunc  func(data map[string]interface{}) error
	commandFunc func(action string, payload []byte) (interface{}, error)
}

// NewDefaultHandler creates a new default handler
func NewDefaultHandler() *DefaultHandler {
	return &DefaultHandler{}
}

// OnStart sets the handler for start command
func (h *DefaultHandler) OnStart(fn func(data map[string]interface{}) error) *DefaultHandler {
	h.startFunc = fn
	return h
}

// OnStop sets the handler for stop command
func (h *DefaultHandler) OnStop(fn func(data map[string]interface{}) error) *DefaultHandler {
	h.stopFunc = fn
	return h
}

// OnStatus sets the handler for status command
func (h *DefaultHandler) OnStatus(fn func() (interface{}, error)) *DefaultHandler {
	h.statusFunc = fn
	return h
}

// OnConfig sets the handler for config command
func (h *DefaultHandler) OnConfig(fn func(data map[string]interface{}) error) *DefaultHandler {
	h.configFunc = fn
	return h
}

// OnCommand sets the handler for generic command
func (h *DefaultHandler) OnCommand(fn func(action string, payload []byte) (interface{}, error)) *DefaultHandler {
	h.commandFunc = fn
	return h
}

// OnCreate sets the handler for create command
func (h *DefaultHandler) OnCreate(fn func(data map[string]interface{}) error) *DefaultHandler {
	h.createFunc = fn
	return h
}

// OnDelete sets the handler for delete command
func (h *DefaultHandler) OnDelete(fn func(data map[string]interface{}) error) *DefaultHandler {
	h.deleteFunc = fn
	return h
}

// HandleCommand implements CommandHandler interface
func (h *DefaultHandler) HandleCommand(cmd *IncomingCommand) *CommandResult {
	result := &CommandResult{
		RequestID: cmd.RequestID,
		Success:   true,
	}

	var err error

	switch cmd.Type {
	case CommandTypeRobotStart:
		if h.startFunc != nil {
			err = h.startFunc(cmd.Data)
		}
	case CommandTypeRobotStop:
		if h.stopFunc != nil {
			err = h.stopFunc(cmd.Data)
		}
	case CommandTypeRobotConfig:
		if h.configFunc != nil {
			err = h.configFunc(cmd.Data)
		}
	case CommandTypeRobotCommand:
		if h.commandFunc != nil {
			result.Result, err = h.commandFunc(cmd.Action, cmd.Payload)
		} else {
			// Fallback to action-based routing
			switch cmd.Action {
			case "robot.start":
				if h.startFunc != nil {
					err = h.startFunc(cmd.Data)
				}
			case "robot.stop":
				if h.stopFunc != nil {
					err = h.stopFunc(cmd.Data)
				}
			case "robot.status":
				if h.statusFunc != nil {
					result.Result, err = h.statusFunc()
				}
			case "robot.config":
				if h.configFunc != nil {
					err = h.configFunc(cmd.Data)
				}
			case "robot.create":
				if h.createFunc != nil {
					err = h.createFunc(cmd.Data)
				}
			case "robot.delete":
				if h.deleteFunc != nil {
					err = h.deleteFunc(cmd.Data)
				}
			default:
				result.Success = false
				result.Error = "unknown action: " + cmd.Action
				return result
			}
		}
	default:
		result.Success = false
		result.Error = "unknown command type"
		return result
	}

	if err != nil {
		result.Success = false
		result.Error = err.Error()
	}

	return result
}
