package base

import (
	"actor/helper"
)

func (f *FatherRs) SetSelectedLine(link LinkType, client ClientType) {
	for _, v := range []*SelectedLine{&f.SelectedLine_Place, &f.SelectedLine_Amend, &f.SelectedLine_Cancel} {
		v.Link = link
		v.Client = client
	}
}

// fixme 全部支持批量下单后，删除
func (f *FatherRs) PlaceOrderSelect(s helper.Signal) {
	info, ok := f.GetPairInfoByPair(&s.Pair)
	if !ok {
		f.Logger.Warnf("not found pairinfo for pair %s", s.Pair)
		return
	}

	if s.SignalChannelType == helper.SignalChannelTypeRs {
		if f.SelectedLine_Place.Link == LinkType_Colo {
			go f.subRsInner.DoPlaceOrderRsColo(info, s)
		} else {
			go f.subRsInner.DoPlaceOrderRsNor(info, s)
		}
		return
	}
	if f.SelectedLine_Place.Client == ClientType_Rs {
		if f.SelectedLine_Place.Link == LinkType_Colo {
			// if !f.CanColoRs {
			if !f.Features.DoPlaceOrderRsColo {
				f.Logger.Warnf("cannot rs colo, use normal link")
				go f.subRsInner.DoPlaceOrderRsNor(info, s)
			} else {
				go f.subRsInner.DoPlaceOrderRsColo(info, s)
			}
		} else {
			go f.subRsInner.DoPlaceOrderRsNor(info, s)
		}
	} else if f.SelectedLine_Place.Client == ClientType_Ws {
		if f.SelectedLine_Place.Link == LinkType_Colo {
			// if !f.CanColoWs {
			if !f.Features.DoPlaceOrderWsColo {
				f.Logger.Warnf("cannot ws colo, use normal link")
				f.EnsureReqWsNorLogged(func() {
					f.subRsInner.DoPlaceOrderWsNor(info, s)
				}, func() {
					f.Logger.Errorf("[%s] failed to login req ws. ", f.ExchangeName)
					event := helper.OrderEvent{ClientID: s.ClientID, Type: helper.OrderEventTypeERROR}
					event.ErrorReason = "failed to login req ws"
					f.Cb.OnOrder(0, event)
				})
			} else {
				f.EnsureReqWsColoLogged(func() {
					f.subRsInner.DoPlaceOrderWsColo(info, s)
				}, func() {
					f.Logger.Errorf("[%s] failed to login req ws. ", f.ExchangeName)
					event := helper.OrderEvent{ClientID: s.ClientID, Type: helper.OrderEventTypeERROR}
					event.ErrorReason = "failed to login req ws"
					f.Cb.OnOrder(0, event)
				})
			}
		} else {
			f.EnsureReqWsNorLogged(func() {
				f.subRsInner.DoPlaceOrderWsNor(info, s)
			},
				func() {
					f.Logger.Errorf("[%s] failed to login req ws. ", f.ExchangeName)
					event := helper.OrderEvent{ClientID: s.ClientID, Type: helper.OrderEventTypeERROR}
					event.ErrorReason = "failed to login req ws"
					f.Cb.OnOrder(0, event)
				})
		}
	} else {
		// 没有设置，使用常规rs下单兜底
		go f.subRsInner.DoPlaceOrderRsNor(info, s)
	}
}

func (f *FatherRs) AmendOrderSelect(s helper.Signal) {
	info, ok := f.GetPairInfoByPair(&s.Pair)
	if !ok {
		f.Logger.Warnf("not found pairinfo for pair %s", s.Pair)
		return
	}

	// 路由逻辑每个所类似，抽象成公共
	if s.SignalChannelType == helper.SignalChannelTypeRs {
		if f.SelectedLine_Amend.Link == LinkType_Colo {
			go f.subRsInner.DoAmendOrderRsColo(info, s)
		} else {
			go f.subRsInner.DoAmendOrderRsNor(info, s)
		}
		return
	}
	if f.SelectedLine_Amend.Client == ClientType_Rs {
		if f.SelectedLine_Amend.Link == LinkType_Colo {
			// if !f.CanColoRs {
			if !f.Features.DoAmendOrderRsColo {
				f.Logger.Warnf("cannot rs colo, use normal link")
				// go rs.placeOrder(info, s.Price, s.Amount, s.ClientID, s.OrderSide, s.OrderType, s.Time, true, false)
				go f.subRsInner.DoAmendOrderRsNor(info, s)
			} else {
				// go rs.placeOrder(info, s.Price, s.Amount, s.ClientID, s.OrderSide, s.OrderType, s.Time, true, true)
				go f.subRsInner.DoAmendOrderRsColo(info, s)
			}
		} else {
			// go rs.placeOrder(info, s.Price, s.Amount, s.ClientID, s.OrderSide, s.OrderType, s.Time, true, false)
			go f.subRsInner.DoAmendOrderRsNor(info, s)
		}
	} else if f.SelectedLine_Amend.Client == ClientType_Ws {
		if f.SelectedLine_Amend.Link == LinkType_Colo {
			// if !f.CanColoWs {
			if !f.Features.DoAmendOrderWsColo {
				f.Logger.Warnf("cannot ws colo, use normal link")
				f.EnsureReqWsNorLogged(func() {
					f.subRsInner.DoAmendOrderWsNor(info, s)
				}, func() {
					event := helper.OrderEvent{ClientID: s.ClientID, Type: helper.OrderEventTypeAmendFail}
					event.ErrorReason = "failed to login req ws"
					f.Cb.OnOrder(0, event)
				})
			} else {
				f.EnsureReqWsColoLogged(func() {
					f.subRsInner.DoAmendOrderWsColo(info, s)
				}, func() {
					event := helper.OrderEvent{ClientID: s.ClientID, Type: helper.OrderEventTypeAmendFail}
					event.ErrorReason = "failed to login req ws"
					f.Cb.OnOrder(0, event)
				})
			}
		} else {
			f.EnsureReqWsNorLogged(func() {
				f.subRsInner.DoAmendOrderWsNor(info, s)
			}, func() {

				event := helper.OrderEvent{ClientID: s.ClientID, Type: helper.OrderEventTypeAmendFail}
				event.ErrorReason = "failed to login req ws"
				f.Cb.OnOrder(0, event)

			})
		}
	} else {
		// 没有设置，使用常规rs下单
		go f.subRsInner.DoAmendOrderRsNor(info, s)
	}
}

// fixme 全部支持批量下单后，删除
func (f *FatherRs) CancelOrderSelect(s helper.Signal) {
	info, ok := f.GetPairInfoByPair(&s.Pair)
	if !ok {
		f.Logger.Warnf("not found pairinfo for pair %s", s.Pair)
		return
	}

	if s.SignalChannelType == helper.SignalChannelTypeRs {
		if f.SelectedLine_Cancel.Link == LinkType_Colo {
			go f.subRsInner.DoCancelOrderRsColo(info, s)
		} else {
			go f.subRsInner.DoCancelOrderRsNor(info, s)
		}
		return
	}
	if f.SelectedLine_Cancel.Client == ClientType_Rs {
		if f.SelectedLine_Cancel.Link == LinkType_Colo {
			// if !f.CanColoRs {
			if !f.Features.DoCancelOrderRsColo {
				f.Logger.Warnf("cannot rs colo, use normal link")
				go f.subRsInner.DoCancelOrderRsNor(info, s)
			} else {
				go f.subRsInner.DoCancelOrderRsColo(info, s)
			}
		} else {
			go f.subRsInner.DoCancelOrderRsNor(info, s)
		}
	} else if f.SelectedLine_Cancel.Client == ClientType_Ws {
		if f.SelectedLine_Cancel.Link == LinkType_Colo {
			// if !f.CanColoWs {
			if !f.Features.DoCancelOrderWsColo {
				f.Logger.Warnf("cannot ws colo, use normal link")
				f.EnsureReqWsNorLogged(func() {
					f.subRsInner.DoCancelOrderWsNor(info, s)
				}, func() {
					f.Logger.Errorf("[%s] failed to login req ws. ", f.ExchangeName)
				})
			} else {
				f.EnsureReqWsColoLogged(func() {
					f.subRsInner.DoCancelOrderWsColo(info, s)
				}, func() {
					f.Logger.Errorf("[%s] failed to login req ws. ", f.ExchangeName)
				})
			}
		} else {
			f.EnsureReqWsNorLogged(func() {
				f.subRsInner.DoCancelOrderWsNor(info, s)
			},
				func() {
					f.Logger.Errorf("[%s] failed to login req ws. ", f.ExchangeName)
				})
		}
	} else {
		// 没有设置，使用常规rs下单兜底
		go f.subRsInner.DoCancelOrderRsNor(info, s)
	}
}

// 批量版本
func (f *FatherRs) PlaceBatchOrderSelect(sigs []helper.Signal) {
	info, ok := f.GetPairInfoByPair(&sigs[0].Pair)
	if !ok {
		f.Logger.Warnf("not found pairinfo for pair %s", sigs[0].Pair)
		return
	}

	if sigs[0].SignalChannelType == helper.SignalChannelTypeRs {
		if f.SelectedLine_Place.Link == LinkType_Colo {
			go f.subRsInner.DoPlaceBatchOrderRsColo(info, sigs)
		} else {
			go f.subRsInner.DoPlaceBatchOrderRsNor(info, sigs)
		}
		return
	}
	if f.SelectedLine_Place.Client == ClientType_Rs {
		if f.SelectedLine_Place.Link == LinkType_Colo {
			// if !f.CanColoRs {
			if !f.Features.DoPlaceOrderRsColo {
				f.Logger.Warnf("cannot rs colo, use normal link")
				go f.subRsInner.DoPlaceBatchOrderRsNor(info, sigs)
			} else {
				go f.subRsInner.DoPlaceBatchOrderRsColo(info, sigs)
			}
		} else {
			go f.subRsInner.DoPlaceBatchOrderRsNor(info, sigs)
		}
	} else if f.SelectedLine_Place.Client == ClientType_Ws {
		if f.SelectedLine_Place.Link == LinkType_Colo {
			// if !f.CanColoWs {
			if !f.Features.DoPlaceOrderWsColo {
				f.Logger.Warnf("cannot ws colo, use normal link")
				f.EnsureReqWsNorLogged(func() {
					f.subRsInner.DoPlaceBatchOrderWsNor(info, sigs)
				}, func() {
					event := helper.OrderEvent{ClientID: sigs[0].ClientID, Type: helper.OrderEventTypeERROR}
					event.ErrorReason = "failed to login req ws"
					f.Cb.OnOrder(0, event)
				})
			} else {
				f.EnsureReqWsColoLogged(func() {
					f.subRsInner.DoPlaceBatchOrderWsColo(info, sigs)
				}, func() {
					event := helper.OrderEvent{ClientID: sigs[0].ClientID, Type: helper.OrderEventTypeERROR}
					event.ErrorReason = "failed to login req ws"
					f.Cb.OnOrder(0, event)
				})
			}
		} else {
			f.EnsureReqWsNorLogged(func() {
				f.subRsInner.DoPlaceBatchOrderWsNor(info, sigs)
			},
				func() {
					event := helper.OrderEvent{ClientID: sigs[0].ClientID, Type: helper.OrderEventTypeERROR}
					event.ErrorReason = "failed to login req ws"
					f.Cb.OnOrder(0, event)
				})
		}
	} else {
		// 没有设置，使用常规rs下单兜底
		go f.subRsInner.DoPlaceBatchOrderRsNor(info, sigs)
	}
}

func (f *FatherRs) AmendBatchOrderSelect(sigs []helper.Signal) {
	info, ok := f.GetPairInfoByPair(&sigs[0].Pair)
	if !ok {
		f.Logger.Warnf("not found pairinfo for pair %s", sigs[0].Pair)
		return
	}

	// 路由逻辑每个所类似，抽象成公共
	if sigs[0].SignalChannelType == helper.SignalChannelTypeRs {
		if f.SelectedLine_Amend.Link == LinkType_Colo {
			go f.subRsInner.DoAmendBatchOrderRsColo(info, sigs)
		} else {
			go f.subRsInner.DoAmendBatchOrderRsNor(info, sigs)
		}
		return
	}
	if f.SelectedLine_Amend.Client == ClientType_Rs {
		if f.SelectedLine_Amend.Link == LinkType_Colo {
			// if !f.CanColoRs {
			if !f.Features.DoAmendOrderRsColo {
				f.Logger.Warnf("cannot rs colo, use normal link")
				// go rs.placeOrder(info, s.Price, s.Amount, s.ClientID, s.OrderSide, s.OrderType, s.Time, true, false)
				go f.subRsInner.DoAmendBatchOrderRsNor(info, sigs)
			} else {
				// go rs.placeOrder(info, s.Price, s.Amount, s.ClientID, s.OrderSide, s.OrderType, s.Time, true, true)
				go f.subRsInner.DoAmendBatchOrderRsColo(info, sigs)
			}
		} else {
			// go rs.placeOrder(info, s.Price, s.Amount, s.ClientID, s.OrderSide, s.OrderType, s.Time, true, false)
			go f.subRsInner.DoAmendBatchOrderRsNor(info, sigs)
		}
	} else if f.SelectedLine_Amend.Client == ClientType_Ws {
		if f.SelectedLine_Amend.Link == LinkType_Colo {
			// if !f.CanColoWs {
			if !f.Features.DoAmendOrderWsColo {
				f.Logger.Warnf("cannot ws colo, use normal link")
				f.EnsureReqWsNorLogged(func() {
					f.subRsInner.DoAmendBatchOrderWsNor(info, sigs)
				}, func() {
					event := helper.OrderEvent{ClientID: sigs[0].ClientID, Type: helper.OrderEventTypeAmendFail}
					event.ErrorReason = "failed to login req ws"
					f.Cb.OnOrder(0, event)
				})
			} else {
				f.EnsureReqWsColoLogged(func() {
					f.subRsInner.DoAmendBatchOrderWsColo(info, sigs)
				}, func() {
					event := helper.OrderEvent{ClientID: sigs[0].ClientID, Type: helper.OrderEventTypeAmendFail}
					event.ErrorReason = "failed to login req ws"
					f.Cb.OnOrder(0, event)
				})
			}
		} else {
			f.EnsureReqWsNorLogged(func() {
				f.subRsInner.DoAmendBatchOrderWsNor(info, sigs)
			}, func() {

				event := helper.OrderEvent{ClientID: sigs[0].ClientID, Type: helper.OrderEventTypeAmendFail}
				event.ErrorReason = "failed to login req ws"
				f.Cb.OnOrder(0, event)

			})
		}
	} else {
		// 没有设置，使用常规rs下单
		go f.subRsInner.DoAmendBatchOrderRsNor(info, sigs)
	}
}

// fixme 全部支持批量下单后，删除
func (f *FatherRs) CancelBatchOrderSelect(sigs []helper.Signal) {
	info, ok := f.GetPairInfoByPair(&sigs[0].Pair)
	if !ok {
		f.Logger.Warnf("not found pairinfo for pair %s", sigs[0].Pair)
		return
	}

	if sigs[0].SignalChannelType == helper.SignalChannelTypeRs {
		if f.SelectedLine_Cancel.Link == LinkType_Colo {
			go f.subRsInner.DoCancelBatchOrderRsColo(info, sigs)
		} else {
			go f.subRsInner.DoCancelBatchOrderRsNor(info, sigs)
		}
		return
	}
	if f.SelectedLine_Cancel.Client == ClientType_Rs {
		if f.SelectedLine_Cancel.Link == LinkType_Colo {
			// if !f.CanColoRs {
			if !f.Features.DoCancelOrderRsColo {
				f.Logger.Warnf("cannot rs colo, use normal link")
				go f.subRsInner.DoCancelBatchOrderRsNor(info, sigs)
			} else {
				go f.subRsInner.DoCancelBatchOrderRsColo(info, sigs)
			}
		} else {
			go f.subRsInner.DoCancelBatchOrderRsNor(info, sigs)
		}
	} else if f.SelectedLine_Cancel.Client == ClientType_Ws {
		if f.SelectedLine_Cancel.Link == LinkType_Colo {
			// if !f.CanColoWs {
			if !f.Features.DoCancelOrderWsColo {
				f.Logger.Warnf("cannot ws colo, use normal link")
				f.EnsureReqWsNorLogged(func() {
					f.subRsInner.DoCancelBatchOrderWsNor(info, sigs)
				}, func() {
				})
			} else {
				f.EnsureReqWsColoLogged(func() {
					f.subRsInner.DoCancelBatchOrderWsColo(info, sigs)
				}, func() {
				})
			}
		} else {
			f.EnsureReqWsNorLogged(func() {
				f.subRsInner.DoCancelBatchOrderWsNor(info, sigs)
			},
				func() {
				})
		}
	} else {
		// 没有设置，使用常规rs下单兜底
		go f.subRsInner.DoCancelBatchOrderRsNor(info, sigs)
	}
}
