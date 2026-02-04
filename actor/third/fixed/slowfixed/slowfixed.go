package slowfixed

import "github.com/shopspring/decimal"

var NaN = SlowFixed{dc: decimal.Zero} // todo decimal库不支持nan值
var BIG = SlowFixed{dc: decimal.NewFromFloat(999999999999)}
var HUNDRED = SlowFixed{dc: decimal.NewFromFloat(100)}
var TEN = SlowFixed{dc: decimal.NewFromFloat(10)}
var FIVE = SlowFixed{dc: decimal.NewFromFloat(5)}
var THREE = SlowFixed{dc: decimal.NewFromFloat(3)}
var TWO = SlowFixed{dc: decimal.NewFromFloat(2)}
var ONE = SlowFixed{dc: decimal.NewFromFloat(1)}
var ZERO = SlowFixed{dc: decimal.NewFromFloat(0)}
var MINUS_ONE = SlowFixed{dc: decimal.NewFromFloat(-1)}

type SlowFixed struct {
	dc decimal.Decimal
}

func SlowFixedNewS(d string) SlowFixed {
	x, _ := decimal.NewFromString(d)
	return SlowFixed{dc: x}
}

func SlowFixedNewI(d int64) SlowFixed {
	x := decimal.NewFromInt(d)
	return SlowFixed{dc: x}
}

func SlowFixedNewF(d float64) SlowFixed {
	x := decimal.NewFromFloat(d)
	return SlowFixed{dc: x}
}

func (d SlowFixed) String() string {
	return d.dc.String()
}

func (d SlowFixed) Float64() float64 {
	f, _ := d.dc.Float64()
	return f
}

func (d SlowFixed) Int64() int64 {
	return d.dc.BigInt().Int64()
}

func (d SlowFixed) Abs() SlowFixed {
	return SlowFixed{dc: d.dc.Abs()}
}

func (d SlowFixed) Add(n SlowFixed) SlowFixed {
	return SlowFixed{dc: d.dc.Add(n.dc)}
}

func (d SlowFixed) Sub(n SlowFixed) SlowFixed {
	return SlowFixed{dc: d.dc.Sub(n.dc)}
}

func (d SlowFixed) Mul(n SlowFixed) SlowFixed {
	return SlowFixed{dc: d.dc.Mul(n.dc)}
}

func (d SlowFixed) Div(n SlowFixed) SlowFixed {
	return SlowFixed{dc: d.dc.Div(n.dc)}
}

func (d SlowFixed) GreaterThan(n SlowFixed) bool {
	return d.dc.GreaterThan(n.dc)
}

func (d SlowFixed) GreaterThanOrEqual(n SlowFixed) bool {
	return d.dc.GreaterThanOrEqual(n.dc)
}

func (d SlowFixed) LessThan(n SlowFixed) bool {
	return d.dc.LessThan(n.dc)
}

func (d SlowFixed) LessThanOrEqual(n SlowFixed) bool {
	return d.dc.LessThanOrEqual(n.dc)
}

func (d SlowFixed) Equal(n SlowFixed) bool {
	return d.dc.Equal(n.dc)
}

func (d SlowFixed) IsZero() bool {
	return d.dc.IsZero()
}

func (d SlowFixed) Cmp(n SlowFixed) int {
	return d.dc.Cmp(n.dc)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (d *SlowFixed) UnmarshalJSON(bytes []byte) error {
	d.dc.UnmarshalJSON(bytes)
	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (d SlowFixed) MarshalJSON() ([]byte, error) {
	return d.dc.MarshalJSON()
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface
func (d *SlowFixed) UnmarshalBinary(data []byte) error {
	d.dc.UnmarshalBinary(data)
	return nil
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (d SlowFixed) MarshalBinary() (data []byte, err error) {
	return d.dc.MarshalBinary()
}

// MarshalText
func (d SlowFixed) MarshalText() (text []byte, err error) {
	return d.dc.MarshalText()
}

// UnmarshalText
func (d SlowFixed) UnmarshalText(text []byte) error {
	return d.dc.UnmarshalText(text)
}
