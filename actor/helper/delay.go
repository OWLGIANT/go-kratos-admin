package helper

// DefaultDelayNewMs 默认下单延迟
type DefaultDelayNewMs int

const (
	DefaultDelayNewMsBinanceUsdtSwap DefaultDelayNewMs = 6
	DefaultDelayNewMsBitgetUsdtSwap  DefaultDelayNewMs = 30
)

// DefaultDelayCancelMs 默认撤单延迟
type DefaultDelayCancelMs int

const (
	DefaultDelayCancelMsBinanceUsdtSwap DefaultDelayNewMs = 6
	DefaultDelayCancelMsBitgetUsdtSwap  DefaultDelayNewMs = 30
)

// DefaultDelaySystemUs 默认系统延迟 不能大于 500us
const DefaultDelaySystemUs = 500
