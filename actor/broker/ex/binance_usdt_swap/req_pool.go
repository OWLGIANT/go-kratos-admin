package binance_usdt_swap

import (
	"bytes"
	"net/http"
	"strconv"
	"sync"
	"time"

	"actor/helper"
	"actor/third/fixed"
	"actor/third/log"
	"github.com/mailru/easyjson/jwriter"
	"github.com/valyala/fasthttp"
)

type OrderPlaceReq struct {
	Request            fasthttp.Request
	paramsWriter       jwriter.Writer
	paramsBytes        []byte
	paramsStrToSignBuf bytes.Buffer
	uriBuf             bytes.Buffer
	signer             *helper.SignerHmacSHA256Hex
}

type OrderPlaceReqPool struct {
	pool sync.Pool
}

func (o *OrderPlaceReq) ResetParams(b *BinanceUsdtSwapRs, symbol string, pairInfo *helper.ExchangeInfo, price float64, size fixed.Fixed, cid string, side helper.OrderSide, orderType helper.OrderType, colo bool) error {
	o.paramsStrToSignBuf.Reset()
	o.paramsStrToSignBuf.WriteString("nl=true&")
	o.paramsStrToSignBuf.WriteString("symbol=")
	o.paramsStrToSignBuf.WriteString(symbol)
	if orderType != helper.OrderTypeMarket {
		o.paramsStrToSignBuf.WriteString("&price=")
		o.paramsStrToSignBuf.WriteString(helper.FixPrice(price, pairInfo.TickSize).String())
	}
	o.paramsStrToSignBuf.WriteString("&quantity=")
	o.paramsStrToSignBuf.WriteString(size.String())

	if orderType == helper.OrderTypeIoc {
		o.paramsStrToSignBuf.WriteString("&type=LIMIT")
		o.paramsStrToSignBuf.WriteString("&timeInForce=IOC")
	} else if orderType == helper.OrderTypePostOnly {
		o.paramsStrToSignBuf.WriteString("&type=LIMIT")
		o.paramsStrToSignBuf.WriteString("&timeInForce=GTX")
	} else if orderType == helper.OrderTypeMarket {
		o.paramsStrToSignBuf.WriteString("&type=MARKET")
	} else if orderType == helper.OrderTypeLimit {
		o.paramsStrToSignBuf.WriteString("&type=LIMIT")
		o.paramsStrToSignBuf.WriteString("&timeInForce=GTC")
	} else {
		var order helper.OrderEvent
		order.Type = helper.OrderEventTypeERROR
		order.ClientID = cid
		b.Cb.OnOrder(0, order)
		log.Errorf("[%s]%s下单失败 下单类型不正确%v", b.ExchangeName, cid, orderType)
		return ErrWrongOrderParams
	}
	switch side {
	case helper.OrderSideKD:
		o.paramsStrToSignBuf.WriteString("&side=BUY")
		// o.params.ReduceOnly = false
	case helper.OrderSideKK:
		o.paramsStrToSignBuf.WriteString("&side=SELL")
		// o.params.ReduceOnly = false
	case helper.OrderSidePD:
		o.paramsStrToSignBuf.WriteString("&side=SELL")
		o.paramsStrToSignBuf.WriteString("&reduceOnly=true") // 双向模式不接受这个参数，小心
	case helper.OrderSidePK:
		o.paramsStrToSignBuf.WriteString("&side=BUY")
		o.paramsStrToSignBuf.WriteString("&reduceOnly=true") // 双向模式不接受这个参数，小心
	}
	if cid != "" {
		o.paramsStrToSignBuf.WriteString("&newClientOrderId=")
		o.paramsStrToSignBuf.WriteString(cid)
	}
	o.paramsStrToSignBuf.WriteString("&recvWindow=5000")

	o.paramsStrToSignBuf.WriteString("&timestamp=")
	o.paramsStrToSignBuf.WriteString(strconv.FormatInt(time.Now().UnixMilli(), 10))

	sign := helper.BytesToString(o.signer.Sign(o.paramsStrToSignBuf.Bytes()))
	o.paramsStrToSignBuf.WriteString("&signature=")
	o.paramsStrToSignBuf.WriteString(sign)

	o.uriBuf.Reset()
	if colo {
		o.uriBuf.WriteString(RS_URL_COLO)
	} else {
		o.uriBuf.WriteString(RS_URL)
	}
	o.uriBuf.WriteString("/fapi/v1/order?")
	o.uriBuf.Write(o.paramsStrToSignBuf.Bytes())

	o.Request.SetRequestURIBytes(o.uriBuf.Bytes())

	return nil
}

func (p *OrderPlaceReqPool) Get(b *BinanceUsdtSwapRs) *OrderPlaceReq {
	v := p.pool.Get()
	if v == nil {
		req := &OrderPlaceReq{}
		req.Request.Header.SetMethod(http.MethodPost)
		req.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Request.Header.Set("X-MBX-APIKEY", b.BrokerConfig.AccessKey)
		// req.Request.Header.SetProtocol("https")
		// req.Request.SetHost(RS_HOST)

		req.signer = helper.NewSignerHmacSHA256Hex(helper.StringToBytes(b.BrokerConfig.SecretKey))

		return req
	}
	return v.(*OrderPlaceReq)
}

func (p *OrderPlaceReqPool) Put(req *OrderPlaceReq) {
	p.pool.Put(req)
}

type OrderCancelParams struct {
	ClientID string `json:"clientOid"`
	OrderID  string `json:"orderId"`
}

type OrderCancelByOidReq struct {
	Request            fasthttp.Request
	paramsStrToSignBuf bytes.Buffer
	uriBuf             bytes.Buffer
	signer             *helper.SignerHmacSHA256Hex
}

type OrderCancelReqPool struct {
	pool sync.Pool
}

func (req *OrderCancelByOidReq) ResetParams(b *BinanceUsdtSwapRs, symbol string, oid, cid string, colo bool) error {
	req.paramsStrToSignBuf.Reset()
	req.paramsStrToSignBuf.WriteString("symbol=")
	req.paramsStrToSignBuf.WriteString(symbol)
	if oid != "" {
		req.paramsStrToSignBuf.WriteString("&orderId=")
		req.paramsStrToSignBuf.WriteString(oid)
	} else if cid != "" {
		req.paramsStrToSignBuf.WriteString("&origClientOrderId=")
		req.paramsStrToSignBuf.WriteString(cid)
	} else {
		return ErrWrongOrderParams
	}
	req.paramsStrToSignBuf.WriteString("&recvWindow=5000")

	req.paramsStrToSignBuf.WriteString("&timestamp=")
	req.paramsStrToSignBuf.WriteString(strconv.FormatInt(time.Now().UnixMilli(), 10))

	sign := helper.BytesToString(req.signer.Sign(req.paramsStrToSignBuf.Bytes()))
	req.paramsStrToSignBuf.WriteString("&signature=")
	req.paramsStrToSignBuf.WriteString(sign)

	req.uriBuf.Reset()
	if colo {
		req.uriBuf.WriteString(RS_URL_COLO)
	} else {
		req.uriBuf.WriteString(RS_URL)
	}
	req.uriBuf.WriteString("/fapi/v1/order?")
	req.uriBuf.Write(req.paramsStrToSignBuf.Bytes())

	req.Request.SetRequestURIBytes(req.uriBuf.Bytes())

	return nil
}

func (p *OrderCancelReqPool) Get(b *BinanceUsdtSwapRs) *OrderCancelByOidReq {
	v := p.pool.Get()
	if v == nil {
		req := &OrderCancelByOidReq{}
		req.Request.Header.SetMethod(http.MethodDelete)

		req.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Request.Header.Set("X-MBX-APIKEY", b.BrokerConfig.AccessKey)
		// req.Request.SetHost(RS_HOST)
		// req.Request.Header.SetProtocol("https")

		req.signer = helper.NewSignerHmacSHA256Hex(helper.StringToBytes(b.BrokerConfig.SecretKey))

		return req
	}
	return v.(*OrderCancelByOidReq)
}

func (p *OrderCancelReqPool) Put(req *OrderCancelByOidReq) {
	p.pool.Put(req)
}

type OrderAmendReq struct {
	Request            fasthttp.Request
	paramsWriter       jwriter.Writer
	paramsBytes        []byte
	paramsStrToSignBuf bytes.Buffer
	uriBuf             bytes.Buffer
	signer             *helper.SignerHmacSHA256Hex
}

type OrderAmendReqPool struct {
	pool sync.Pool
}

func (o *OrderAmendReq) ResetParams(b *BinanceUsdtSwapRs, info *helper.ExchangeInfo, price float64, size fixed.Fixed, cid, oid string, side helper.OrderSide, colo bool) error {
	o.paramsStrToSignBuf.Reset()
	o.paramsStrToSignBuf.WriteString("symbol=")
	o.paramsStrToSignBuf.WriteString(info.Symbol)
	o.paramsStrToSignBuf.WriteString("&price=")
	o.paramsStrToSignBuf.WriteString(helper.FixPrice(price, info.TickSize).String())
	o.paramsStrToSignBuf.WriteString("&quantity=")
	o.paramsStrToSignBuf.WriteString(size.String())

	switch side {
	case helper.OrderSideKD:
		o.paramsStrToSignBuf.WriteString("&side=BUY")
	case helper.OrderSideKK:
		o.paramsStrToSignBuf.WriteString("&side=SELL")
	case helper.OrderSidePD:
		o.paramsStrToSignBuf.WriteString("&side=SELL")
	case helper.OrderSidePK:
		o.paramsStrToSignBuf.WriteString("&side=BUY")
	}
	if oid != "" {
		o.paramsStrToSignBuf.WriteString("&orderId=")
		o.paramsStrToSignBuf.WriteString(oid)
	} else if cid != "" {
		o.paramsStrToSignBuf.WriteString("&origClientOrderId=")
		o.paramsStrToSignBuf.WriteString(cid)
	}
	o.paramsStrToSignBuf.WriteString("&recvWindow=5000")

	o.paramsStrToSignBuf.WriteString("&timestamp=")
	o.paramsStrToSignBuf.WriteString(strconv.FormatInt(time.Now().UnixMilli(), 10))

	sign := helper.BytesToString(o.signer.Sign(o.paramsStrToSignBuf.Bytes()))
	o.paramsStrToSignBuf.WriteString("&signature=")
	o.paramsStrToSignBuf.WriteString(sign)

	o.uriBuf.Reset()
	if colo {
		o.uriBuf.WriteString(RS_URL_COLO)
	} else {
		o.uriBuf.WriteString(RS_URL)
	}
	o.uriBuf.WriteString("/fapi/v1/order?")
	o.uriBuf.Write(o.paramsStrToSignBuf.Bytes())

	o.Request.SetRequestURIBytes(o.uriBuf.Bytes())

	return nil
}

func (p *OrderAmendReqPool) Get(b *BinanceUsdtSwapRs) *OrderAmendReq {
	v := p.pool.Get()
	if v == nil {
		req := &OrderAmendReq{}
		req.Request.Header.SetMethod(http.MethodPut)
		req.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Request.Header.Set("X-MBX-APIKEY", b.BrokerConfig.AccessKey)

		req.signer = helper.NewSignerHmacSHA256Hex(helper.StringToBytes(b.BrokerConfig.SecretKey))

		return req
	}
	return v.(*OrderAmendReq)
}

func (p *OrderAmendReqPool) Put(req *OrderAmendReq) {
	p.pool.Put(req)
}
