package helper

import (
	"encoding/json"
	"fmt"
	"testing"

	"actor/third/fixed"
	"github.com/stretchr/testify/require"
)

func TestRingMa(t *testing.T) {
	r := NewRingMa(10)
	r2 := RollingMa{N: 10}
	for i := 0; i < 100; i++ {
		fmt.Println(r.Update(float64(i)))
		fmt.Println(r2.Update(float64(i)))
	}
}

func TestRingMa_SetSize(t *testing.T) {
	r := NewRingMa(10)
	for i := 0; i < 4; i++ {
		r.Update(float64(i))
	}
	fmt.Println(r)
	r.Update(10)
	fmt.Println(r)
	r.SetSize(8)
	fmt.Println(r)
	r.Update(20)
	fmt.Println(r)
}

// BenchmarkRollingMa-8   	1000000000	         0.002948 ns/op
func BenchmarkRollingMa(b *testing.B) {
	r := RollingMa{N: 100}
	for i := 0; i < 100000; i++ {
		r.Update(float64(i))
	}
}

// BenchmarkRingMa-8   	1000000000	         0.0009460 ns/op
func BenchmarkRingMa(b *testing.B) {
	r := NewRingMa(100)
	for i := 0; i < 100000; i++ {
		r.Update(float64(i))
	}
}

func TestSeqMarshal(t *testing.T) {
	ticker := Equity{}
	ticker.Seq.InnerServerId = 888888888
	ticker.Seq.NewerAndStore(2347328, 342)
	ticker.Coin = 234134
	ticker.CoinFree = 234134.23423
	jsondata, _ := json.Marshal(ticker)
	fmt.Printf("ticker v: %v\n", ticker)
	fmt.Printf("marshal json: %v\n", string(jsondata))
	ticker2 := Equity{}
	json.Unmarshal(jsondata, &ticker2)
	fmt.Printf("unmarshal ticker2 v: %v\n", ticker2)

	seq2 := ticker.Seq
	fmt.Printf("seq2 v: %v\n", seq2)
}

func TestPosMarshal(t *testing.T) {
	pos := Pos{}
	pos.Seq.InnerServerId = 888888888
	pos.Seq.NewerAndStore(2347328, 342)
	pos.LongPos = fixed.NewF(234.234)
	pos.ShortPos = fixed.NewS("123.123")
	pos.LongAvg = 234.1123
	pos.ShortAvg = 888.1234

	jsondata, _ := json.Marshal(pos)
	fmt.Printf("pos v: %v\n", pos)
	fmt.Printf("marshal json: %v\n", string(jsondata))
	pos2 := Pos{}
	json.Unmarshal(jsondata, &pos2)
	fmt.Printf("unmarshal pos v: %v\n", pos2)
}

func TestRoundToPrecision(t *testing.T) {
	require.Equal(t, 3.1416, RoundToPrecision(3.1415926, 4))
	require.Equal(t, 3.1415, RoundToPrecision(3.1415426, 4))
	require.Equal(t, 3.14152387, RoundToPrecision(3.1415238712374834, 8))
	require.Equal(t, 3.14152388, RoundToPrecision(3.1415238792374834, 8))
}

func TestBuildVersionInfo(t *testing.T) {
	fmt.Println(BuildVersionInfo(AppVersionInfo{
		ApplicationName: "beastStraSheep",
		BuildTime:       "2021-09-09T16:00:00+08:00",
		GitCommitHash:   "1234567890",
		GitTag:          "v1.0.0",
		GoVersion:       "go1.16.7",
		QuantCommitHash: "0987654321",
	}))
}

func TestBuildVersionInfo2(t *testing.T) {
	msg := BuildVersionInfo(AppVersionInfo{
		ApplicationName: "beastStraSheep",
		BuildTime:       "2021-09-09T16:00:00+08:00",
		GitCommitHash:   "1234567890",
		GitTag:          "v1.0.0",
		GoVersion:       "go1.16.7",
		QuantCommitHash: "0987654321",
	})
	fmt.Println(msg)
	fmt.Println(ParseVersionInfo(msg))
}

func TestRingMa_ResetToSize_HappyPath(t *testing.T) {
	r := NewRingMa(10)
	r.ResetToSize(5, 2.0)

	require.Equal(t, 5, r.GetSize())
	require.Equal(t, 5, len(r.GetQueue()))
	for _, v := range r.GetQueue() {
		require.Equal(t, 2.0, v)
	}
	require.Equal(t, 5.0, r.fsize)
	require.Equal(t, false, r.first)
	require.Equal(t, 4, r.index)
	require.Equal(t, 10.0, r.sum)
	require.Equal(t, 2.0, r.Avg)
}

func TestRingMa_ResetToSize_SizeZero(t *testing.T) {
	r := NewRingMa(10)
	r.ResetToSize(0, 2.0)

	require.Equal(t, 10, r.GetSize())
	require.Equal(t, 10, len(r.GetQueue()))
}

func TestRingMa_ResetToSize_SizeNegative(t *testing.T) {
	r := NewRingMa(10)
	r.ResetToSize(-5, 2.0)

	require.Equal(t, 10, r.GetSize())
	require.Equal(t, 10, len(r.GetQueue()))
}

func TestTickerSet(t *testing.T) {
	ticker := Ticker{}
	ticker.Set(1.1, 1.2, 1.3, 1.4)
	ticker.Ap.Store(101.0)
	ticker.Aq.Store(102.0)
	ticker.Bp.Store(103.0)
	ticker.Bq.Store(104.0)
	ticker.Seq.Ex.Store(1234)
	ticker.Seq.InnerServerId = 1235
	ticker.Delay.Store(2)
	ticker.DelayE.Store(3)

	tickerStr, err := ticker.MarshalJSONCustom()
	require.NoError(t, err)
	fmt.Println(string(tickerStr))
	expectStr := `{"Seq":{"e":1234,"i":0,"ii":1235},"Ap":101.000000,"Aq":102.000000,"Bp":103.000000,"Bq":104.000000,"Delay":2,"DelayE":3}`
	require.Equal(t, expectStr, string(tickerStr))

	////////////////////
	str := `{"Seq":{"e":1234,"i":0,"ii":1235},"Bp":1.100000,"Bq":1.200000,"Ap":1.300000,"Aq":1.400000,"Delay":2,"DelayE":3}`
	ticker2 := Ticker{}
	err = ticker2.UnmarshalJSONCustom([]byte(str))
	require.NoError(t, err)
	require.Equal(t, int64(1234), ticker2.Seq.Ex.Load())
	require.Equal(t, int64(1235), ticker2.Seq.InnerServerId)
	require.Equal(t, 1.1, ticker2.Bp.Load())
	require.Equal(t, 1.2, ticker2.Bq.Load())
	require.Equal(t, 1.3, ticker2.Ap.Load())
	require.Equal(t, 1.4, ticker2.Aq.Load())
	require.Equal(t, int64(2), ticker2.Delay.Load())
	require.Equal(t, int64(3), ticker2.DelayE.Load())
	// fmt.Println(ticker2.Seq.String())
	// fmt.Println(ticker2)

}
