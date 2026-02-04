package bitget_spot

type ExchangeInfo struct {
	Code        string `json:"code"`
	Msg         string `json:"msg"`
	RequestTime int64  `json:"requestTime"`
	Data        []Data `json:"data"`
}
type Data struct {
	Symbol               string `json:"symbol"`
	SymbolName           string `json:"symbolName"`
	SymbolDisplayName    string `json:"symbolDisplayName"`
	BaseCoin             string `json:"baseCoin"`
	BaseCoinDisplayName  string `json:"baseCoinDisplayName"`
	QuoteCoin            string `json:"quoteCoin"`
	QuoteCoinDisplayName string `json:"quoteCoinDisplayName"`
	MinTradeAmount       string `json:"minTradeAmount"`
	MaxTradeAmount       string `json:"maxTradeAmount"`
	// TakerFeeRate         string `json:"takerFeeRate"`
	// MakerFeeRate         string `json:"makerFeeRate"`
	PriceScale    string `json:"priceScale"`
	QuantityScale string `json:"quantityScale"`
	// QuotePrecision string `json:"quotePrecision"`
	Status       string `json:"status"`
	MinTradeUSDT string `json:"minTradeUSDT"`
	// BuyLimitPriceRatio  string `json:"buyLimitPriceRatio"`
	// SellLimitPriceRatio string `json:"sellLimitPriceRatio"`
	// MaxOrderNum          any    `json:"maxOrderNum"`
}
