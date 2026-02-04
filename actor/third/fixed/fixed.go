package fixed

import (
	"actor/third/fixed/fastfixed"
	"actor/third/fixed/slowfixed"
)

var (
	NaN = Fixed{
		slow: slowfixed.NaN,
		fast: fastfixed.NaN,
	}
	ZERO = Fixed{
		slow: slowfixed.ZERO,
		fast: fastfixed.ZERO,
	}
	MINUS_ONE = Fixed{
		slow: slowfixed.MINUS_ONE,
		fast: fastfixed.MustParse("-1"),
	}
	ONE = Fixed{
		slow: slowfixed.ONE,
		fast: fastfixed.ONE,
	}
	TWO = Fixed{
		slow: slowfixed.TWO,
		fast: fastfixed.TWO,
	}
	THREE = Fixed{
		slow: slowfixed.THREE,
		fast: fastfixed.THREE,
	}
	FIVE = Fixed{
		slow: slowfixed.FIVE,
		fast: fastfixed.FIVE,
	}
	TEN = Fixed{
		slow: slowfixed.TEN,
		fast: fastfixed.TEN,
	}
	HUNDRED = Fixed{
		slow: slowfixed.HUNDRED,
		fast: fastfixed.HUNDRED,
	}
	BIG = Fixed{
		slow: slowfixed.BIG,
		fast: fastfixed.BIG,
	} // 目前允许的安全最大值
)

// 默认使用decimal库作为底层处理
var useHighDecimal = true

func IsHighDecimal() bool {
	return useHighDecimal
}

// TurnOnFastDecimal
// 在需要快速低精度数据的时候 调用此函数
// 一定要在创建任何fixed struct实例之前调用
func TurnOnFastDecimal() {
	useHighDecimal = false
}

// TurnOnHighDecimal
// 在需要高精度数据的时候 调用此函数
// 一定要在创建任何fixed struct实例之前调用
func TurnOnHighDecimal() {
	useHighDecimal = true
}

type Fixed struct {
	slow slowfixed.SlowFixed
	fast fastfixed.FastFixed
}

func NewS(d string) Fixed {
	if useHighDecimal {
		return Fixed{slow: slowfixed.SlowFixedNewS(d)}
	} else {
		return Fixed{fast: fastfixed.FastFixedNewS(d)}
	}
}

func Min(a, b Fixed) Fixed {
	if a.LessThan(b) {
		return a
	} else {
		return b
	}
}

func NewF(d float64) Fixed {
	if useHighDecimal {
		return Fixed{slow: slowfixed.SlowFixedNewF(d)}
	} else {
		return Fixed{fast: fastfixed.FastFixedNewF(d)}
	}
}

func NewI(d int64, n uint) Fixed {
	if useHighDecimal {
		return Fixed{slow: slowfixed.SlowFixedNewI(d)}
	} else {
		return Fixed{fast: fastfixed.FastFixedNewI(d, n)}
	}
}

func (d Fixed) String() string {
	if useHighDecimal {
		return d.slow.String()
	} else {
		return d.fast.String()
	}
}

func (d Fixed) Float() float64 {
	if useHighDecimal {
		return d.slow.Float64()
	} else {
		return d.fast.Float()
	}
}

func (d Fixed) Int64() int64 {
	if useHighDecimal {
		return d.slow.Int64()
	} else {
		return d.fast.Int64()
	}
}

func (d Fixed) Abs() Fixed {
	if useHighDecimal {
		return Fixed{slow: d.slow.Abs()}
	} else {
		return Fixed{fast: d.fast.Abs()}
	}
}

func (d Fixed) Add(n Fixed) Fixed {
	if useHighDecimal {
		return Fixed{slow: d.slow.Add(n.slow)}
	} else {
		return Fixed{fast: d.fast.Add(n.fast)}
	}
}

func (d Fixed) Sub(n Fixed) Fixed {
	if useHighDecimal {
		return Fixed{slow: d.slow.Sub(n.slow)}
	} else {
		return Fixed{fast: d.fast.Sub(n.fast)}
	}
}

func (d Fixed) Mul(n Fixed) Fixed {
	if useHighDecimal {
		return Fixed{slow: d.slow.Mul(n.slow)}
	} else {
		return Fixed{fast: d.fast.Mul(n.fast)}
	}
}

func (d Fixed) Div(n Fixed) Fixed {
	if useHighDecimal {
		return Fixed{slow: d.slow.Div(n.slow)}
	} else {
		return Fixed{fast: d.fast.Div(n.fast)}
	}
}

func (d Fixed) GreaterThan(n Fixed) bool {
	if useHighDecimal {
		return d.slow.GreaterThan(n.slow)
	} else {
		return d.fast.GreaterThan(n.fast)
	}
}

func (d Fixed) GreaterThanOrEqual(n Fixed) bool {
	if useHighDecimal {
		return d.slow.GreaterThanOrEqual(n.slow)
	} else {
		return d.fast.GreaterThanOrEqual(n.fast)
	}
}

func (d Fixed) LessThan(n Fixed) bool {
	if useHighDecimal {
		return d.slow.LessThan(n.slow)
	} else {
		return d.fast.LessThan(n.fast)
	}
}

func (d Fixed) LessThanOrEqual(n Fixed) bool {
	if useHighDecimal {
		return d.slow.LessThanOrEqual(n.slow)
	} else {
		return d.fast.LessThanOrEqual(n.fast)
	}
}

func (d Fixed) Equal(n Fixed) bool {
	if useHighDecimal {
		return d.slow.Equal(n.slow)
	} else {
		return d.fast.Equal(n.fast)
	}
}

func (d Fixed) IsZero() bool {
	if useHighDecimal {
		return d.slow.IsZero()
	} else {
		return d.fast.IsZero()
	}
}

func (d Fixed) Cmp(n Fixed) int {
	if useHighDecimal {
		return d.slow.Cmp(n.slow)
	} else {
		return d.fast.Cmp(n.fast)
	}
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (d *Fixed) UnmarshalJSON(bytes []byte) error {
	if useHighDecimal {
		d.slow.UnmarshalJSON(bytes)
	} else {
		d.slow.UnmarshalJSON(bytes)
		d.fast = fastfixed.FastFixedNewS(d.slow.String())
	}
	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (d Fixed) MarshalJSON() ([]byte, error) {
	if useHighDecimal {
		return d.slow.MarshalJSON()
	} else {
		f0 := slowfixed.SlowFixedNewS(d.fast.String())
		return f0.MarshalJSON()
	}
}
