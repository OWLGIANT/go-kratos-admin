package binance_usdt_swap

import (
	"fmt"
	"testing"
	"time"

	"actor/helper"
)

func TestPriHandler(t *testing.T) {
	tm := helper.TradeMsg{}
	params := helper.BrokerConfigExt{}
	pairInfo := helper.ExchangeInfo{}
	ws0 := NewWs(&params, &tm, &pairInfo, helper.CallbackFunc{
		OnTicker: func(ts int64) {
			fmt.Println("ws ticker ", tm.Ticker.String())
		},
		OnOrder: func(ts int64, event helper.OrderEvent) {
			fmt.Println("ws order ", event.String())
		},
	})
	ws, _ := ws0.(*BinanceUsdtSwapWs)
	msg := `{"e":"ORDER_TRADE_UPDATE","T":1704705592498,"E":1704705592499,"o":{"s":"ACEUSDT","c":"x-nXtHr5jjSNF17322011","S":"BUY","o":"LIMIT","f":"IOC","q":"1.26","p":"7.883300","ap":"0","sp":"0","x":"EXPIRED","X":"EXPIRED","i":374805065,"l":"0","z":"0","L":"0","n":"0","N":"USDT","T":1704705592498,"t":0,"b":"0","a":"0","m":false,"R":false,"wt":"CONTRACT_PRICE","ot":"LIMIT","ps":"BOTH","cp":false,"rp":"0","pP":false,"si":0,"ss":0,"V":"NONE","pm":"NONE","gtd":0}}`
	ws.priHandler(helper.StringToBytes(msg), time.Now().UnixNano())
}
