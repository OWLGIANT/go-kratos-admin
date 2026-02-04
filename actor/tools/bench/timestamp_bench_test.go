package bench

import (
	"fmt"
	"testing"
	"time"

	"github.com/templexxx/tsc"
)

/*
go test -bench=. -benchmem
goos: linux
goarch: amd64
pkg: actor/tools/bench
cpu: AMD Ryzen 7 7840HS with Radeon 780M Graphics
BenchmarkGetTimsMs-16           30687446                37.90 ns/op            0 B/op          0 allocs/op
BenchmarkGetTimsNs-16           30531888                37.67 ns/op            0 B/op          0 allocs/op
BenchmarkGetTsc-16              149740472                8.058 ns/op           0 B/op          0 allocs/op
BenchmarkGetTscNs-16            149594164                8.456 ns/op           0 B/op          0 allocs/op
*/

func BenchmarkGetTimsMs(b *testing.B) {
	t := time.Now().UnixMicro()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t = time.Now().UnixMicro()
	}
	_ = t
}

func BenchmarkGetTimsNs(b *testing.B) {
	t := time.Now().UnixNano()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t = time.Now().UnixNano()
	}
	_ = t
}

func GetTSC() uint64

// 读取tsc寄存器的值
func BenchmarkGetTsc(b *testing.B) {
	var tsc uint64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tsc = GetTSC()
	}
	_ = tsc
}

func BenchmarkGetTscNs(b *testing.B) {
	var v int64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v = tsc.UnixNano()
	}
	_ = v
}

// 误差低于 1 us
/*
1742689789836650496 1742689789836650188 308 0.000000000000000222
1742689789836666624 1742689789836666509 115 0.000000000000000000
1742689789836669184 1742689789836668824 360 0.000000000000000222
1742689789836670976 1742689789836670827 149 0.000000000000000222
1742689789836673024 1742689789836672821 203 0.000000000000000222
1742689789836 1742689789836 0 0.000000000000000000
1742689789836 1742689789836 0 0.000000000000000000
1742689789836 1742689789836 0 0.000000000000000000
*/

func TestTscNs(t *testing.T) {
	for i := 0; i < 10; i++ {
		v1 := tsc.UnixNano()
		v2 := time.Now().UnixNano()
		fmt.Printf("%d %d %d %.18f\n", v1, v2, v1-v2, float64(v1)/float64(v2)-1)
	}
	for i := 0; i < 10; i++ {
		v1 := tsc.UnixNano() / 1e6
		v2 := time.Now().UnixMilli()
		fmt.Printf("%d %d %d %.18f\n", v1, v2, v1-v2, float64(v1)/float64(v2)-1)
	}
}
