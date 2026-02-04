package helper

import (
	set "github.com/duke-git/lancet/v2/datastructure/set"
)

// 仅支持双向持仓模式的交易所 其他统一用单向持仓模式 可以最大化保证金利用
// 仅支持双向持仓模式的交易所 其他统一用单向持仓模式 可以最大化保证金利用
var posModeDualSet = set.NewSetFromSlice[BrokerName]([]BrokerName{
	BrokernameBitmartUsdtSwap,
	BrokernameAscendexUsdtSwap,
})

// GetPosMode 获取是否是单向持仓模式 如果true 说明是单向持仓 则需要合并双向仓位 如果是false 说明只支持双向持仓
func GetPosMode(ex BrokerName) bool {
	if posModeDualSet.Contain(ex) {
		return false // 双向持仓
	} else {
		return true // 单向持仓
	}
}

var posEventFasterSet = set.NewSetFromSlice[BrokerName]([]BrokerName{
	BrokernameBinanceUsdtSwapUm,
	BrokernameBinanceUsdtSwap,
	BrokernameBitgetUsdtSwap,
	BrokernameBitgetUsdtSwapV2,
	BrokernameBitmartUsdtSwap,
	BrokernameCoinexUsdtSwap,
	// BrokernameGateUsdtSwap,
	BrokernamePhemexUsdtSwap,
	BrokernameLbankUsdtSwap,
})

// GetTriggerSeq 获取成交信息推送顺序 如果true 说明是onPos更快 否则是onTrade更快
func GetTriggerSeq(ex BrokerName) bool {
	if posEventFasterSet.Contain(ex) {
		return true
	}
	return false
}
