package fixed

import (
	"fmt"
	"math/big"
	"testing"
)

func BenchmarkNewS(b *testing.B) {
	//TurnOnFastDecimal()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewS("123.5345345")
	}
}

func BenchmarkDecimal_String(b *testing.B) {
	//TurnOnFastDecimal()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewS("123.5345345")
	}
}

func BenchmarkDecimal_Float(b *testing.B) {
	//TurnOnFastDecimal()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewF(123.5345345)
	}
}

func BenchmarkDecimal_Int(b *testing.B) {
	//TurnOnFastDecimal()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewI(123, 0)
	}
}

func BenchmarkDecimal_Mul(b *testing.B) {
	//TurnOnFastDecimal()
	f0 := NewF(1.0)
	f1 := NewF(2.0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f0.Mul(f1)
	}
}

func BenchmarkDecimal_Div(b *testing.B) {
	//TurnOnFastDecimal()
	f0 := NewF(1.0)
	f1 := NewF(2.0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f0.Div(f1)
	}
}

func BenchmarkMul_FastFixed(b *testing.B) {
	TurnOnFastDecimal()
	f0 := NewF(1.0)
	f1 := NewF(2.0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f0.Mul(f1)
	}
}

func BenchmarkDiv_FastFixed(b *testing.B) {
	TurnOnFastDecimal()
	f0 := NewF(1.0)
	f1 := NewF(2.0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f0.Div(f1)
	}
}
func BenchmarkMul_BigInt(b *testing.B) {
	f0 := big.NewInt(1.0)
	f1 := big.NewInt(2.0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f0.Mul(f0, f1)
	}
}

/*
BenchmarkDiv_FastFixed-16       329762923                3.525 ns/op
BenchmarkDiv_BigInt-16          100000000               11.46 ns/op
*/
func BenchmarkDiv_BigInt(b *testing.B) {
	f0 := big.NewInt(1.0)
	f1 := big.NewInt(2.0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f0.Div(f0, f1)
	}
}

func BenchmarkFixed_Add(b *testing.B) {
	//TurnOnFastDecimal()
	f0 := NewF(1.0)
	f1 := NewF(2.0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f0.Add(f1)
	}
}

func BenchmarkFixed_GreaterThan(b *testing.B) {
	//TurnOnFastDecimal()
	f0 := NewF(1.0)
	f1 := NewF(2.0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f0.GreaterThan(f1)
	}
}

func BenchmarkFixed_IsZero(b *testing.B) {
	//TurnOnFastDecimal()
	f0 := NewF(1.0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f0.IsZero()
	}
}

func BenchmarkFixed_Abs(b *testing.B) {
	//TurnOnFastDecimal()
	f0 := NewF(-1.0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f0.Abs()
	}
}

func TestFixed_MarshalJSON(t *testing.T) {
	f0 := NewS("1.234")
	fmt.Println(f0.MarshalJSON())
	TurnOnFastDecimal()
	f1 := NewS("1.234")
	fmt.Println(f1.MarshalJSON())
}

func TestFixed_UnmarshalJSON(t *testing.T) {
	b := []byte("1.234")
	f0 := NewS("1")
	fmt.Println(f0.UnmarshalJSON(b))
	fmt.Println(f0)
	TurnOnFastDecimal()
	f1 := NewS("1")
	fmt.Println(f1.UnmarshalJSON(b))
	fmt.Println(f1)
}
func TestFixed_Case1(t *testing.T) {
	multi := NewS("0.0001")
	var f float64 = 17
	longPos := NewF(f).Mul(multi)
	s := fmt.Sprintf("%s", longPos)
	fmt.Println(s)
	if longPos.GreaterThan(ZERO) {
		fmt.Println("true")
	}
}

//func TestFixed_MarshalBinary(t *testing.T) {
//	f0 := NewS("1.234")
//	fmt.Println(f0.MarshalBinary())
//	TurnOnFastDecimal()
//	f1 := NewS("1.234")
//	fmt.Println(f1.MarshalBinary())
//}
//
//func TestFixed_UnmarshalBinary(t *testing.T) {
//	encoding.BinaryUnmarshaler().UnmarshalBinary()
//	f0 := NewS("1")
//	fmt.Println(f0.UnmarshalBinary(b))
//	fmt.Println(f0)
//	TurnOnFastDecimal()
//	f1 := NewS("1")
//	fmt.Println(f1.UnmarshalBinary(b))
//	fmt.Println(f1)
//}
