package helper

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBrokerName(t *testing.T) {
	require.Equal(t, StringToBrokerName("mexc_usdt_swap"), BrokernameMexcUsdtSwap)
	require.Equal(t, "gate_usdt_swap", BrokernameGateUsdtSwap.String())

}

func TestBrokerNameAbbr(t *testing.T) {
	require.Equal(t, "mx_um", BrokernameMexcUsdtSwap.StringAbbr())
	require.Equal(t, "mx_spot", BrokernameMexcSpot.StringAbbr())
	require.Equal(t, "gt_um", BrokernameGateUsdtSwap.StringAbbr())
	require.Equal(t, "bg_um.uta", BrokernameBitgetUsdtUm.StringAbbr())
	require.Equal(t, "bg_spot.uta", BrokernameBitgetSpotUm.StringAbbr())
	//
	require.Equal(t, StringAbbrToBrokerName("mx_um"), BrokernameMexcUsdtSwap)
	require.Equal(t, StringAbbrToBrokerName("mx_spot"), BrokernameMexcSpot)
	require.Equal(t, StringAbbrToBrokerName("gt_um"), BrokernameGateUsdtSwap)
	require.Equal(t, StringAbbrToBrokerName("bg_um.uta"), BrokernameBitgetUsdtUm)
	require.Equal(t, StringAbbrToBrokerName("bg_spot.uta"), BrokernameBitgetSpotUm)
}
