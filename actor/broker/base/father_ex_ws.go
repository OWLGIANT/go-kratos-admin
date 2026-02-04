package base

import (
	"fmt"
	"time"

	"actor/broker/client/ws"
	"actor/helper"
)

type FatherWs struct {
	FatherCommon
	subWsInner             WsInner
	UsingDepthType         DepthType     // 深度使用方式
	TotalPubWsMsgCnt       int64         // pub ws收到数据的总次数
	OptimistBBOCnt         int64         // pubHandler 扣字节成功解析的次数
	StopCPub               chan struct{} // 接受停机信号
	StopC                  chan struct{} // 接受停机信号
	latestTickerUpdateTsns int64         // 最新ticker更新时间戳
	wsConnections          []*ws.WS      // 用于重连等管理
	ColoTickFasterTime     int64         // colo比pub tick快的次数
}

func InitFatherWs(msg *helper.TradeMsg, i WsInner, ws *FatherWs, params *helper.BrokerConfigExt, info *helper.ExchangeInfo, cb helper.CallbackFunc) {
	ws.subWsInner = i
	ws.InitCommon(params, info, cb, msg)

	pairInEx := fmt.Sprintf("%s@%s", params.BrokerConfig.Pairs[0].ToString(), params.BrokerConfig.Name)
	// 行情长时间没更新检测
	if params.NeedTicker {
		go func() {
			var _GAP_SEC int64 = 90
			wait := 0
			ticker := time.NewTicker(time.Second * time.Duration(_GAP_SEC))
			for {
				select {
				case <-ticker.C:
					if ws.latestTickerUpdateTsns == 0 && wait < 5 {
						ws.Logger.Warnf("[%s] still not call OnTicker one time", pairInEx)
						wait++
						continue
					}
					nowTsns := time.Now().UnixNano()
					if ws.latestTickerUpdateTsns+_GAP_SEC*1e9 < nowTsns {
						ws.Logger.Warnf("[%s] ticker too long not updated, reconnect now", pairInEx)
						for _, wsC := range ws.wsConnections {
							// ws.subWsInner.Resub()
							if wsC == nil {
								ws.Logger.Infof("[%s] ws is nil, maybe not inited or in stopping", pairInEx)
								continue
							}
							if err := wsC.Reconnect(); err != nil {
								ws.Logger.Errorf("[%s] failed to reconnect ws, %v", pairInEx, err)
							} else {
								ws.Logger.Infof("[%s] reconnect %p 成功发起", pairInEx, wsC)
							}
						}
					}
				case <-ws.StopCPub:
					ws.Logger.Infof("[%s] exit resub goroutine", pairInEx)
					return
				case <-ws.StopC:
					ws.Logger.Infof("[%s] exit resub goroutine", pairInEx)
					return
				}
			}
		}()
	}
}

func (w *FatherWs) OptimistParseBBO(msg []byte, ts int64) bool {
	if helper.DEBUGMODE {
		panic("OptimistParseBBO not implemented, should not be here")
	}
	return false
}

// 会过滤 nil， 放心添加参数. 不可多次调用
func (w *FatherWs) AddWsConnections(c ...*ws.WS) {
	w.wsConnections = make([]*ws.WS, 0)
	for _, c0 := range c {
		if c0 == nil {
			continue
		}
		w.wsConnections = append(w.wsConnections, c0)
		c0.AddReconnectPreHandler(func() error {
			if w.Cb.OnWsReconnecting != nil {
				w.Cb.OnWsReconnecting(helper.StringToBrokerName(w.BrokerConfig.Name))
			}
			return nil
		})
		if w.Cb.OnWsReconnected != nil {
			c0.SetReconnectPostHandler(func() error {
				w.Cb.OnWsReconnected(helper.StringToBrokerName(w.BrokerConfig.Name))
				return nil
			})
		}
	}
}
func (w *FatherWs) Stop() {
	w.subWsInner.DoStop()
	if !IsUtf {
		w.saveWsPerformanceMetrics()
	}
}
func (w *FatherWs) UpdateLatestTickerTs(tsns int64) {
	w.latestTickerUpdateTsns = tsns
}

func (w *FatherWs) saveWsPerformanceMetrics() {
	tags := map[string]string{
		"robot_id": w.BrokerConfig.RobotId,
		"ex":       w.BrokerConfig.Name,
	}
	percent := 0.0
	if w.TotalPubWsMsgCnt != 0 {
		percent = float64(w.OptimistBBOCnt) / float64(w.TotalPubWsMsgCnt)
	}
	percent2 := 0.0
	if w.TotalPubWsMsgCnt != 0 {
		percent2 = float64(w.ColoTickFasterTime) / float64(w.TotalPubWsMsgCnt)
	}

	fields := map[string]interface{}{
		"total_pub_ws_msg_cnt": w.TotalPubWsMsgCnt,
		"optimist_bbo_cnt":     w.OptimistBBOCnt,
		"optimist_percent":     percent,
		"colo_faster_cnt":      w.ColoTickFasterTime,
		"colo_faster_percent":  percent2,
	}
	w.Logger.Infof("ws msgCnt: %d, optimistCnt:%d, percent:%f", w.TotalPubWsMsgCnt, w.OptimistBBOCnt, percent)
	w.Logger.Infof("colo faster cnt: %d, percent: %f", w.ColoTickFasterTime, percent2)
	w.saveOnePoint("performance", "ws", tags, fields)
}
