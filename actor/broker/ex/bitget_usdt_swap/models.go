package bitget_usdt_swap
type ExchangeInfo struct {
	Code        string `json:"code"`
	Msg         string `json:"msg"`
	RequestTime int64  `json:"requestTime"`
	Data        []Data `json:"data"`
}
type Data struct {
	Symbol               string   `json:"symbol"`
	SymbolDisplayName    string   `json:"symbolDisplayName"`
	MakerFeeRate         string   `json:"makerFeeRate"`
	TakerFeeRate         string   `json:"takerFeeRate"`
	FeeRateUpRatio       string   `json:"feeRateUpRatio"`
	OpenCostUpRatio      string   `json:"openCostUpRatio"`
	QuoteCoin            string   `json:"quoteCoin"`
	QuoteCoinDisplayName string   `json:"quoteCoinDisplayName"`
	BaseCoin             string   `json:"baseCoin"`
	BaseCoinDisplayName  string   `json:"baseCoinDisplayName"`
	BuyLimitPriceRatio   string   `json:"buyLimitPriceRatio"`
	SellLimitPriceRatio  string   `json:"sellLimitPriceRatio"`
	SupportMarginCoins   []string `json:"supportMarginCoins"`
	MinTradeNum          string   `json:"minTradeNum"`
	PriceEndStep         string   `json:"priceEndStep"`
	VolumePlace          string   `json:"volumePlace"`
	PricePlace           string   `json:"pricePlace"`
	SizeMultiplier       string   `json:"sizeMultiplier"`
	SymbolType           string   `json:"symbolType"`
	SymbolStatus         string   `json:"symbolStatus"`
	OffTime              string   `json:"offTime"`
	LimitOpenTime        string   `json:"limitOpenTime"`
	MaintainTime         string   `json:"maintainTime"`
	SymbolName           string   `json:"symbolName"`
	MinTradeUSDT         any      `json:"minTradeUSDT"`
	MaxPositionNum       any      `json:"maxPositionNum"`
	MaxOrderNum          any      `json:"maxOrderNum"`
}
