package helper

import (
	"fmt"
	"testing"

	"actor/third/fixed"
	"github.com/stretchr/testify/require"
)

// BenchmarkFixPrice1-8   	41825187	        28.33 ns/op
func BenchmarkFixPrice1(b *testing.B) {
	f := fixed.NewS("1.12345678")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		FixAmount(f, 0.001)
	}
}

// 80ns
func BenchmarkTssecToReadableStr(b *testing.B) {
	tssec := int64(1702483200)
	fmt.Println(TssecToReadableStr(tssec))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		TssecToReadableStr(tssec)
	}
}

// 40ns
func BenchmarkTssecToReadableInt(b *testing.B) {
	tssec := int64(1702483200)
	fmt.Println(TsToHumanSec(tssec))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		TsToHumanSec(tssec)
	}
}

func TestTsToHumanMillis(t *testing.T) {
	tsms := int64(1728487165997)
	require.Equal(t, int64(20241009231925997), TsToHumanMillis(tsms))
	require.Equal(t, int64(20241009231925), TsToHumanSec(1728487165))
}

func TestBytesToFloat64(t *testing.T) {
	s := []byte("8.32E-6")
	f := BytesToFloat64(s)
	fmt.Printf("%f\n", f)
	if f != 8.32e-6 {
		t.Fatal("error")
	}
}
func TestKeepOnlyNumbers(t *testing.T) {
	s := "2024-04-07T16:23:29+0800"
	fmt.Println(KeepOnlyNumbers(s))
}
func TestFixAmount(t *testing.T) {
	// 用例设计
	// 1. stepsize, 大整数、小整数、大小数、小小数、小小小数
	// 2. amout 大数，小数，>5分数，<5分数, （负数？）

	// 结构 stepSize:[[execpted, test_amouts...]]
	m := map[float64][][]float64{
		1000: {
			{88000, 88000, 88900, 88100, 88900.1001, 88723.201, 88100.1001, 88999.999999}, //
			{0, 0, 100.234, 23.1234, 45.214324, 7.2134123, 0.14343},
		},
		5: {
			{8000, 8000, 8003, 8002, 8002.101, 8003.9999999}, //
			{15, 15, 15.2, 15.000003, 17.2391, 18.29291, 19.999999},
			{0, 0, 0.234, 3.1234, 4.214324, 3.2134123, 0.14343},
		},
		1: {
			{8000, 8000, 8000.234, 8000.2, 8000.2101, 8000.39999999}, //
			{15, 15, 15.2, 15.000003, 15.2391, 15.29291, 15.999999},
			{0, 0, 0.234, 0.1234, 0.214324, 0.2134123, 0.14343},
		},
		0.5: {
			{40241.5, 40241.5, 40241.63, 40241.6300005, 40241.69, 40241.6900001, 40241.610005},
			{40241.5, 40241.5, 40241.53, 40241.5300005, 40241.59, 40241.5900001, 40241.510005},
			{241.5, 241.5, 241.53, 241.5300005, 241.59, 241.5900001, 241.510005},
			{0, 0, 0.234, 0.1234, 0.214324, 0.2134123, 0.14343},
			{0.5, 0.5, 0.734, 0.6234, 0.714324, 0.7134123, 0.54343},
		},
		0.1: {
			{40241.6, 40241.6, 40241.63, 40241.6300005, 40241.69, 40241.6900001, 40241.610005},
			{40241.5, 40241.5, 40241.53, 40241.5300005, 40241.59, 40241.5900001, 40241.510005},
			{0.2, 0.2, 0.234, 0.2234, 0.214324, 0.2134123, 0.24343},
			{0, 0, 0.0234, 0.01234, 0.0214324, 0.02134123, 0.014343},
		},
		0.0001: {
			{40241.63, 40241.63, 40241.63, 40241.6300005, 40241.630002, 40241.6300001, 40241.630005},
			{40241.5, 40241.5, 40241.5000009, 40241.500005, 40241.5000099, 40241.500001},
			{6.3, 6.3, 6.30001, 6.30009, 6.3000999, 6.3000501},
			{0.0002, 0.0002, 0.000234, 0.00029999, 0.00020001, 0.00021111},
			{0, 0, 0.000001234, 0.00001234, 0.0000214324, 0.00002134123, 0.000014343},
		},
		0.0000001: {
			{0.0019349, 0.0019349, 0.001934908, 0.001934998},
		},
	}
	for stepSize, valuesList := range m {
		for _, values := range valuesList {
			expected := values[0]
			for _, v := range values[1:] {
				a := FixAmount(fixed.NewF(v), stepSize)
				require.Equal(t, expected, a.Float(), fmt.Sprintf("%v not fixamount %v", a, expected))
				// fmt.Println(stepSize, expected, a)
			}
		}
	}
	fmt.Println("FixAmount正向相等")

	for stepSize, valuesList := range m {
		for _, values := range valuesList {
			unexpected := values[0] + 0.1
			for _, v := range values[1:] {
				a := FixAmount(fixed.NewF(v), stepSize)
				require.NotEqual(t, unexpected, a.Float(), fmt.Sprintf("%v not fixamount %v", a, unexpected))
				// fmt.Println(stepSize, unexpected, a)
			}
		}
	}
	fmt.Println("FixAmount负向符合")

	// a := fixed.NewF(40241.63)
	b := FixAmount(fixed.NewF(0.00315), 0.00001)
	// // a := fixed.NewF(4024.1)
	// // b := FixAmount(a, 0.1) // 正确 4024.1
	// fmt.Printf("%v\n", a)
	fmt.Printf("b: %s\n", b)
}

func TestFixPrice(t *testing.T) {
	const PRICE_SCALE = 0.0001
	p := FixPrice(23431.635000000002, 0.1).Div(fixed.NewF(PRICE_SCALE))
	fmt.Printf("%v\n", p)
	fmt.Printf("%s\n", p.String())
}

func TestTrim10Multiple(t *testing.T) {
	baseCoin := "1000PEPE"
	require.Equal(t, "PEPE", Trim10Multiple(baseCoin))
	baseCoin = "PEPE10000"
	require.Equal(t, "PEPE", Trim10Multiple(baseCoin))
}

func TestFixAmount2(t *testing.T) {
	a := FixAmount(fixed.NewF(0.0078), 0.001)
	fmt.Println(a.Float())
}

func TestCeil(t *testing.T) {
	tests := [][3]int64{
		{2, 1, 2},
		{2, 2, 2},
		{2, 3, 3},
		{10, 4, 12},
		{12, 4, 12},
		{1702465447786, 8 * 3600 * 1000, 1702483200000},
		{1702483200000, 8 * 3600 * 1000, 1702483200000},
		{1702483200001, 8 * 3600 * 1000, 1702483200000 + 8*3600*1000},
	}
	for _, test := range tests {
		require.Equal(t, test[2], Ceil(test[0], test[1]))
	}
	badtests := [][3]int64{
		{2, 1, 3},
		{2, 2, 3},
		{2, 3, 4},
		{10, 4, 13},
		{12, 4, 13},
		{1702465447786, 8 * 3600 * 1000, 1702483200001},
		{1702483200000, 8 * 3600 * 1000, 1702483200001},
		{1702483200001, 8 * 3600 * 1000, 1702483200001 + 8*3600*1000},
	}
	for _, test := range badtests {
		require.NotEqual(t, test[2], Ceil(test[0], test[1]))
	}
}

func TestLcm(t *testing.T) {
	fmt.Println(Lcm(fixed.NewF(0.002), fixed.NewF(0.005)))
}

func TestUniqueIdentifyShortest(t *testing.T) {
	require.Equal(t, 0, UniqueIdentifyShortest(nil))
	require.Equal(t, 0, UniqueIdentifyShortest([]string{"ab"}))
	require.Equal(t, 1, UniqueIdentifyShortest([]string{"ab", "b"}))
	require.Equal(t, -1, UniqueIdentifyShortest([]string{"ab", "a"}))
	require.Equal(t, -1, UniqueIdentifyShortest([]string{"a", "a"}))
	require.Equal(t, 2, UniqueIdentifyShortest([]string{"ab", "ac"}))
	require.Equal(t, -1, UniqueIdentifyShortest([]string{"ab", "ac", "c"})) // 有字符串太短
	require.Equal(t, -1, UniqueIdentifyShortest([]string{"ab", "abc"}))     // 不能是别人的前缀
}
