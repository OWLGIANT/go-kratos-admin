package base

import (
	"actor/helper"
)

// 提醒 ！！！ 不可随意改名，有运行时反射依赖名字，无法检测。修改必须线下确认

type DummyDoPlaceBatchOrderRsColo struct {
}

func (f *DummyDoPlaceBatchOrderRsColo) DoPlaceBatchOrderRsColo(info *helper.ExchangeInfo, sigs []helper.Signal) {
	panic("should not here")
}

type DummyDoCancelBatchOrderRsColo struct {
}

func (f *DummyDoCancelBatchOrderRsColo) DoCancelBatchOrderRsColo(info *helper.ExchangeInfo, sigs []helper.Signal) {
	panic("shoul not here")
}

type DummyDoAmendBatchOrderRsColo struct {
}

func (f *DummyDoAmendBatchOrderRsColo) DoAmendBatchOrderRsColo(info *helper.ExchangeInfo, sigs []helper.Signal) {
	panic("should not here")
}

type DummyDoPlaceBatchOrderWsColo struct {
}

func (f *DummyDoPlaceBatchOrderWsColo) DoPlaceBatchOrderWsColo(info *helper.ExchangeInfo, sigs []helper.Signal) {
	panic("should not here")
}

type DummyDoCancelBatchOrderWsColo struct {
}

func (f *DummyDoCancelBatchOrderWsColo) DoCancelBatchOrderWsColo(info *helper.ExchangeInfo, sigs []helper.Signal) {
	panic("should not here")
}

type DummyDoAmendBatchOrderWsColo struct {
}

func (f *DummyDoAmendBatchOrderWsColo) DoAmendBatchOrderWsColo(info *helper.ExchangeInfo, sigs []helper.Signal) {
	panic("should not here")
}

type DummyDoPlaceBatchOrderWsNor struct {
}

func (f *DummyDoPlaceBatchOrderWsNor) DoPlaceBatchOrderWsNor(info *helper.ExchangeInfo, sigs []helper.Signal) {
	panic("should not here")
}

type DummyDoCancelBatchOrderWsNor struct {
}

func (f *DummyDoCancelBatchOrderWsNor) DoCancelBatchOrderWsNor(info *helper.ExchangeInfo, sigs []helper.Signal) {
	panic("should not here")
}

type DummyDoAmendBatchOrderWsNor struct {
}

func (f *DummyDoAmendBatchOrderWsNor) DoAmendBatchOrderWsNor(info *helper.ExchangeInfo, sigs []helper.Signal) {
	panic("should not here")
}

type DummyDoAmendBatchOrderRsNor struct {
}

func (f *DummyDoAmendBatchOrderRsNor) DoAmendBatchOrderRsNor(info *helper.ExchangeInfo, sigs []helper.Signal) {
	panic("should not here")
}

type DummyDoPlaceBatchOrderRsNor struct {
}

func (f *DummyDoPlaceBatchOrderRsNor) DoPlaceBatchOrderRsNor(info *helper.ExchangeInfo, sigs []helper.Signal) {
	panic("should not here")
}

type DummyDoCancelBatchOrderRsNor struct {
}

func (f *DummyDoCancelBatchOrderRsNor) DoCancelBatchOrderRsNor(info *helper.ExchangeInfo, sigs []helper.Signal) {
	panic("shoul not here")
}

type DummyBatchOrderActionRsColo struct {
	DummyDoPlaceBatchOrderRsColo
	DummyDoAmendBatchOrderRsColo
	DummyDoCancelBatchOrderRsColo
}
type DummyBatchOrderActionWsColo struct {
	DummyDoPlaceBatchOrderWsColo
	DummyDoAmendBatchOrderWsColo
	DummyDoCancelBatchOrderWsColo
}
type DummyBatchOrderActionWsNor struct {
	DummyDoPlaceBatchOrderWsNor
	DummyDoAmendBatchOrderWsNor
	DummyDoCancelBatchOrderWsNor
}

type DummyBatchOrderAction struct {
	DummyDoPlaceBatchOrderRsNor
	DummyDoCancelBatchOrderRsNor
	DummyDoAmendBatchOrderRsNor
	DummyBatchOrderActionRsColo
	DummyBatchOrderActionWsColo
	DummyBatchOrderActionWsNor
}
