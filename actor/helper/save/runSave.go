// save 保存行情到文件 供回测系统使用
package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"actor"
	"actor/helper"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("程序运行失败: %v", err)
	}
}

func run() error {
	// 创建上下文用于优雅退出
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n接收到退出信号，正在关闭...")
		cancel()
	}()

	// 初始化消息容器
	tradeMsg := helper.TradeMsg{}
	refMsg := helper.TradeMsg{}

	// 创建主交易所WebSocket客户端
	_, ws := actor.GetClient(
		actor.ClientTypeWs,
		"binance_usdt_swap",
		helper.BrokerConfig{},
		&tradeMsg,
		helper.CallbackFunc{OnTicker: func(ts int64) {}},
	)
	ws.Run()

	// 创建参考交易所WebSocket客户端
	_, refws := actor.GetClient(
		actor.ClientTypeWs,
		"coinex_usdt_swap",
		helper.BrokerConfig{},
		&refMsg,
		helper.CallbackFunc{OnTicker: func(ts int64) {}},
	)
	refws.Run()

	// 创建CSV文件和写入器
	file, err := os.Create("history.csv")
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入CSV头部
	headers := []string{
		"ts", "refbp", "refbq", "refap", "refaq",
		"bp", "ap", "maxFill", "minFill",
		"buyNum", "sellNum", "buyQ", "sellQ", "buyV", "sellV",
	}
	if err := writer.Write(headers); err != nil {
		return fmt.Errorf("写入CSV头部失败: %w", err)
	}

	// 等待行情数据就绪
	fmt.Println("等待行情数据就绪...")
	if err := waitForMarketData(ctx, &tradeMsg, &refMsg); err != nil {
		return err
	}
	fmt.Println("开始记录行情")

	// 主循环：记录行情数据
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("程序正常退出")
			return nil
		case <-ticker.C:
			if err := writeMarketData(writer, &tradeMsg, &refMsg); err != nil {
				log.Printf("写入数据失败: %v", err)
			}
		}
	}
}

// waitForMarketData 等待市场数据就绪
func waitForMarketData(ctx context.Context, tradeMsg, refMsg *helper.TradeMsg) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("等待行情数据时被中断")
		case <-ticker.C:
			if tradeMsg.Ticker.Mp.Load() > 0 && refMsg.Ticker.Mp.Load() > 0 {
				return nil
			}
		}
	}
}

// writeMarketData 写入市场数据到CSV
func writeMarketData(writer *csv.Writer, tradeMsg, refMsg *helper.TradeMsg) error {
	maxFill, minFill, buyNum, sellNum, buyQ, sellQ, buyV, sellV := tradeMsg.Trade.Get()

	record := []string{
		fmt.Sprint(time.Now().UnixMilli()),
		fmt.Sprint(refMsg.Ticker.Bp),
		fmt.Sprint(refMsg.Ticker.Bq),
		fmt.Sprint(refMsg.Ticker.Ap),
		fmt.Sprint(refMsg.Ticker.Aq),
		fmt.Sprint(tradeMsg.Ticker.Bp),
		fmt.Sprint(tradeMsg.Ticker.Ap),
		fmt.Sprint(maxFill),
		fmt.Sprint(minFill),
		fmt.Sprint(buyNum),
		fmt.Sprint(sellNum),
		fmt.Sprint(buyQ),
		fmt.Sprint(sellQ),
		fmt.Sprint(buyV),
		fmt.Sprint(sellV),
	}

	if err := writer.Write(record); err != nil {
		return err
	}
	writer.Flush()

	if err := writer.Error(); err != nil {
		return err
	}

	fmt.Println(time.Now().Format("2006-01-02 15:04:05.000"))
	return nil
}
