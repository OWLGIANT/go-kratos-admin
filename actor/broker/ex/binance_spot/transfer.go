package binance_spot

import (
	"fmt"
	"actor/helper"
	"actor/helper/transfer"
	"actor/third/fixed"
	"actor/third/log"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"net/http"
	"strings"
	"time"
)

// ================================账户===========================================================
func (b *BinanceSpotRs) GetBalance(params transfer.GetBalanceParams) (balance float64, err helper.ApiError) {
	url := "/api/v3/account"
	reqBody := make(map[string]interface{})
	reqBody["omitZeroBalances"] = true
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

		for _, v := range value.GetArray("balances") {
			if helper.BytesToString(v.GetStringBytes("asset")) == "USDT" {
				balance = helper.GetFloat64FromBytes(v, "free")
			}
		}
	}, false)
	return
}

func (rs *BinanceSpotRs) DoTransferSub(params transfer.TransferParams) (err helper.ApiError) {
	var uri = "/sapi/v1/sub-account/universalTransfer"
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
	}, false)
	if err.NetworkError != nil {
		log.Errorf("Subtransfer network error %v", err)
		return
	}
	return
}

func (rs *BinanceSpotRs) DoTransferMain(params transfer.TransferParams) (err helper.ApiError) {
	var uri = "/sapi/v1/sub-account/universalTransfer"
	reqBody := make(map[string]interface{})
	reqBody["asset"] = "USDT"
	reqBody["amount"] = fmt.Sprintf("%f", params.Amount)
	reqBody["fromAccountType"] = params.Source
	reqBody["toAccountType"] = params.Target
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
	}, false)
	if err.NetworkError != nil {
		log.Errorf("Subtransfer network error %v", err)
		return
	}
	return
}

func (b *BinanceSpotRs) GetSubList() (accountsInfo []transfer.AccountInfo, err helper.ApiError) {
	p := handyPool.Get()
	defer handyPool.Put(p)
	var value *fastjson.Value
	url := "/sapi/v1/sub-account/list"
	reqBody := make(map[string]interface{})
	reqBody["limit"] = 200
	reqBody["page"] = 1
	err.NetworkError = b.call(http.MethodGet, url, reqBody, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
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
	}, false)
	return
}

func (b *BinanceSpotRs) CreateSubAcct(params transfer.CreateSubAcctParams) (accountsInfo []transfer.AccountInfo, err helper.ApiError) {
	url := "/sapi/v1/sub-account/virtualSubAccount"
	reqBody := make(map[string]interface{})
	reqBody["subAccountString"] = params.SubName
	err.NetworkError = b.call(http.MethodPost, url, nil, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
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
	}, false)
	return
}

func (b *BinanceSpotRs) CreateSubAPI(params transfer.APIOperateParams) (result transfer.Api, err helper.ApiError) {
	url := "/sub_account/auth/api"
	reqBody := make(map[string]interface{})
	reqBody["sub_user_name"] = params.SubName
	reqBody["allowed_ips"] = []string{params.IP}
	reqBody["allow_trade"] = true
	reqBody["remark"] = params.Note
	err.NetworkError = b.call(http.MethodPost, url, nil, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
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
	}, false)
	return
}

func (b *BinanceSpotRs) ModifySubAPIDeleteIps(params transfer.APIOperateParams) (result transfer.Api, err helper.ApiError) {
	url := "/sapi/v1/sub-account/subAccountApi/ipRestriction/ipList"
	reqBody := make(map[string]interface{})
	reqBody["email"] = params.SubName
	reqBody["subAccountApiKey"] = params.APIKey
	if params.RemoveIP == "" {
		return
	} else {
		ipSet := mapset.NewSet[string]()
		for _, ip := range strings.Split(params.RemoveIP, ",") {
			if ip != "" {
				ipSet.Add(ip)
			}
		}
		if ipSet.Cardinality() > 0 {
			reqBody["ipAddress"] = strings.Join(ipSet.ToSlice(), ",")
		} else {
			return
		}
	}
	p := handyPool.Get()
	defer handyPool.Put(p)
	err.NetworkError = b.call(http.MethodDelete, url, nil, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			return
		}
		if helper.GetStringFromBytes(value, "apiKey") != params.APIKey {
			err.HandlerError = fmt.Errorf(value.String())
		}
		log.Infof("==MethodDelete=======%v========", value.String())
		var ips []string
		for _, ip := range value.GetArray("ipList") {
			tempIp := string(ip.GetStringBytes())
			if tempIp == "" {
				continue
			}
			ips = append(ips, tempIp)
		}
		result = transfer.Api{
			SubName: params.SubName,
			Key:     helper.GetStringFromBytes(value, "apiKey"),
			Ip:      strings.Join(ips, ","),
			IPs:     ips,
		}
	}, false)
	return
}

func (b *BinanceSpotRs) ModifySubAPI(params transfer.APIOperateParams) (result transfer.Api, err helper.ApiError) {
	url := "/sapi/v2/sub-account/subAccountApi/ipRestriction"
	reqBody := make(map[string]interface{})
	reqBody["email"] = params.SubName
	reqBody["subAccountApiKey"] = params.APIKey
	reqBody["status"] = "2"
	if params.RemoveIP != "" {
		result, err = b.ModifySubAPIDeleteIps(params)
	} else {
		result, err = b.GetSubSpecificAPI(params)
	}
	if params.IP == "" {
		return
	} else {
		ipSet := mapset.NewSet[string]()
		for _, ip := range strings.Split(params.IP, ",") {
			if ip != "" {
				ipSet.Add(ip)
			}
		}
		for _, ip := range result.IPs {
			if ip != "" {
				ipSet.Remove(ip)
			}
		}
		if ipSet.Cardinality() > 0 {
			if params.RemoveIP != "" {
				for _, ip := range strings.Split(params.RemoveIP, ",") {
					if ip != "" {
						ipSet.Remove(ip)
					}
				}
			}
			reqBody["ipAddress"] = strings.Join(ipSet.ToSlice(), ",")
		} else {
			return
		}
	}
	p := handyPool.Get()
	defer handyPool.Put(p)
	err.NetworkError = b.call(http.MethodPost, url, nil, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			return
		}
		if helper.GetStringFromBytes(value, "apiKey") != params.APIKey {
			err.HandlerError = fmt.Errorf(value.String())
		}
		log.Infof("==MethodPost=======%v========", value.String())
		var ips []string
		for _, ip := range value.GetArray("ipList") {
			tempIp := string(ip.GetStringBytes())
			if tempIp == "" {
				continue
			}
			ips = append(ips, tempIp)
		}
		result = transfer.Api{
			SubName: params.SubName,
			Key:     helper.GetStringFromBytes(value, "apiKey"),
			Ip:      strings.Join(ips, ","),
			IPs:     ips,
		}
	}, false)
	return
}

func (b *BinanceSpotRs) GetMainBalance(params transfer.GetMainBalanceParams) (balance float64, err helper.ApiError) {
	url := "/api/v3/account"
	reqBody := make(map[string]interface{})
	reqBody["omitZeroBalances"] = true
	err.NetworkError = b.call(http.MethodGet, url, reqBody, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			return
		}
		for _, v := range value.GetArray("balances") {
			if helper.BytesToString(v.GetStringBytes("asset")) == "USDT" {
				balance = helper.GetFloat64FromBytes(v, "free")
			}
		}
	}, false)
	return
}

func (rs *BinanceSpotRs) DoTransferSubInner(params transfer.TransferParams) (err helper.ApiError) {
	var uri = "/sapi/v1/sub-account/universalTransfer"
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
		if respHeader.StatusCode() != http.StatusOK {
			err.NetworkError = fmt.Errorf(value.String())
			return
		}
	}, false)
	return
}

func (b *BinanceSpotRs) GetAssert2() (accountsInfo []transfer.AccountInfo, err helper.ApiError) {
	url := "/sapi/v1/sub-account/spotSummary"
	reqBody := make(map[string]interface{})
	pageSize := 20
	reqBody["size"] = pageSize
	accountInfoMap := make(map[string]transfer.AccountInfo)
	for i := 1; i <= 15; i++ {
		time.Sleep(time.Second / 5)
		reqBody["page"] = i
		err.NetworkError = b.call(http.MethodGet, url, reqBody, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
			p := handyPool.Get()
			defer handyPool.Put(p)
			var value *fastjson.Value
			value, err.HandlerError = p.ParseBytes(respBody)
			if err.HandlerError != nil {
				return
			}
			for _, v := range value.GetArray("spotSubUserAssetBtcVoList") {
				accountInfoMap[helper.GetStringFromBytes(v, "email")] = transfer.AccountInfo{
					SubName: helper.GetStringFromBytes(v, "email"),
					Assert:  helper.GetFloat64FromBytes(v, "totalAsset"),
				}
			}
		}, false)
	}
	for _, account := range accountInfoMap {
		accountsInfo = append(accountsInfo, account)
	}
	return
}

func (b *BinanceSpotRs) GetAssert(params transfer.GetBalanceParams) (accountsInfo []transfer.AccountInfo, err helper.ApiError) {
	url := "/api/v3/account"
	reqBody := make(map[string]interface{})
	reqBody["omitZeroBalances"] = true
	err.NetworkError = b.call(http.MethodGet, url, reqBody, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			return
		}
		for _, v := range value.GetArray("balances") {
			if helper.BytesToString(v.GetStringBytes("asset")) == "USDT" {
				accountsInfo = append(accountsInfo, transfer.AccountInfo{
					SubUid:  params.SubAcctUid,
					SubName: params.SubAcctName,
					Assert:  helper.GetFloat64FromBytes(v, "free") + helper.GetFloat64FromBytes(v, "locked"),
				})
			}
		}
	}, false)
	return
}

func (b *BinanceSpotRs) GetSubSpecificAPI(params transfer.APIOperateParams) (result transfer.Api, err helper.ApiError) {
	url := "/sapi/v1/sub-account/subAccountApi/ipRestriction"
	reqBody := make(map[string]interface{})
	reqBody["email"] = params.SubName
	reqBody["subAccountApiKey"] = params.APIKey
	p := handyPool.Get()
	defer handyPool.Put(p)
	err.NetworkError = b.call(http.MethodGet, url, reqBody, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			return
		}
		var ips []string
		for _, ip := range value.GetArray("ipList") {
			tempIp := string(ip.GetStringBytes())
			if tempIp == "" {
				continue
			}
			ips = append(ips, tempIp)
		}
		result = transfer.Api{
			SubName: params.SubName,
			Key:     helper.GetStringFromBytes(value, "apiKey"),
			Ip:      strings.Join(ips, ","),
			IPs:     ips,
		}
	}, false)
	return
}

func (b *BinanceSpotRs) GetDepositAddress() (assress []transfer.WithDraw, err helper.ApiError) {
	url := "/sapi/v1/capital/deposit/address"
	reqBody := make(map[string]interface{})
	reqBody["coin"] = "USDT"
	reqBody["network"] = "ETH"
	p := handyPool.Get()
	defer handyPool.Put(p)
	var value *fastjson.Value
	err.NetworkError = b.call(http.MethodGet, url, reqBody, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
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
	}, false)

	reqBody["network"] = "TRX"
	err.NetworkError = b.call(http.MethodGet, url, reqBody, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
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
	}, false)
	return
}

func (b *BinanceSpotRs) WithDraw(params transfer.WithDraw) (result string, err helper.ApiError) {
	var uri = "/sapi/v1/capital/withdraw/apply"
	reqBody := map[string]interface{}{
		"coin":    params.Coin,
		"address": params.ToAddress,
		"amount":  fixed.NewF(params.Amount).String(),
		"network": params.Chain, //TRX
	}
	err.NetworkError = b.call(http.MethodPost, uri, nil, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
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
	}, false)
	return
}

func (b *BinanceSpotRs) DoTransferAllDireaction(params transfer.TransferAllDirectionParams) (err helper.ApiError) {
	// Binance 不支持此功能，返回空实现
	return
}

func (b *BinanceSpotRs) WithDrawHistory(param transfer.GetWithDrawHistoryParams) (records []transfer.WithDrawHistory, err helper.ApiError) {
	var uri = "/sapi/v1/capital/withdraw/history"
	reqBody := make(map[string]interface{})
	if param.Start != 0 && param.End != 0 {
		reqBody["startTime"] = param.Start
		reqBody["endTime"] = param.End
	}
	err.NetworkError = b.call(http.MethodGet, uri, reqBody, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
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
	}, false)
	return
}
