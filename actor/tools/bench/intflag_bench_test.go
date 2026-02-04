package bench

import (
	"testing"

	"go.uber.org/atomic"
)

/*
go test -bench=. -benchmem
goos: linux
goarch: amd64
pkg: actor/tools/bench
cpu: AMD Ryzen 7 7840HS with Radeon 780M Graphics
BenchmarkIntFlagUint64Inc-16            1000000000               0.2609 ns/op          0 B/op          0 allocs/op
BenchmarkIntFlagAtomicStore-16          632933888                1.773 ns/op           0 B/op          0 allocs/op
*/

func BenchmarkIntFlagUint64Inc(b *testing.B) {
	a := uint64(0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a++
	}
	_ = a
}

func BenchmarkIntFlagAtomicStore(b *testing.B) {
	var a atomic.Int64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Store(int64(i))
	}
	_ = a
}
