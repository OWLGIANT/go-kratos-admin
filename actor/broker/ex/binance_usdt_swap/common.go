package binance_usdt_swap

import "errors"

const (
	RS_URL     = "https://fapi.binance.com"
	RS_HOST    = "fapi.binance.com"
	LEVER_RATE = 6
)

var RS_URL_COLO = ""
var WS_URL_COLO = ""

var (
	ErrWrongOrderParams = errors.New("wrong order params")
	LeverMap            map[string]int
)

func init() {
	LeverMap = make(map[string]int)
}
