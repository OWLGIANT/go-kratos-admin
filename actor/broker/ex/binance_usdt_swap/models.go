package binance_usdt_swap

type ExchangeInfo struct {
	Code int `json:"code,omitempty"`
	// Timezone        string       `json:"timezone"`
	// ServerTime      int64        `json:"serverTime"`
	// FuturesType     string       `json:"futuresType"`
	// RateLimits      []RateLimits `json:"rateLimits"`
	// ExchangeFilters []any     `json:"exchangeFilters"`
	// Assets  []Assets  `json:"assets"`
	Symbols []Symbols `json:"symbols"`
}
type RateLimits struct {
	RateLimitType string `json:"rateLimitType"`
	Interval      string `json:"interval"`
	IntervalNum   int    `json:"intervalNum"`
	Limit         int    `json:"limit"`
}
type Assets struct {
	Asset             string `json:"asset"`
	MarginAvailable   bool   `json:"marginAvailable"`
	AutoAssetExchange string `json:"autoAssetExchange"`
}
type Filters struct {
	MinPrice          string `json:"minPrice,omitempty"`
	MaxPrice          string `json:"maxPrice,omitempty"`
	FilterType        string `json:"filterType"`
	TickSize          string `json:"tickSize,omitempty"`
	MinQty            string `json:"minQty,omitempty"`
	MaxQty            string `json:"maxQty,omitempty"`
	StepSize          string `json:"stepSize,omitempty"`
	Limit             int    `json:"limit,omitempty"`
	Notional          string `json:"notional,omitempty"`
	MultiplierDecimal string `json:"multiplierDecimal,omitempty"`
	MultiplierUp      string `json:"multiplierUp,omitempty"`
	MultiplierDown    string `json:"multiplierDown,omitempty"`
}
type Symbols struct {
	Symbol       string `json:"symbol"`
	Pair         string `json:"pair"`
	ContractType string `json:"contractType"`
	// DeliveryDate          int64     `json:"deliveryDate"`
	// OnboardDate           int64     `json:"onboardDate"`
	Status string `json:"status"`
	// MaintMarginPercent    string    `json:"maintMarginPercent"`
	// RequiredMarginPercent string    `json:"requiredMarginPercent"`
	BaseAsset  string `json:"baseAsset"`
	QuoteAsset string `json:"quoteAsset"`
	// MarginAsset           string    `json:"marginAsset"`
	// PricePrecision        int       `json:"pricePrecision"`
	// QuantityPrecision     int       `json:"quantityPrecision"`
	// BaseAssetPrecision    int       `json:"baseAssetPrecision"`
	// QuotePrecision        int       `json:"quotePrecision"`
	// UnderlyingType        string    `json:"underlyingType"`
	// UnderlyingSubType     []string  `json:"underlyingSubType"`
	// SettlePlan            int       `json:"settlePlan"`
	// TriggerProtect        string    `json:"triggerProtect"`
	// LiquidationFee        string    `json:"liquidationFee"`
	// MarketTakeBound       string    `json:"marketTakeBound"`
	// MaxMoveOrderLimit     int       `json:"maxMoveOrderLimit"`
	Filters []Filters `json:"filters"`
	// OrderTypes            []string  `json:"orderTypes"`
	// TimeInForce           []string  `json:"timeInForce"`
}
