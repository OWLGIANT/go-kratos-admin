package ws

type MsgHandle func(msg []byte, ts int64)

// action handler
type ConnectedHandle func(wsURL string)
type DisconnectedHandle func(wsURL string)
type ErrorHandle func(wsURL string, err error)
