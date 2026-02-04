package helper

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBase32(t *testing.T) {
	require.Equal(t, "6D1W3Q7E2I", Base32Encode(1723432993291822))
	require.Equal(t, int64(1723432993291822), Base32Decode("6D1W3Q7E2I"))
}
func TestBHex2Num(t *testing.T) {
	require.Equal(t, BHex2Num("1E6K", 36), 65036)
	require.Equal(t, BHex2Num("FE0C", 16), 65036)

	require.Equal(t, NumToBHex(65036, 36), "1E6K")
	require.Equal(t, NumToBHex(65036, 16), "FE0C")
	fmt.Println(NumToBHex(1723432993291822, 36))
	fmt.Println(NumToBHex(1723432993291822, 16))
}

/*
cpu: AMD Ryzen 7 7840HS with Radeon 780M Graphics
BenchmarkBase32-16      10618245               108.4 ns/op/
*/
func BenchmarkBase32(b *testing.B) {
	v := time.Now().UnixMicro()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v++
		Base32Encode(v)
	}
}

/*
cpu: AMD Ryzen 7 7840HS with Radeon 780M Graphics
BenchmarkHex_36_1-16             5230672               216.6 ns/op
BenchmarkHex_36_2-16             6296904               185.0 ns/op
*/
func BenchmarkHex_36_1(b *testing.B) {
	v := time.Now().UnixMicro()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v++
		NumToBHex(v, 36)
	}
}

func BenchmarkHex_36_2(b *testing.B) {
	// v := time.Now().UnixMicro()
	v := "GYWM6QHG6M"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BHex2Num(v, 36)
	}
}
