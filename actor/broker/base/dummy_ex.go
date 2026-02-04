package base

import (
	"errors"

	"actor/broker/client/ws"
	"actor/helper"
	"actor/helper/transfer"
	"actor/third/log"
)

type DummyGetTicker struct {
}

func (rs *DummyGetTicker) GetTickerByPair(pair *helper.Pair) (ticker helper.Ticker, err helper.ApiError) {
	err.HandlerError = &helper.ApiErrorNotImplemented
	return
}

type DummyRs struct{}

func (rs *DummyRs) GetExchangeInfos() []helper.ExchangeInfo {
	panic("not support GetExchangeInfos in dummy rs")
}

type DummyDoSetLeverage struct{}

func (rs *DummyDoSetLeverage) DoSetLeverage(pairInfo helper.ExchangeInfo, leverage int) (err helper.ApiError) {
	log.Error("not support SetLeverage in spot")
	err.HandlerError = errors.New("not support SetLeverage in spot")
	return
}

type DummyDoSetMarginMode struct{}

func (rs *DummyDoSetMarginMode) DoSetMarginMode(symbol string, marginMode helper.MarginMode) helper.ApiError {
	return helper.ApiErrorWithHandlerError("not support SetMarginMode")
}

type DummyDoSetPositionMode struct{}

func (rs *DummyDoSetPositionMode) DoSetPositionMode(symbol string, positionMode helper.PosMode) helper.ApiError {
	return helper.ApiErrorWithHandlerError("not support SetPositionMode")
}

type DummyDoGetDepthOI struct{}

func (rs *DummyDoGetDepthOI) DoGetDepth(info *helper.ExchangeInfo) (respDepth helper.Depth, err helper.ApiError) {
	return helper.Depth{}, helper.ApiErrorWithHandlerError("not support get depth")
}
func (rs *DummyDoGetDepthOI) DoGetOI(info *helper.ExchangeInfo) (oi float64, err helper.ApiError) {
	return 0, helper.ApiErrorWithHandlerError("not support get oi")
}

type DummyDoForSpot struct {
	DummyDoSetLeverage
	DummyDoSetMarginMode
	DummyDoSetPositionMode
	DummyDoGetDepthOI
}

type DummyCreateReqWs struct{}

func (rs *DummyCreateReqWs) DoCreateReqWsNor() error {
	return errors.New("not implement")
}
func (rs *DummyCreateReqWs) DoCreateReqWsColo() error {
	return errors.New("not implement")
}

type DummyDoCancelOrdersIfPresent struct{}

func (rs *DummyDoCancelOrdersIfPresent) DoCancelOrdersIfPresent(only bool) (hasPendingOrderBefore bool) {
	panic("not tradeable")
}

type DummyDoForUnTradeableEx struct {
	DummyDoCancelOrdersIfPresent
}

type DummyDoCancelPendingOrders struct{}

func (rs *DummyDoCancelPendingOrders) DoCancelPendingOrders(symbol string) helper.ApiError {
	panic("not tradeable")
}

type DummyDoGetPriceLimit struct{}

func (f *DummyDoGetPriceLimit) DoGetPriceLimit(symbol string) (helper.PriceLimit, helper.ApiError) {
	panic("not implemented")
}

type DummyGetPriWs struct{}

func (f *DummyGetPriWs) GetPriWs() *ws.WS {
	panic("not implemented")
}

type DummyGetAllPendingOrders struct{}

func (rs *DummyGetAllPendingOrders) GetAllPendingOrders() (resp []helper.OrderForList, err helper.ApiError) {
	panic("not implemented")
}

type DummyDoGetPendingOrders struct{}

func (rs *DummyDoGetPendingOrders) DoGetPendingOrders(symbol string) (results []helper.OrderForList, err helper.ApiError) {
	panic("not implemented")
}

type DummyTransfer struct{}

func (rs *DummyTransfer) GetSubList() ([]transfer.AccountInfo, helper.ApiError) {
	panic("not implemented")
}
func (rs *DummyTransfer) DoTransferSubInner(params transfer.TransferParams) helper.ApiError {
	panic("not implemented")
}
func (rs *DummyTransfer) DoTransferSub(params transfer.TransferParams) helper.ApiError {
	panic("not implemented")
}
func (rs *DummyTransfer) DoTransferAllDireaction(params transfer.TransferAllDirectionParams) helper.ApiError {
	panic("not implemented")
}
func (rs *DummyTransfer) DoTransferMain(params transfer.TransferParams) helper.ApiError {
	panic("not implemented")
}
func (rs *DummyTransfer) CreateSubAcct(params transfer.CreateSubAcctParams) ([]transfer.AccountInfo, helper.ApiError) {
	panic("not implemented")
}
func (rs *DummyTransfer) CreateSubAPI(params transfer.APIOperateParams) (transfer.Api, helper.ApiError) {
	panic("not implemented")
}
func (rs *DummyTransfer) ModifySubAPI(params transfer.APIOperateParams) (transfer.Api, helper.ApiError) {
	panic("not implemented")
}
func (rs *DummyTransfer) GetBalance(params transfer.GetBalanceParams) (float64, helper.ApiError) {
	panic("not implemented")
}
func (rs *DummyTransfer) GetMainBalance(params transfer.GetMainBalanceParams) (float64, helper.ApiError) {
	panic("not implemented")
}
func (rs *DummyTransfer) DoGetAcctSum() (acctSum helper.AcctSum, err helper.ApiError) {
	panic("not implemented")
}
func (rs *DummyTransfer) GetAssert(params transfer.GetBalanceParams) (accountsInfo []transfer.AccountInfo, err helper.ApiError) {
	panic("not implemented")
}
func (rs *DummyTransfer) GetDepositAddress() (assress []transfer.WithDraw, err helper.ApiError) {
	panic("not implemented")
}
func (rs *DummyTransfer) WithDraw(param transfer.WithDraw) (restlt string, err helper.ApiError) {
	panic("not implemented")
}
func (rs *DummyTransfer) WithDrawHistory(param transfer.GetWithDrawHistoryParams) (records []transfer.WithDrawHistory, err helper.ApiError) {
	panic("not implemented")
}
