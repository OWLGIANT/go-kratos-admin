package starkex

import (
	"fmt"
	"testing"
	"time"
)

func TestSignOrder(t *testing.T) {
	const MOCK_PRIVATE_KEY = "58c7d5a90b1776bde86ebac077e053ed85b0f7164f53b080304a531947f46e3"
	param := OrderSignParam{
		// NetworkId:  NETWORK_ID_ROPSTEN,
		NetworkId: NETWORK_ID_MAINNET,
		Market:    "BTC-USDT",
		Side:      "BUY",
		// PositionId: 12345,
		PositionId: 519064523538169932,
		HumanSize:  "0.1",
		HumanPrice: "36890",
		LimitFee:   "0.0005",
		ClientId:   "9275491174640984",
		// ClientId:   "This is an ID that the client came up with to describe this order",
		// Expiration: "2020-09-17T04:15:55.028Z",
		// Expiration: "2023-12-04T00:30:12.028Z",
		// Expiration: "2023-12-04T08:30:12.028Z",
		// Expiration: "2023-12-03T16:30:12.028Z", // utc 时间
		Expiration: time.Now().UnixMilli() + 28*24*3600*1000,
	}
	fee, sign, err := OrderSign(MOCK_PRIVATE_KEY, param)
	// 00cecbe513ecdbf782cd02b2a5efb03e58d5f63d15f2b840e9bc0029af04e8dd0090b822b16f50b2120e4ea9852b340f7936ff6069d02acca02f2ed03029ace5
	fmt.Println("fee, sign,err", fee, sign, err)
}
