package base

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMonitorGenCid(t *testing.T) {

	var monitor DelayMonitor
	frsShouldNotBeNil := &FatherRs{}
	InitDelayMonitor(&monitor, frsShouldNotBeNil, InfluxDBConfig{}, "gate_usdt_swap", "btc_usdt", func(key EndpointKey_T, activeFirst bool) {})
	cid := monitor.GenCid(EndpointKey_RsNorPlace, true)
	fmt.Println("cid ", cid)
	require.True(t, strings.HasPrefix(cid, "t-M_a_01"))
	{
		active, key, ok := monitor.GetActiveAndKeyFromCid(cid)
		require.True(t, ok)
		require.True(t, active)
		require.Equal(t, key, EndpointKey_RsNorPlace)
	}

	cid = monitor.GenCid(EndpointKey_RsColoPlace, true)
	fmt.Println("cid ", cid)
	require.True(t, strings.HasPrefix(cid, "t-M_a_02"))
	{
		active, key, ok := monitor.GetActiveAndKeyFromCid(cid)
		require.True(t, ok)
		require.True(t, active)
		require.Equal(t, key, EndpointKey_RsColoPlace)
	}

	cid = monitor.GenCid(EndpointKey_RsColoPlace, false)
	fmt.Println("cid ", cid)
	require.True(t, strings.HasPrefix(cid, "t-M_n_02"))
	{
		active, key, ok := monitor.GetActiveAndKeyFromCid(cid)
		require.True(t, ok)
		require.False(t, active)
		require.Equal(t, key, EndpointKey_RsColoPlace)
	}

	{
		_, _, ok := monitor.GetActiveAndKeyFromCid("12348")
		require.False(t, ok)
	}

	// bybit
	monitor = DelayMonitor{}
	InitDelayMonitor(&monitor, frsShouldNotBeNil, InfluxDBConfig{}, "bybit_usdt_swap", "btc_usdt", func(key EndpointKey_T, activeFirst bool) {})
	cid = monitor.GenCid(EndpointKey_RsNorPlace, true)
	fmt.Println("cid ", cid)
	require.True(t, strings.HasPrefix(cid, "M_a_01"))
	{
		active, key, ok := monitor.GetActiveAndKeyFromCid(cid)
		require.True(t, ok)
		require.True(t, active)
		require.Equal(t, key, EndpointKey_RsNorPlace)
	}

	cid = monitor.GenCid(EndpointKey_RsColoPlace, true)
	fmt.Println("cid ", cid)
	require.True(t, strings.HasPrefix(cid, "M_a_02"))
	{
		active, key, ok := monitor.GetActiveAndKeyFromCid(cid)
		require.True(t, ok)
		require.True(t, active)
		require.Equal(t, key, EndpointKey_RsColoPlace)
	}

	cid = monitor.GenCid(EndpointKey_RsColoPlace, false)
	fmt.Println("cid ", cid)
	require.True(t, strings.HasPrefix(cid, "M_n_02"))
	{
		active, key, ok := monitor.GetActiveAndKeyFromCid(cid)
		require.True(t, ok)
		require.False(t, active)
		require.Equal(t, key, EndpointKey_RsColoPlace)
	}

	// others
	{
		cid = "23087628231765"
		_, _, ok := monitor.GetActiveAndKeyFromCid(cid)
		require.False(t, ok)
	}
	{
		cid = "M_n_02_1723087628231765"
		_, _, ok := monitor.GetActiveAndKeyFromCid(cid)
		require.True(t, ok)
		require.True(t, monitor.IsMonitorOrder(cid))
		require.Equal(t, int64(1723087628231765), monitor.parseStr2Delay(cid))
	}

}
