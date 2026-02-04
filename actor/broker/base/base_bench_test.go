package base

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

/*
lookup 性能对比
cpu: 12th Gen Intel(R) Core(TM) i7-12700F
BenchmarkExchangeInfoConcurrentMap-20           71383131                16.97 ns/op
BenchmarkExchangeInfoPureMapWithLock-20         72180745                19.39 ns/op
BenchmarkExchangeInfoPureMapWithoutLock-20      154140511               12.68 ns/op
*/
// func BenchmarkExchangeInfoConcurrentMap(b *testing.B) {
// 	pair := helper.NewPair("btc", "usdt", "")
// 	e1 := make(map[string]*helper.ExchangeInfo)
// 	e2 := make(map[string]*helper.ExchangeInfo)
// 	file := "./exchangeInfo@binance_spot..json"
// 	exec.Command("touch", file).Run()
// 	_, _, ok := helper.TryGetExchangeInfosPtrFromFile(file, pair, e1, e2)
// 	if !ok {
// 		panic("failed")
// 	}
// 	sm1 := cmap.New[*helper.ExchangeInfo]()
// 	sm2 := cmap.New[*helper.ExchangeInfo]()
// 	for k, v := range e1 {
// 		sm1.Set(k, v)
// 	}
// 	for k, v := range e2 {
// 		sm2.Set(k, v)
// 	}

// 	for i := 0; i < b.N; i++ {
// 		_, _ = sm1.Get("eth_usdt")
// 	}
// }
// func BenchmarkExchangeInfoPureMapWithLock(b *testing.B) {
// 	pair := helper.NewPair("btc", "usdt", "")
// 	e1 := make(map[string]*helper.ExchangeInfo)
// 	e2 := make(map[string]*helper.ExchangeInfo)
// 	file := "./exchangeInfo@binance_spot..json"
// 	exec.Command("touch", file).Run()
// 	_, _, ok := helper.TryGetExchangeInfosPtrFromFile(file, pair, e1, e2)
// 	if !ok {
// 		panic("failed")
// 	}

// 	lock := sync.RWMutex{}
// 	for i := 0; i < b.N; i++ {
// 		lock.RLock()
// 		_ = e1["eth_usdt"]
// 		lock.RUnlock()
// 	}
// }

// func BenchmarkExchangeInfoPureMapWithoutLock(b *testing.B) {
// 	pair := helper.NewPair("btc", "usdt", "")
// 	e1 := make(map[string]*helper.ExchangeInfo)
// 	e2 := make(map[string]*helper.ExchangeInfo)
// 	file := "./exchangeInfo@binance_spot..json"
// 	exec.Command("touch", file).Run()
// 	_, _, ok := helper.TryGetExchangeInfosPtrFromFile(file, pair, e1, e2)
// 	if !ok {
// 		panic("failed")
// 	}

// 	for i := 0; i < b.N; i++ {
// 		_ = e1["eth_usdt"]
// 	}
// }

/*
cpu: 12th Gen Intel(R) Core(TM) i7-12700F
BenchmarkSplitClientOrderId-20          26904277                42.45 ns/op
*/
func BenchmarkSplitClientOrderId(b *testing.B) {
	s := "btc_28391487289479127349"
	for i := 0; i < b.N; i++ {
		_ = strings.SplitN(s, "_", 1)
	}
}

/*
cpu: 12th Gen Intel(R) Core(TM) i7-12700F
BenchmarkSplitClientOrderIdFirst-20    635577413.9                1.690 ns/op
*/
func BenchmarkSplitClientOrderIdFirst(b *testing.B) {
	s := "btcXXXXXXXXXX_28391487289479127349"
	s0 := s[0:6]
	s0 = strings.TrimRight(s0, "X")
	fmt.Println("s0 ", s0)
	for i := 0; i < b.N; i++ {
		s0 := s[0:14]
		_ = strings.TrimRight(s0, "X")
		// _ = s1
		// _ = strings.Replace("Sbtc", "S", "$", 1) // 相当慢，32ns
	}
}

/*
567316850                2.052 ns/op
*/
func BenchmarkSplitClientOrderIdFind(b *testing.B) {
	s := "btcXXXX_28391487289479127349"
	idx := strings.Index(s, "_")
	s0 := s[0:idx]
	s0 = strings.TrimRight(s0, "X")
	fmt.Println("s0 ", s0)
	for i := 0; i < b.N; i++ {
		idx := strings.Index(s, "_")
		_ = s[0:idx]
		// _ = strings.TrimRight(s0, "X")
		// _ = s1
		// _ = strings.Replace("Sbtc", "S", "$", 1) // 相当慢，32ns
	}
}

func BenchmarkStringToNum(b *testing.B) {
	v := "t-M_n_07_1723437805291809"
	fmt.Println("len ", len(v))
	b.ResetTimer()
	a, _ := strconv.Atoi(v[9:]) // 7ns
	fmt.Println(time.Now().UnixMicro() - int64(a))
	for i := 0; i < b.N; i++ {
		// _, _ = strconv.ParseInt(v[10:], 10, 64) // 15.63ns
		// _, _ = strconv.ParseInt(v[10:], 10, 0) // 15.63ns
		_, _ = strconv.Atoi(v[9:]) // 7ns
	}

}

func BenchmarkStringToNum2(b *testing.B) {
	// 10-
	// 23437805291809
	// x4378eHex   10
	// t-M_n07 7
	// const num2char = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_-"
	// v2 := "t-M_n0723437805291809"
}

func BenchmarkNum2String(b *testing.B) {
	v := time.Now().UnixMicro()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// _ = fmt.Sprintf("%d", v) // 59ns
		_ = strconv.Itoa(int(v)) // 27ns
	}
}

func BenchmarkGetTs(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// _ = time.Now().UnixMicro() // 41.27ns
		// _ = time.Now().UnixMilli() // 36 ns
		// _ = time.Now().Format("2006-01-02 15:04:05") // 124 ns
	}
}

// 2.9 ns
func BenchmarkStringCompare(b *testing.B) {
	b.ResetTimer()
	v1 := "t-M_00_1234567890123456789"
	v2 := "t-M_00_1234567890123456780"
	for i := 0; i < b.N; i++ {
		_ = v1 == v2
	}
}

// 1.7 ns
func BenchmarkStringCompare2(b *testing.B) {
	b.ResetTimer()
	v1 := "t-M_00_1234567890123456789"
	v2 := "t-M_90_1234567890123456780"
	for i := 0; i < b.N; i++ {
		_ = v1 == v2
	}
}
