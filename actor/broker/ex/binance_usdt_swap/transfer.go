package binance_usdt_swap

import (
	"fmt"
	"actor/helper"
	"actor/helper/transfer"
	"actor/third/fixed"
	"actor/third/log"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"net/http"
	"time"
)

// ======================================账户==============================================
func (b *BinanceUsdtSwapRs) GetBalance(params transfer.GetBalanceParams) (balance float64, err helper.ApiError) {
	url := "/fapi/v2/account"
	reqBody := make(map[string]interface{})
	switch params.AccountType {
	case "FUNDING":
		b.baseUrl = "https://api.binance.com"
		url = "/sapi/v1/asset/get-funding-asset"
		err.NetworkError = b.call(http.MethodPost, url, nil, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
			p := handyPool.Get()
			defer handyPool.Put(p)
			var value *fastjson.Value
			value, err.HandlerError = p.ParseBytes(respBody)
			if err.HandlerError != nil {
				return
			}
			if respHeader.StatusCode() != http.StatusOK {
				err.HandlerError = fmt.Errorf("%s", value.String())
				return
			}
			for _, v := range value.GetArray() {
				if helper.BytesToString(v.GetStringBytes("asset")) == "USDT" {
					balance = helper.GetFloat64FromBytes(v, "free")
				}
			}
		})
	default: //合约
		err.NetworkError = b.call(http.MethodGet, url, reqBody, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
			p := handyPool.Get()
			defer handyPool.Put(p)
			var value *fastjson.Value
			value, err.HandlerError = p.ParseBytes(respBody)
			if err.HandlerError != nil {
				return
			}
			if respHeader.StatusCode() != http.StatusOK {
				err.HandlerError = fmt.Errorf("%s", value.String())
				return
			}
			for _, v := range value.GetArray("assets") {
				if helper.BytesToString(v.GetStringBytes("asset")) == "USDT" {
					balance = helper.GetFloat64FromBytes(v, "availableBalance")
				}
			}
		})
	}
	return
}

func (rs *BinanceUsdtSwapRs) DoTransferSub(params transfer.TransferParams) (err helper.ApiError) {
	var uri = "/sapi/v1/sub-account/universalTransfer"
	rs.baseUrl = "https://api.binance.com"
	reqBody := make(map[string]interface{})
	reqBody["asset"] = "USDT"
	reqBody["amount"] = fmt.Sprintf("%f", params.Amount)
	reqBody["fromAccountType"] = params.Source
	reqBody["toAccountType"] = params.Target
	if params.TransferDirection == "in" {
		reqBody["fromEmail"] = params.SubAcctName
	} else if params.TransferDirection == "out" {
		reqBody["toEmail"] = params.SubAcctName
	}

	//"SPOT"
	//"USDT_FUTURE"
	err.NetworkError = rs.call(http.MethodPost, uri, nil, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			log.Errorf("Subtransfer parse error %v", err)
			return
		}
		if value.GetInt("code") != 0 {
			err.HandlerError = fmt.Errorf(value.Get("message").String())
		}
	})
	return
}

func (b *BinanceUsdtSwapRs) GetMainBalance(params transfer.GetMainBalanceParams) (balance float64, err helper.ApiError) {
	url := "/fapi/v2/account"
	reqBody := make(map[string]interface{})
	switch params.AccountType {
	case "FUNDING":
		b.baseUrl = "https://api.binance.com"
		url = "/sapi/v1/asset/get-funding-asset"
		err.NetworkError = b.call(http.MethodPost, url, nil, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
			p := handyPool.Get()
			defer handyPool.Put(p)
			var value *fastjson.Value
			value, err.HandlerError = p.ParseBytes(respBody)
			if err.HandlerError != nil {
				return
			}
			if respHeader.StatusCode() != http.StatusOK {
				err.HandlerError = fmt.Errorf("%s", value.String())
				return
			}
			for _, v := range value.GetArray() {
				if helper.BytesToString(v.GetStringBytes("asset")) == "USDT" {
					balance = helper.GetFloat64FromBytes(v, "free")
				}
			}
		})
	default: //合约
		err.NetworkError = b.call(http.MethodGet, url, reqBody, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
			p := handyPool.Get()
			defer handyPool.Put(p)
			var value *fastjson.Value
			value, err.HandlerError = p.ParseBytes(respBody)
			if err.HandlerError != nil {
				return
			}
			if respHeader.StatusCode() != http.StatusOK {
				err.HandlerError = fmt.Errorf("%s", value.String())
				return
			}
			for _, v := range value.GetArray("assets") {
				if helper.BytesToString(v.GetStringBytes("asset")) == "USDT" {
					balance = helper.GetFloat64FromBytes(v, "availableBalance")
				}
			}
		})
	}
	return
}

func (rs *BinanceUsdtSwapRs) DoTransferSubInner(params transfer.TransferParams) (err helper.ApiError) {
	var uri = "/sapi/v1/sub-account/universalTransfer"
	rs.baseUrl = "https://api.binance.com"
	reqBody := make(map[string]interface{})
	reqBody["fromEmail"] = params.SubAcctName
	reqBody["toEmail"] = params.ToSubAcctName
	reqBody["asset"] = "USDT"
	reqBody["amount"] = fmt.Sprintf("%f", params.Amount)
	reqBody["fromAccountType"] = params.Source
	reqBody["toAccountType"] = params.Target
	//"SPOT"
	//"USDT_FUTURE"
	//"COIN_FUTURE"
	//"MARGIN"(Cross)
	//"ISOLATED_MARGIN"
	err.NetworkError = rs.call(http.MethodPost, uri, nil, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			log.Errorf("Subtransfer parse error %v", err)
			return
		}
		if !value.Exists("tranId") {
			err.NetworkError = fmt.Errorf(value.String())
			log.Error(err.NetworkError)
			return
		}
	})
	return
}

func (b *BinanceUsdtSwapRs) GetAssert(params transfer.GetBalanceParams) (accountsInfo []transfer.AccountInfo, err helper.ApiError) {
	url := "/fapi/v2/account"
	reqBody := make(map[string]interface{})
	err.NetworkError = b.call(http.MethodGet, url, reqBody, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			return
		}
		for _, v := range value.GetArray("assets") {
			if helper.BytesToString(v.GetStringBytes("asset")) == "USDT" {
				accountsInfo = append(accountsInfo, transfer.AccountInfo{
					SubUid:        params.SubAcctUid,
					SubName:       params.SubAcctName,
					MarginAccount: helper.GetFloat64FromBytes(v, "walletBalance"),
				})
			}
		}
	})
	return
}

func (b *BinanceUsdtSwapRs) GetAssert2(params transfer.GetBalanceParams) (accountsInfo []transfer.AccountInfo, err helper.ApiError) {
	url := "/sapi/v1/sub-account/futures/accountSummary"
	reqBody := make(map[string]interface{})
	err.NetworkError = b.call(http.MethodGet, url, reqBody, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			return
		}
		for _, v := range value.GetArray("subAccountList") {
			accountsInfo = append(accountsInfo, transfer.AccountInfo{
				SubName:       helper.GetStringFromBytes(v, "email"),
				MarginAccount: helper.GetFloat64FromBytes(v, "totalMarginBalance"),
			})
		}
	})
	return
}

func (rs *BinanceUsdtSwapRs) GetSubList() (accountsInfo []transfer.AccountInfo, err helper.ApiError) {
	p := handyPool.Get()
	defer handyPool.Put(p)
	var value *fastjson.Value
	url := "/sapi/v1/sub-account/list"
	rs.baseUrl = "https://api.binance.com"
	reqBody := make(map[string]interface{})
	reqBody["limit"] = 200
	reqBody["page"] = 1
	err.NetworkError = rs.call(http.MethodGet, url, reqBody, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			err.HandlerError = fmt.Errorf("GetSubList err: %s", value.String())
			return
		}
		accounts := helper.GetArray(value, "subAccounts")
		accountsInfo = make([]transfer.AccountInfo, 0, len(accounts))
		for _, v := range accounts {
			account := transfer.AccountInfo{
				SubName: v.Get("email").String(),
			}
			accountsInfo = append(accountsInfo, account)
		}
	})
	return
}

func (rs *BinanceUsdtSwapRs) CreateSubAcct(params transfer.CreateSubAcctParams) (accountsInfo []transfer.AccountInfo, err helper.ApiError) {
	url := "/sapi/v1/sub-account/virtualSubAccount"
	rs.baseUrl = "https://api.binance.com"
	reqBody := make(map[string]interface{})
	reqBody["subAccountString"] = params.SubName
	err.NetworkError = rs.call(http.MethodPost, url, nil, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			return
		}
		account := transfer.AccountInfo{
			SubName: helper.BytesToString(value.GetStringBytes("email")),
		}
		accountsInfo = append(accountsInfo, account)
	})
	return
}

func (rs *BinanceUsdtSwapRs) CreateSubAPI(params transfer.APIOperateParams) (result transfer.Api, err helper.ApiError) {
	url := "/sub_account/auth/api"
	rs.baseUrl = "https://api.binance.com"
	reqBody := make(map[string]interface{})
	reqBody["sub_user_name"] = params.SubName
	reqBody["allowed_ips"] = []string{params.IP}
	reqBody["allow_trade"] = true
	reqBody["remark"] = params.Note
	err.NetworkError = rs.call(http.MethodPost, url, nil, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			return
		}
		if value.GetInt("code") == 0 {
			data := value.Get("data")
			result = transfer.Api{
				SubName:    params.SubName,
				Subuid:     data.Get("user_auth_id").String(),
				Key:        "",
				Secret:     data.Get("secret_key").String(),
				Passphrase: data.Get("secret_key").String(),
				Remark:     data.Get("remark").String(),
				Permission: "",
				Ip:         data.Get("allowed_ips").String(),
			}
		} else {
			message := string(value.GetStringBytes("message"))
			err.HandlerError = fmt.Errorf("CreateSubAPI err: %s, sub-acct: %d", message, params.UID)
		}
	})
	return
}

func (rs *BinanceUsdtSwapRs) ModifySubAPI(params transfer.APIOperateParams) (result transfer.Api, err helper.ApiError) {
	// Binance USDT Swap 使用与 Spot 相同的 API 管理接口
	// 此功能需要通过 Binance 的 API 管理接口实现
	return
}

func (rs *BinanceUsdtSwapRs) DoTransferMain(params transfer.TransferParams) (err helper.ApiError) {
	var uri = "/sapi/v1/asset/transfer"
	rs.baseUrl = "https://api.binance.com"
	reqBody := make(map[string]interface{})
	reqBody["type"] = params.TransferType
	reqBody["asset"] = "USDT"
	reqBody["amount"] = fmt.Sprintf("%f", params.Amount)
	err.NetworkError = rs.call(http.MethodPost, uri, nil, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			log.Errorf("DoTransferMain parse error %v", err)
			return
		}
		if value.GetInt("code") != 0 {
			err.HandlerError = fmt.Errorf(value.Get("message").String())
		}
	})
	return
}

func (rs *BinanceUsdtSwapRs) DoTransferAllDireaction(params transfer.TransferAllDirectionParams) (err helper.ApiError) {
	// Binance 不支持此功能，返回空实现
	return
}

func (rs *BinanceUsdtSwapRs) GetDepositAddress() (assress []transfer.WithDraw, err helper.ApiError) {
	url := "/sapi/v1/capital/deposit/address"
	rs.baseUrl = "https://api.binance.com"
	reqBody := make(map[string]interface{})
	reqBody["coin"] = "USDT"
	reqBody["network"] = "ETH"
	p := handyPool.Get()
	defer handyPool.Put(p)
	var value *fastjson.Value
	err.NetworkError = rs.call(http.MethodGet, url, reqBody, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			return
		}
		log.Infof("====GetDepositAddress==%v==", value.String())
		coin := helper.GetStringFromBytes(value, "coin")
		if coin == "USDT" {
			assress = append(assress, transfer.WithDraw{
				Coin:      "USDT",
				Chain:     "ETH",
				ToAddress: helper.GetStringFromBytes(value, "address"),
			})
		}
	})

	reqBody["network"] = "TRX"
	err.NetworkError = rs.call(http.MethodGet, url, reqBody, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			return
		}
		log.Infof("====GetDepositAddress==%v==", value.String())
		coin := helper.GetStringFromBytes(value, "coin")
		if coin == "USDT" {
			assress = append(assress, transfer.WithDraw{
				Coin:      "USDT",
				Chain:     "TRX",
				ToAddress: helper.GetStringFromBytes(value, "address"),
			})
		}
	})
	return
}

func (rs *BinanceUsdtSwapRs) WithDraw(params transfer.WithDraw) (result string, err helper.ApiError) {
	var uri = "/sapi/v1/capital/withdraw/apply"
	rs.baseUrl = "https://api.binance.com"
	reqBody := map[string]interface{}{
		"coin":    params.Coin,
		"address": params.ToAddress,
		"amount":  fixed.NewF(params.Amount).String(),
		"network": params.Chain, //TRX
	}
	err.NetworkError = rs.call(http.MethodPost, uri, nil, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			log.Errorf("Withdraw parse error %v", err)
			return
		}
		if respHeader.StatusCode() != http.StatusOK {
			err.HandlerError = fmt.Errorf("WithDrawHistory err: %s", value.String())
			return
		}
		result = string(respBody)
	})
	return
}

func (rs *BinanceUsdtSwapRs) WithDrawHistory(param transfer.GetWithDrawHistoryParams) (records []transfer.WithDrawHistory, err helper.ApiError) {
	var uri = "/sapi/v1/capital/withdraw/history"
	rs.baseUrl = "https://api.binance.com"
	reqBody := make(map[string]interface{})
	if param.Start != 0 && param.End != 0 {
		reqBody["startTime"] = param.Start
		reqBody["endTime"] = param.End
	}
	err.NetworkError = rs.call(http.MethodGet, uri, reqBody, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			log.Errorf("Withdraw parse error %v", err)
			return
		}
		if respHeader.StatusCode() != http.StatusOK {
			err.HandlerError = fmt.Errorf("WithDrawHistory err: %s", value.String())
			return
		}
		for _, item := range value.GetArray() {
			if helper.GetInt(item, "status") == 6 {
				applyTimeStr := helper.GetStringFromBytes(item, "applyTime")
				var applyTime time.Time
				var pErr error
				if applyTimeStr != "" {
					applyTime, pErr = time.Parse(time.DateTime, applyTimeStr)
					if pErr != nil {
						log.Error(pErr)
					}
				}

				completeTimeStr := helper.GetStringFromBytes(item, "completeTime")
				var completeTime time.Time
				var cErr error
				if completeTimeStr != "" {
					completeTime, cErr = time.Parse(time.DateTime, completeTimeStr)
					if cErr != nil {
						log.Error(cErr)
					}
				}

				records = append(records, transfer.WithDrawHistory{
					Platform:     "binance",
					Account:      param.Account,
					ApplyTime:    applyTime.UnixMilli(),
					CompleteTime: completeTime.UnixMilli(),
					Amount:       helper.GetFloat64FromBytes(item, "amount"),
					Address:      helper.GetStringFromBytes(item, "address"),
					OrderID:      helper.GetStringFromBytes(item, "txId"),
					Currency:     helper.GetStringFromBytes(item, "coin"),
				})
			}
		}
	})
	return
}
