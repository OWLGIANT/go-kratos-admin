package okx_usdt_swap

import (
	"actor/helper"
	"fmt"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"time"
)

func (rs *OkxUsdtSwapRs) GetCandles(bar string) (resp []*helper.Kline, apiErr helper.ApiError) {
	msg := map[string]interface{}{
		"instId": rs.Symbol,
		"bar":    bar,
		"limit":  300,
	}
	p := handyPool.Get()
	defer handyPool.Put(p)
	_, apiErr.NetworkError = rs.client.get("/api/v5/market/candles", msg, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value *fastjson.Value
		value, apiErr.HandlerError = p.ParseBytes(respBody)
		if apiErr.HandlerError != nil {
			rs.Logger.Errorf("获取candles 解析错误 %v", apiErr.HandlerError)
			return
		}
		if !rs.isOkApiResponse(value, "/api/v5/market/candles") {
			apiErr.HandlerError = fmt.Errorf("获取candles 失败 %v", value)
			return
		}
		/*{
		    "code":"0",
		    "msg":"",
		    "data":[
		     [
		        "1597026383085",
		        "3.721",
		        "3.743",
		        "3.677",
		        "3.708",
		        "8422410",
		        "22698348.04828491",
		        "12698348.04828491",
		        "0"
		    ],
		    [
		        "1597026383085",
		        "3.731",
		        "3.799",
		        "3.494",
		        "3.72",
		        "24912403",
		        "67632347.24399722",
		        "37632347.24399722",
		        "1"
		    ]
		    ]
		}
		*/
		for _, klineItem := range value.GetArray("data") {
			resp = append(resp, &helper.Kline{
				Symbol:     rs.Symbol,
				OpenTimeMs: helper.GetInt64(klineItem.GetArray()[0]),
				//CloseTimeMs:  0,
				//EventTimeMs:  0,
				LocalTimeMs:  time.Now().UnixMilli(),
				Open:         helper.GetFloat64(klineItem.GetArray()[1]),
				Close:        helper.GetFloat64(klineItem.GetArray()[4]),
				High:         helper.GetFloat64(klineItem.GetArray()[2]),
				Low:          helper.GetFloat64(klineItem.GetArray()[3]),
				BuyNotional:  0,
				BuyVolume:    helper.GetFloat64(klineItem.GetArray()[7]),
				SellNotional: 0,
				SellVolume:   0,
				BuyTradeNum:  0,
				SellTradeNum: 0,
			})
		}
	})
	if !apiErr.Nil() {
		rs.Logger.Errorf("获取candles失败 网络错误 %v", apiErr)
	}
	return
}
