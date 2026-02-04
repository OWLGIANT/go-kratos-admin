package bitget_spot

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"actor/helper"
	"actor/helper/transfer"
	"actor/third/log"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
)

// ================================账户===========================================================
func (b *BitgetSpot) GetBalance(params transfer.GetBalanceParams) (balance float64, err helper.ApiError) {
	url := "/api/spot/v1/account/assets" //獲取賬戶信息，適用於現貨交易市場
	reqBody := make(map[string]interface{})
	reqBody["coin"] = "USDT"
	err.NetworkError = b.call(http.MethodGet, url, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			return
		}
		if helper.GetStringFromBytes(value, "code") != "00000" {
			err.HandlerError = fmt.Errorf("GetBalance err: %s", value.String())
			return
		}
		for _, v := range helper.GetArray(value, "data") {
			if strings.EqualFold(string(v.GetStringBytes("coinName")), "USDT") {
				sBalance := string(v.GetStringBytes("available"))
				if sBalance != "" {
					balance, _ = strconv.ParseFloat(sBalance, 64)
				}
			}
		}
	})
	return
}

func (b *BitgetSpot) GetMainBalance(params transfer.GetMainBalanceParams) (balance float64, err helper.ApiError) {
	url := "/api/spot/v1/account/assets"
	reqBody := make(map[string]interface{})
	reqBody["coin"] = "USDT"
	err.NetworkError = b.call(http.MethodGet, url, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			return
		}
		if helper.GetStringFromBytes(value, "code") != "00000" {
			err.HandlerError = fmt.Errorf("GetMainBalance err: %s", value.String())
			return
		}
		for _, v := range helper.GetArray(value, "data") {
			if strings.EqualFold(string(v.GetStringBytes("coinName")), "USDT") {
				sBalance := string(v.GetStringBytes("available"))
				if sBalance != "" {
					balance, _ = strconv.ParseFloat(sBalance, 64)
				}
			}
		}
	})
	return
}

func (b *BitgetSpot) DoTransferMain(params transfer.TransferParams) (err helper.ApiError) {
	var uri = "/api/v2/spot/wallet/transfer"
	reqBody := make(map[string]interface{})
	reqBody["fromType"] = params.Source
	reqBody["toType"] = params.Target
	reqBody["coin"] = params.Asset
	reqBody["amount"] = fmt.Sprintf("%f", params.Amount)
	reqBody["fromUserId"] = params.SubAcctUid
	reqBody["toUserId"] = params.ToMainAcctUid
	log.Infof("DoTransferMain params: %v", params)
	err.NetworkError = b.call(http.MethodPost, uri, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			log.Errorf("DoTransferSub parse error %v", err)
			return
		}
		if value.GetInt("code") != 00000 {
			err.HandlerError = fmt.Errorf("DoTransferMain err: %s", value.String())
			return
		}
	})
	if err.NetworkError != nil {
		log.Errorf("DoTransferSub network error %v", err)
		return
	}
	return
}

func (b *BitgetSpot) DoTransferSub(params transfer.TransferParams) (err helper.ApiError) {
	var uri = "/api/v2/spot/wallet/subaccount-transfer"
	reqBody := make(map[string]interface{})
	reqBody["fromType"] = params.Source //usdt_futures
	reqBody["toType"] = params.Target
	reqBody["coin"] = params.Asset
	reqBody["amount"] = fmt.Sprintf("%f", params.Amount)
	if params.TransferDirection == "in" {
		reqBody["fromUserId"] = params.SubAcctUid
		reqBody["toUserId"] = params.MainAcctUid
	} else if params.TransferDirection == "out" {
		reqBody["fromUserId"] = params.MainAcctUid
		reqBody["toUserId"] = params.SubAcctUid
	}
	err.NetworkError = b.call(http.MethodPost, uri, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			log.Errorf("DoTransferSub parse error %v", err)
			return
		}
		if helper.BytesToString(value.GetStringBytes("code")) != "00000" {
			err.HandlerError = fmt.Errorf("DoTransferSub err: %s", value.String())
			return
		}
	})
	return
}

func (b *BitgetSpot) GetSubList() (accountsInfo []transfer.AccountInfo, err helper.ApiError) {
	p := handyPool.Get()
	defer handyPool.Put(p)
	var value *fastjson.Value
	url := "/api/user/v1/sub/virtual-list"
	reqBody := make(map[string]interface{})
	reqBody["pageSize"] = 100
	uidMap := make(map[string]bool)
	for i := 1; i <= 2; i++ {
		reqBody["pageNo"] = i
		err.NetworkError = b.call(http.MethodGet, url, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
			value, err.HandlerError = p.ParseBytes(respBody)
			if err.HandlerError != nil {
				return
			}
			if helper.BytesToString(value.GetStringBytes("code")) != "00000" {
				err.HandlerError = fmt.Errorf("GetSubList err: %s", value.String())
				return
			}
			accounts := helper.MustGetArray(value, "data")
			for _, v := range accounts {
				account := transfer.AccountInfo{
					SubUid:     v.Get("subUid").String(),
					SubName:    v.Get("subName").String(),
					Permission: v.Get("auth").String(),
				}
				if !uidMap[account.SubUid] {
					accountsInfo = append(accountsInfo, account)
				}
				uidMap[account.SubUid] = true
			}
		})
	}

	return
}

func (b *BitgetSpot) CreateSubAcct(params transfer.CreateSubAcctParams) (accountsInfo []transfer.AccountInfo, err helper.ApiError) {
	url := "/api/user/v1/sub/virtual-create"
	reqBody := make(map[string]interface{})
	reqBody["subName"] = []string{params.SubName}
	err.NetworkError = b.call(http.MethodPost, url, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			return
		}
		if helper.BytesToString(value.GetStringBytes("code")) != "00000" {
			err.HandlerError = fmt.Errorf("CreateSubAcct err: %s", value.String())
			return
		}
		accounts := helper.MustGetArray(value.Get("data"), "successAccounts")
		accountsInfo = make([]transfer.AccountInfo, 0, len(accounts))
		for _, v := range accounts {
			account := transfer.AccountInfo{
				SubUid:     v.Get("subUid").String(),
				SubName:    v.Get("subName").String(),
				Permission: v.Get("auth").String(),
			}
			accountsInfo = append(accountsInfo, account)
		}
	})
	return
}

func (b *BitgetSpot) CreateSubAPI(params transfer.APIOperateParams) (result transfer.Api, err helper.ApiError) {
	url := "/api/v2/user/create-virtual-subaccount-apikey"
	reqBody := make(map[string]interface{})
	reqBody["subAccountUid"] = params.UID
	reqBody["label"] = params.Note
	reqBody["passphrase"] = b.BrokerConfig.PassKey
	reqBody["permList"] = []string{"spot_trade", "margin_trade", "contract_trade"}
	if params.IP != "" {
		ipSet := mapset.NewSet[string]()
		for _, ip := range strings.Split(params.IP, ",") {
			if ip != "" {
				ipSet.Add(ip)
			}
		}
		if ipSet.Cardinality() > 0 {
			reqBody["ipList"] = ipSet.ToSlice()
		}
	}
	err.NetworkError = b.call(http.MethodPost, url, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			return
		}
		if helper.BytesToString(value.GetStringBytes("code")) == "00000" {
			data := value.Get("data")
			var ips []string
			for _, ip := range helper.GetArray(data, "ipList") {
				ips = append(ips, helper.GetStringFromBytes(ip))
			}
			result = transfer.Api{
				SubName:    params.SubName,
				Subuid:     helper.GetStringFromBytes(data, "subAccountUid"),
				Key:        helper.GetStringFromBytes(data, "subAccountApiKey"),
				Secret:     helper.GetStringFromBytes(data, "secretKey"),
				Passphrase: b.BrokerConfig.PassKey,
				Remark:     helper.GetStringFromBytes(data, "label"),
				Ip:         strings.Join(ips, ","),
				IPs:        ips,
			}
		} else {
			err.HandlerError = fmt.Errorf("CreateSubAPI err: %s, sub-acct: %d", value.String(), params.UID)
		}
	})
	return
}

func (b *BitgetSpot) ModifySubAPI(params transfer.APIOperateParams) (result transfer.Api, err helper.ApiError) {
	url := "/api/v2/user/modify-virtual-subaccount-apikey"
	reqBody := make(map[string]interface{})
	reqBody["subAccountUid"] = params.UID
	reqBody["subAccountApiKey"] = params.APIKey
	reqBody["label"] = params.Note
	reqBody["passphrase"] = b.BrokerConfig.PassKey
	reqBody["permList"] = []string{"spot_trade", "margin_trade", "contract_trade"}
	result, err = b.GetSubSpecificAPI(params)
	if !err.Nil() {
		return
	}
	ipSet := mapset.NewSet[string]()
	if result.Key != "" {
		for _, ip := range result.IPs {
			if ip != "" {
				ipSet.Add(ip)
			}
		}
		if params.IP != "" {
			for _, ip := range strings.Split(params.IP, ",") {
				if ip != "" {
					ipSet.Add(ip)
				}
			}
		}
	}
	if ipSet.Cardinality() > 0 {
		reqBody["ipList"] = ipSet.ToSlice()
	} else {
		return
	}
	err.NetworkError = b.call(http.MethodPost, url, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			return
		}
		log.Infof("====ModifySubAPI==respBody=%v====", value.String())
		if helper.BytesToString(value.GetStringBytes("code")) == "00000" {
			data := value.Get("data")
			var ips []string
			for _, ip := range helper.GetArray(data, "ipList") {
				ips = append(ips, helper.GetStringFromBytes(ip))
			}
			result = transfer.Api{
				SubName:    params.SubName,
				Subuid:     helper.GetStringFromBytes(data, "subAccountUid"),
				Key:        helper.GetStringFromBytes(data, "subAccountApiKey"),
				Passphrase: b.BrokerConfig.PassKey,
				Remark:     helper.GetStringFromBytes(data, "label"),
				Ip:         strings.Join(ips, ","),
				IPs:        ips,
			}
		} else {
			err.HandlerError = fmt.Errorf("ModifySubAPI err: %s, sub-acct: %v", value.String(), params.UID)
		}
	})
	return
}

func (b *BitgetSpot) DoTransferSubInner(params transfer.TransferParams) (err helper.ApiError) {
	var uri = "/api/v2/spot/wallet/subaccount-transfer"
	reqBody := make(map[string]interface{})
	reqBody["fromType"] = params.Source
	reqBody["toType"] = params.Target
	reqBody["coin"] = "USDT"
	reqBody["amount"] = fmt.Sprintf("%f", params.Amount)
	reqBody["fromUserId"] = params.SubAcctUid
	reqBody["toUserId"] = params.ToSubAcctUid
	err.NetworkError = b.call(http.MethodPost, uri, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			log.Errorf("DoTransferSub parse error %v", err)
			return
		}
		if helper.BytesToString(value.GetStringBytes("code")) != "00000" {
			err.HandlerError = fmt.Errorf("GetBalance err: %s", value.String())
			return
		}
	})
	return
}

func (b *BitgetSpot) GetLVT() (ltvInfo string, err helper.ApiError) {
	uri := "/api/v2/spot/ins-loan/ltv-convert"
	err.NetworkError = b.call(http.MethodGet, uri, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			log.Errorf("GetLVT parse err: %s", err.String())
			return
		}
		if helper.BytesToString(value.GetStringBytes("code")) != "00000" {
			err.HandlerError = fmt.Errorf("GetLVT err: %s", value.String())
			return
		}
		ltvInfo = value.Get("data").String()
	})
	return
}

func (b *BitgetSpot) GetLoanAccountInfo() (accountInfo string, err helper.ApiError) {
	return
}

func (b *BitgetSpot) Withdrawal() (ltvInfo string, err helper.ApiError) {
	uri := "/api/spot/v1/wallet/withdrawal-v2"
	reqBody := make(map[string]interface{})
	reqBody["coin"] = "USDT"
	reqBody["address"] = "TG2HZiVvj47MNGnwNhiHZHeuGkw33WMzUi"
	reqBody["chain"] = "trc20"
	reqBody["amount"] = "1034.39"
	err.NetworkError = b.call(http.MethodPost, uri, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			log.Errorf("GetLVT parse err: %s", err.String())
			return
		}
		if helper.BytesToString(value.GetStringBytes("code")) != "00000" {
			err.HandlerError = fmt.Errorf("GetLVT err: %s", value.String())
			return
		}
		ltvInfo = value.Get("data").String()
	})
	return
}

func (b *BitgetSpot) GetSubSpecificAPI(params transfer.APIOperateParams) (result transfer.Api, err helper.ApiError) {
	url := "/api/user/v1/sub/virtual-api-list"
	reqBody := make(map[string]interface{})
	reqBody["subUid"] = params.UID
	err.NetworkError = b.call(http.MethodGet, url, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			return
		}
		if helper.BytesToString(value.GetStringBytes("code")) == "00000" {
			for _, item := range value.GetArray("data") {
				if helper.GetStringFromBytes(item, "apiKey") == params.APIKey {
					var ip = helper.GetStringFromBytes(item, "ip")
					result = transfer.Api{
						SubName:    params.SubName,
						Subuid:     helper.GetStringFromBytes(item, "subUid"),
						Key:        helper.GetStringFromBytes(item, "apiKey"),
						Passphrase: b.BrokerConfig.PassKey,
						Remark:     helper.GetStringFromBytes(item, "label"),
						Ip:         ip,
						IPs:        strings.Split(ip, ","),
					}
				}
			}
		} else {
			err.HandlerError = fmt.Errorf("ModifySubAPI err: %s, sub-acct: %v", value.String(), params.UID)
		}
	})
	return
}

func (rs *BitgetSpot) GetDepositAddress() (assress []transfer.WithDraw, err helper.ApiError) {
	url := "/api/v2/spot/wallet/deposit-address"
	reqBody := make(map[string]interface{})
	reqBody["coin"] = "USDT"
	reqBody["chain"] = "trc20"
	err.NetworkError = rs.call(http.MethodGet, url, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			log.Errorf("GetDepositAddress p.ParseBytes error %s", err.HandlerError.Error())
			return
		}
		if helper.GetStringFromBytes(value, "code") != "00000" {
			err.HandlerError = fmt.Errorf("GetDepositAddress err response: %s", value.String())
			return
		}
		chain := helper.GetStringFromBytes(value, "data", "chain")
		assress = append(assress, transfer.WithDraw{
			Coin:      "USDT",
			Chain:     chain,
			ToAddress: helper.GetStringFromBytes(value, "data", "address"),
		})
	})

	reqBody["chain"] = "erc20"
	err.NetworkError = rs.call(http.MethodGet, url, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			log.Errorf("GetDepositAddress p.ParseBytes error %s", err.HandlerError.Error())
			return
		}
		if helper.GetStringFromBytes(value, "code") != "00000" {
			err.HandlerError = fmt.Errorf("GetDepositAddress err response: %s", value.String())
			return
		}
		chain := helper.GetStringFromBytes(value, "data", "chain")
		assress = append(assress, transfer.WithDraw{
			Coin:      "USDT",
			Chain:     chain,
			ToAddress: helper.GetStringFromBytes(value, "data", "address"),
		})
	})

	return
}

func (b *BitgetSpot) WithDraw(params transfer.WithDraw) (result string, err helper.ApiError) {
	var uri = "/api/v2/spot/wallet/withdrawal"
	reqBody := make(map[string]interface{})
	reqBody["address"] = params.ToAddress
	reqBody["chain"] = params.Chain
	reqBody["coin"] = params.Coin
	reqBody["size"] = fmt.Sprintf("%f", params.Amount)
	reqBody["transferType"] = "on_chain"
	reqBody["clientOid"] = fmt.Sprint(time.Now().UnixMilli())
	log.Infof("WithDraw params: %v", params)
	err.NetworkError = b.call(http.MethodPost, uri, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			log.Errorf("Withdraw parse error %v", err)
			return
		}
		if helper.BytesToString(value.GetStringBytes("code")) != "00000" {
			err.HandlerError = fmt.Errorf("Withdraw err: %s", value.String())
			return
		}

		log.Infof("withdraw :%s", value.String())
		result = value.String()
	})
	return
}


func (b *BitgetSpot) WithDrawHistory(param transfer.GetWithDrawHistoryParams) (records []transfer.WithDrawHistory, err helper.ApiError) {
	var uri = "/api/v2/spot/wallet/withdrawal-records"
	reqBody := make(map[string]interface{})
	reqBody["limit"] = "100"
	reqBody["startTime"] = fmt.Sprintf("%v", param.Start)
	reqBody["endTime"] = fmt.Sprintf("%v", param.End)
	err.NetworkError = b.call(http.MethodGet, uri, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			log.Errorf("Withdraw parse error %v", err)
			return
		}
		if helper.BytesToString(value.GetStringBytes("code")) != "00000" {
			err.HandlerError = fmt.Errorf("WithDrawHistory err: %s", value.String())
			return
		}
		for _, item := range value.GetArray("data") {
			if helper.GetStringFromBytes(item, "status") == "success" {
				records = append(records, transfer.WithDrawHistory{
					Platform:     "bitget",
					Account:      param.Account,
					ApplyTime:    helper.GetInt64FromBytes(item, "cTime"),
					CompleteTime: helper.GetInt64FromBytes(item, "uTime"),
					Amount:       helper.GetFloat64FromBytes(item, "size"),
					Address:      helper.GetStringFromBytes(item, "toAddress"),
					OrderID:      helper.GetStringFromBytes(item, "orderId"),
					Currency:     helper.GetStringFromBytes(item, "coin"),
				})
			}
		}
	})
	return
}

func (b *BitgetSpot) DoTransferAllDireaction(params transfer.TransferAllDirectionParams) (err helper.ApiError) {
	// Bitget 不支持此功能，返回空实现
	return
}
