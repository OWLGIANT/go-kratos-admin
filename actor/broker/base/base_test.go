package base

import (
	"fmt"
	"math"
	"sync/atomic"
	"testing"

	"actor/helper"
	"actor/third/fixed"
	"github.com/valyala/fastjson/fastfloat"
)

func TestFixedAmount(t *testing.T) {
	valShare := 226.85150000000004
	price := 2268.52
	multi := fixed.NewS("0.1")
	orderAmount := helper.FixAmount(fixed.NewF(valShare/(price*multi.Float())), 0.1)
	fmt.Println("orderAmount", orderAmount)
}

func TestXxx(t *testing.T) {
	v := atomic.Int32{}
	v.Store(int32(math.Pow(2, 30)))
	fmt.Printf("%d\n", v.Add(1))
	fmt.Printf("%.6d\n", v.Add(1))
	fmt.Printf("%.6d\n", v.Add(1))
	fmt.Printf("%.6d\n", v.Add(1))
	fmt.Printf("%.6d\n", v.Add(1))
}

func TestFastjsonFixed(t *testing.T) {
	amt := "0.001641998731566833"
	frozen := fastfloat.ParseBestEffort(amt)
	fmt.Println(frozen)
	msg := fmt.Sprintf("Coin %f ", frozen)
	fmt.Println(msg)
	msg1 := fmt.Sprintf("Coin %.18f ", frozen)
	fmt.Println(msg1)
}
