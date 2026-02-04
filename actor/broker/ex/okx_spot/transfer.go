package okx_spot

import (
	"errors"
	"fmt"
	"actor/helper"
	"actor/helper/transfer"
	"actor/third/fixed"
	"actor/third/log"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"golang.org/x/exp/slices"
	"strconv"
	"strings"
)

func (rs *OkxSpotRs) DoTransferSub(params transfer.TransferParams) (err helper.ApiError) {
	uri := "/api/v5/asset/transfer"
	reqBody := make(map[string]interface{})
	reqBody["type"] = strings.Split(params.TransferType, "-")[0]
	reqBody["ccy"] = "USDT"
	reqBody["from"] = strings.Split(params.TransferType, "-")[1]
	reqBody["to"] = strings.Split(params.TransferType, "-")[2]
	reqBody["amt"] = strconv.FormatFloat(params.Amount, 'f', -1, 64)
	reqBody["subAcct"] = params.SubAcctName

	_, err.NetworkError = rs.client.post(uri, reqBody, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			log.Errorf("Subtransfer parse error %v", err)
			return
		}
		if helper.GetStringFromBytes(value, "code") != "0" {
			err.HandlerError = fmt.Errorf("Subtransfer err: %s, sub-acct: %d", value, params.SubAcctUid)
			return
		}
		log.Info("Subtransfer success ,transId %v", value.GetArray("data")[0].Get("transId").String())
	})

	if err.NetworkError != nil {
		log.Errorf("Subtransfer network error %v", err)
		return
	}
	return
}

// 母账户主要是资金账户，没必要互转
func (rs *OkxSpotRs) DoTransferMain(params transfer.TransferParams) (err helper.ApiError) {
	uri := "/api/v5/asset/transfer"
	reqBody := make(map[string]interface{})
	reqBody["type"] = strings.Split(params.TransferType, "-")[0]
	reqBody["ccy"] = "USDT"
	reqBody["from"] = strings.Split(params.TransferType, "-")[1]
	reqBody["to"] = strings.Split(params.TransferType, "-")[2]
	reqBody["amt"] = strconv.FormatFloat(params.Amount, 'f', -1, 64)
	reqBody["subAcct"] = params.SubAcctName

	_, err.NetworkError = rs.client.post(uri, reqBody, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			log.Errorf("Subtransfer parse error %v", err)
			return
		}
		if helper.GetStringFromBytes(value, "code") != "0" {
			err.HandlerError = fmt.Errorf("subtransfer err: %v, sub-acct: %v", value, params.SubAcctUid)
			return
		}
		log.Infof("Subtransfer success ,transId %v", value.GetArray("data")[0].Get("transId").String())
	})

	if err.NetworkError != nil {
		log.Errorf("Subtransfer network error %v", err)
		return
	}
	return
}

func (rs *OkxSpotRs) GetSubList() (accountsInfo []transfer.AccountInfo, err helper.ApiError) {
	p := handyPool.Get()
	defer handyPool.Put(p)
	var value *fastjson.Value
	url := "/api/v5/users/subaccount/list"

	_, err.NetworkError = rs.client.get(url, nil, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			return
		}
		if helper.GetStringFromBytes(value, "code") != "0" {
			err.HandlerError = fmt.Errorf("GetSubList err: %s", string(value.String()))
			return
		}
		accounts := helper.MustGetArray(value, "data")
		accountsInfo = make([]transfer.AccountInfo, 0, len(accounts))
		for _, v := range accounts {
			account := transfer.AccountInfo{
				SubUid:  v.Get("uid").String(),
				SubName: v.Get("subAcct").String(),
				Note:    v.Get("label").String(),
			}
			accountsInfo = append(accountsInfo, account)
		}
	})
	return
}

// 交易所暂时不支持
func (rs *OkxSpotRs) CreateSubAcct(params transfer.CreateSubAcctParams) (accountsInfo []transfer.AccountInfo, err helper.ApiError) {
	return
}

// 交易所暂时不支持
func (rs *OkxSpotRs) CreateSubAPI(params transfer.APIOperateParams) (result transfer.Api, err helper.ApiError) {
	return
}

func (rs *OkxSpotRs) ModifySubAPI(params transfer.APIOperateParams) (result transfer.Api, err helper.ApiError) {
	url := "/api/v5/users/subaccount/modify-apikey"
	reqBody := make(map[string]interface{})
	reqBody["label"] = params.Note
	reqBody["subAcct"] = params.SubName
	reqBody["apiKey"] = params.APIKey
	reqBody["ip"] = params.IP

	_, err.NetworkError = rs.client.post(url, reqBody, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			return
		}
		if helper.GetStringFromBytes(value, "code") != "0" {
			log.Infof("ModifySubAPI errr: %+v", value.String())
			err.HandlerError = fmt.Errorf("ModifySubAPI err: %s, sub-acct: %s", value, params.UID)
		} else {
			data := value.GetArray("data")
			result.Subuid = fmt.Sprintf("%d", params.UID)
			result.Key = data[0].Get("apiKey").String()
			result.Remark = data[0].Get("label").String()
			result.Permission = data[0].Get("perm").String()
			result.Ip = data[0].Get("ip").String()
			result.SubName = data[0].Get("subAcct").String()
			log.Infof("ModifySubAPI success, params: %+v", result)
		}
	})
	return
}

func (rs *OkxSpotRs) GetMainBalance(params transfer.GetMainBalanceParams) (balance float64, err helper.ApiError) {
	if params.AccountType == "trade" {
		url := "/api/v5/account/balance"
		reqBody := make(map[string]interface{})
		reqBody["ccy"] = "USDT"
		_, err.NetworkError = rs.client.get(url, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
			p := handyPool.Get()
			defer handyPool.Put(p)
			var value *fastjson.Value
			value, err.HandlerError = p.ParseBytes(respBody)
			if err.HandlerError != nil {
				return
			}
			log.Infof("get main trade acct balance resp: %s", value.String())
			if helper.GetStringFromBytes(value, "code") != "0" {
				err.HandlerError = errors.New(string(respBody))
				return
			}
			data := value.GetArray("data")[0].GetArray("details")
			balance = helper.MustGetFloat64(data[0], "availBal")
			log.Infof("get main trade acct balance: %d", balance)
		})
		return
	}
	url := "/api/v5/asset/balances"
	reqBody := make(map[string]interface{})
	reqBody["ccy"] = "USDT"
	_, err.NetworkError = rs.client.get(url, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			return
		}
		log.Infof("get main balance resp: %s", value.String())
		if helper.GetStringFromBytes(value, "code") != "0" {
			err.HandlerError = errors.New(string(respBody))
			return
		}
		data := value.GetArray("data")
		balance = helper.MustGetFloat64(data[0], "availBal")
		log.Infof("get main acct balance: %d", balance)
	})
	return
}

func (rs *OkxSpotRs) GetBalance(params transfer.GetBalanceParams) (balance float64, err helper.ApiError) {
	url := "/api/v5/account/subaccount/balances"
	reqBody := make(map[string]interface{})
	reqBody["subAcct"] = params.SubAcctName
	_, err.NetworkError = rs.client.get(url, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			log.Errorf("GetBalance p.ParseBytes error %s", err.HandlerError.Error())
			return
		}
		if helper.GetStringFromBytes(value, "code") != "0" {
			err.HandlerError = fmt.Errorf("GetBalance err, sub-acct: %s", params.SubAcctUid)
			return
		}
		data := value.GetArray("data")[0].GetArray("details")
		for _, v := range data {
			if strings.Index(v.Get("ccy").String(), "USDT") > -1 {
				balance = helper.MustGetFloat64(v, "availBal")
			}
		}
		log.Infof("GetBalance success: %+v", value.String())
	})
	return
}

func (rs *OkxSpotRs) DoTransferSubInner(params transfer.TransferParams) (err helper.ApiError) {
	uri := "/api/v5/asset/subaccount/transfer"
	reqBody := make(map[string]interface{})
	reqBody["ccy"] = "USDT"
	reqBody["from"] = strings.Split(params.TransferType, "-")[0]
	reqBody["to"] = strings.Split(params.TransferType, "-")[1]
	reqBody["amt"] = strconv.FormatFloat(params.Amount, 'f', -1, 64)
	reqBody["fromSubAccount"] = params.SubAcctName
	reqBody["toSubAccount"] = params.ToSubAcctName

	_, err.NetworkError = rs.client.post(uri, reqBody, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			log.Errorf("Subtransfer parse error %v", err)
			return
		}
		if helper.GetStringFromBytes(value, "code") != "0" {
			err.HandlerError = fmt.Errorf("Subtransfer err: %s, sub-acct: %d", value, params.SubAcctUid)
			return
		}
		log.Info("Sub to  sub transfer success ,transId %v", value.GetArray("data")[0].Get("transId").String())
	})

	if err.NetworkError != nil {
		log.Errorf("Sub to  sub transfer network error %v", err)
		return
	}
	return
}

func (rs *OkxSpotRs) GetAssert(params transfer.GetBalanceParams) (accountsInfo []transfer.AccountInfo, err helper.ApiError) {
	url := "/api/v5/account/subaccount/balances"
	reqBody := make(map[string]interface{})
	reqBody["subAcct"] = params.SubAcctName
	_, err.NetworkError = rs.client.get(url, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			log.Errorf("GetBalance p.ParseBytes error %s", err.HandlerError.Error())
			return
		}
		if helper.GetStringFromBytes(value, "code") != "0" {
			err.HandlerError = fmt.Errorf("GetBalance err, sub-acct: %s", params.SubAcctUid)
			return
		}
		var balance float64
		for _, v := range value.GetArray("data")[0].GetArray("details") {
			balance = balance + helper.MustGetFloat64(v, "eqUsd")
		}
		accountsInfo = append(accountsInfo, transfer.AccountInfo{
			SubName: params.SubAcctName,
			Assert:  balance,
		})
	})
	return
}

func (rs *OkxSpotRs) GetDepositAddress() (assress []transfer.WithDraw, err helper.ApiError) {
	url := "/api/v5/asset/deposit-address?ccy=USDT"
	_, err.NetworkError = rs.client.get(url, map[string]interface{}{}, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			log.Errorf("GetDepositAddress p.ParseBytes error %s", err.HandlerError.Error())
			return
		}
		if helper.GetStringFromBytes(value, "code") != "0" {
			err.HandlerError = fmt.Errorf("GetDepositAddress err%s", value.String())
			return
		}
		log.Infof("=======respBody============ %s", value.String())
		for _, item := range value.GetArray("data") {
			chain := helper.GetStringFromBytes(item, "chain")
			if slices.Contains([]string{"USDT-TRC20", "USDT-ERC20"}, chain) {
				assress = append(assress, transfer.WithDraw{
					Coin:      "USDT",
					Chain:     chain,
					ToAddress: helper.GetStringFromBytes(item, "addr"),
				})
			}
		}
	})
	return
}

func (b *OkxSpotRs) WithDraw(params transfer.WithDraw) (result string, err helper.ApiError) {
	var uri = "/api/v5/asset/withdrawal"
	query := map[string]interface{}{
		"ccy":    params.Coin,
		"amt":    fixed.NewF(params.Amount).String(),
		"dest":   "4", //提币方式 3：内部转账 4：链上提币
		"toAddr": params.ToAddress,
		"chain":  params.Chain, //"BTC-Bitcoin",USDT-ERC20，USDT-TRC20
		"rcvrInfo": map[string]interface{}{
			"walletType":    "private", // or "exchange"
			"rcvrFirstName": "Jige",
			"rcvrLastName":  "Omni",
		},
	}
	_, err.NetworkError = b.client.post(uri, query, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			log.Errorf("Withdraw parse error %v", err)
			return
		}
		log.Infof("==query:%v==respBody===%v=======", query, string(respBody))
		result = value.String()
	})
	return
}

func (b *OkxSpotRs) WithDrawHistory(param transfer.GetWithDrawHistoryParams) (records []transfer.WithDrawHistory, err helper.ApiError) {
	var uri = "/api/v5/asset/withdrawal-history"
	reqBody := make(map[string]interface{})
	reqBody["limit"] = "100"
	if param.Start != 0 {
		reqBody["before"] = fmt.Sprintf("%v", param.Start)
	}
	if param.End != 0 {
		reqBody["after"] = fmt.Sprintf("%v", param.End)
	}
	_, err.NetworkError = b.client.get(uri, reqBody, true, func(respBody []byte, respHeader *fasthttp.ResponseHeader) {
		p := handyPool.Get()
		defer handyPool.Put(p)
		var value *fastjson.Value
		value, err.HandlerError = p.ParseBytes(respBody)
		if err.HandlerError != nil {
			log.Errorf("Withdraw parse error %v", err)
			return
		}
		log.Infof("====respBody===%v=======", string(respBody))
		if helper.BytesToString(value.GetStringBytes("code")) != "0" {
			err.HandlerError = fmt.Errorf("WithDrawHistory err: %s", value.String())
			return
		}
		for _, item := range value.GetArray("data") {
			if helper.GetStringFromBytes(item, "state") == "2" {
				records = append(records, transfer.WithDrawHistory{
					Platform:     "okx",
					Account:      param.Account,
					ApplyTime:    helper.GetInt64FromBytes(item, "ts"),
					CompleteTime: helper.GetInt64FromBytes(item, "ts"),
					Amount:       helper.GetFloat64FromBytes(item, "amt"),
					Address:      helper.GetStringFromBytes(item, "to"),
					OrderID:      helper.GetStringFromBytes(item, "txId"),
					Currency:     helper.GetStringFromBytes(item, "ccy"),
				})
			}
		}
	})
	return
}

func (rs *OkxSpotRs) DoTransferAllDireaction(params transfer.TransferAllDirectionParams) (err helper.ApiError) {
	// OKX 不支持此功能，返回空实现
	return
}
