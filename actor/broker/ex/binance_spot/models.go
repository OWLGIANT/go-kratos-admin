package binance_spot

type ExchangeInfo struct {
	Code            string       `json:"code,omitempty"`
	Timezone        string       `json:"timezone"`
	ServerTime      int64        `json:"serverTime"`
	RateLimits      []RateLimits `json:"rateLimits"`
	ExchangeFilters []any        `json:"exchangeFilters"`
	Symbols         []Symbols    `json:"symbols"`
}
type RateLimits struct {
	RateLimitType string `json:"rateLimitType"`
	Interval      string `json:"interval"`
	IntervalNum   int    `json:"intervalNum"`
	Limit         int    `json:"limit"`
}
type Filters struct {
	FilterType            string `json:"filterType"`
	MinPrice              string `json:"minPrice,omitempty"`
	MaxPrice              string `json:"maxPrice,omitempty"`
	TickSize              string `json:"tickSize,omitempty"`
	MinQty                string `json:"minQty,omitempty"`
	MaxQty                string `json:"maxQty,omitempty"`
	StepSize              string `json:"stepSize,omitempty"`
	Limit                 int    `json:"limit,omitempty"`
	MinTrailingAboveDelta int    `json:"minTrailingAboveDelta,omitempty"`
	MaxTrailingAboveDelta int    `json:"maxTrailingAboveDelta,omitempty"`
	MinTrailingBelowDelta int    `json:"minTrailingBelowDelta,omitempty"`
	MaxTrailingBelowDelta int    `json:"maxTrailingBelowDelta,omitempty"`
	BidMultiplierUp       string `json:"bidMultiplierUp,omitempty"`
	BidMultiplierDown     string `json:"bidMultiplierDown,omitempty"`
	AskMultiplierUp       string `json:"askMultiplierUp,omitempty"`
	AskMultiplierDown     string `json:"askMultiplierDown,omitempty"`
	AvgPriceMins          int    `json:"avgPriceMins,omitempty"`
	MinNotional           string `json:"minNotional,omitempty"`
	ApplyMinToMarket      bool   `json:"applyMinToMarket,omitempty"`
	MaxNotional           string `json:"maxNotional,omitempty"`
	ApplyMaxToMarket      bool   `json:"applyMaxToMarket,omitempty"`
	MaxNumOrders          int    `json:"maxNumOrders,omitempty"`
	MaxNumAlgoOrders      int    `json:"maxNumAlgoOrders,omitempty"`
}
type Symbols struct {
	Symbol    string `json:"symbol"`
	Status    string `json:"status"`
	BaseAsset string `json:"baseAsset"`
	// BaseAssetPrecision         int       `json:"baseAssetPrecision"`
	QuoteAsset string `json:"quoteAsset"`
	// QuotePrecision             int       `json:"quotePrecision"`
	// QuoteAssetPrecision        int       `json:"quoteAssetPrecision"`
	// BaseCommissionPrecision    int       `json:"baseCommissionPrecision"`
	// QuoteCommissionPrecision   int       `json:"quoteCommissionPrecision"`
	// OrderTypes                 []string  `json:"orderTypes"`
	// IcebergAllowed             bool      `json:"icebergAllowed"`
	// OcoAllowed                 bool      `json:"ocoAllowed"`
	// OtoAllowed                 bool      `json:"otoAllowed"`
	// QuoteOrderQtyMarketAllowed bool      `json:"quoteOrderQtyMarketAllowed"`
	// AllowTrailingStop          bool      `json:"allowTrailingStop"`
	// CancelReplaceAllowed       bool      `json:"cancelReplaceAllowed"`
	IsSpotTradingAllowed bool `json:"isSpotTradingAllowed"`
	// IsMarginTradingAllowed     bool      `json:"isMarginTradingAllowed"`
	Filters []Filters `json:"filters"`
	// Permissions                []any     `json:"permissions"`
	// // PermissionSets []PermissionSets[]string `json:"permissionSets"`
	// DefaultSelfTradePreventionMode  string   `json:"defaultSelfTradePreventionMode"`
	// AllowedSelfTradePreventionModes []string `json:"allowedSelfTradePreventionModes"`
}
