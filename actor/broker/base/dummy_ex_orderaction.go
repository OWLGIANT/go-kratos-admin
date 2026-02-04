package base

import (
	"actor/helper"
)

// 提醒 ！！！ 不可随意改名，有运行时反射依赖名字，无法检测。修改必须线下确认

type DummyDoPlaceOrderRsColo struct {
}

func (f *DummyDoPlaceOrderRsColo) DoPlaceOrderRsColo(info *helper.ExchangeInfo, s helper.Signal) {
	panic("should not here")
}

type DummyDoCancelOrderRsColo struct {
}

func (f *DummyDoCancelOrderRsColo) DoCancelOrderRsColo(info *helper.ExchangeInfo, s helper.Signal) {
	panic("shoul not here")
}

type DummyDoAmendOrderRsColo struct {
}

func (f *DummyDoAmendOrderRsColo) DoAmendOrderRsColo(info *helper.ExchangeInfo, s helper.Signal) {
	panic("should not here")
}

type DummyDoPlaceOrderWsColo struct {
}

func (f *DummyDoPlaceOrderWsColo) DoPlaceOrderWsColo(info *helper.ExchangeInfo, s helper.Signal) {
	panic("should not here")
}

type DummyDoCancelOrderWsColo struct {
}

func (f *DummyDoCancelOrderWsColo) DoCancelOrderWsColo(info *helper.ExchangeInfo, s helper.Signal) {
	panic("should not here")
}

type DummyDoAmendOrderWsColo struct {
}

func (f *DummyDoAmendOrderWsColo) DoAmendOrderWsColo(info *helper.ExchangeInfo, s helper.Signal) {
	panic("should not here")
}

type DummyDoPlaceOrderWsNor struct {
}

func (f *DummyDoPlaceOrderWsNor) DoPlaceOrderWsNor(info *helper.ExchangeInfo, s helper.Signal) {
	panic("should not here")
}

type DummyDoCancelOrderWsNor struct {
}

func (f *DummyDoCancelOrderWsNor) DoCancelOrderWsNor(info *helper.ExchangeInfo, s helper.Signal) {
	panic("should not here")
}

type DummyDoAmendOrderWsNor struct {
}

func (f *DummyDoAmendOrderWsNor) DoAmendOrderWsNor(info *helper.ExchangeInfo, s helper.Signal) {
	panic("should not here")
}

type DummyDoAmendOrderRsNor struct {
}

func (f *DummyDoAmendOrderRsNor) DoAmendOrderRsNor(info *helper.ExchangeInfo, s helper.Signal) {
	panic("should not here")
}

type DummyDoPlaceOrderRsNor struct {
}

func (f *DummyDoPlaceOrderRsNor) DoPlaceOrderRsNor(info *helper.ExchangeInfo, s helper.Signal) {
	panic("should not here")
}

type DummyDoCancelOrderRsNor struct {
}

func (f *DummyDoCancelOrderRsNor) DoCancelOrderRsNor(info *helper.ExchangeInfo, s helper.Signal) {
	panic("shoul not here")
}

type DummyOrderActionRsColo struct {
	DummyDoPlaceOrderRsColo
	DummyDoAmendOrderRsColo
	DummyDoCancelOrderRsColo
}
type DummyOrderActionWsColo struct {
	DummyDoPlaceOrderWsColo
	DummyDoAmendOrderWsColo
	DummyDoCancelOrderWsColo
}
type DummyOrderActionWsNor struct {
	DummyDoPlaceOrderWsNor
	DummyDoAmendOrderWsNor
	DummyDoCancelOrderWsNor
}

type DummyOrderActionRsNor struct {
	DummyDoPlaceOrderRsNor
	DummyDoAmendOrderRsNor
	DummyDoCancelOrderRsNor
}

type DummyOrderAction struct {
	DummyOrderActionRsNor
	DummyOrderActionRsColo
	DummyOrderActionWsColo
	DummyOrderActionWsNor
}
type DummyOrderActionAmend struct {
	DummyDoAmendOrderWsNor
	DummyDoAmendOrderRsNor
	DummyDoAmendBatchOrderRsColo
	DummyDoAmendBatchOrderWsColo
	DummyDoAmendBatchOrderRsNor
	DummyDoAmendBatchOrderWsNor
	DummyDoAmendOrderWsColo
	DummyDoAmendOrderRsColo
}
