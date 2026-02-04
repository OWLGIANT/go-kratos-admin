package fastfixed

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

type FastFixed struct {
	fp int64
}

// the following constants can be changed to configure a different number of decimal places - these are
// the only required changes. only 18 significant digits are supported due to NaN

const NPlaces = 10
const scale = int64(10 * 10 * 10 * 10 * 10 * 10 * 10 * 10 * 10 * 10)

// const scale = int64(10 * 10 * 10 * 10 * 10 * 10 * 10 )
const zeros = "0000000000"

// const zeros = "0000000"
const MAX = float64(99999999.9999999999)

const nan = int64(1<<63 - 1)

var (
	NaN = FastFixed{fp: nan}

	//MAX = float64(nan)/math.Pow10(nPlaces)
	ZERO    = FastFixed{fp: 0}
	ONE     = FastFixed{fp: 1 * int64(math.Pow10(NPlaces))}
	TWO     = FastFixed{fp: 2 * int64(math.Pow10(NPlaces))}
	THREE   = FastFixed{fp: 3 * int64(math.Pow10(NPlaces))}
	FOUR    = FastFixed{fp: 4 * int64(math.Pow10(NPlaces))}
	FIVE    = FastFixed{fp: 5 * int64(math.Pow10(NPlaces))}
	SIX     = FastFixed{fp: 6 * int64(math.Pow10(NPlaces))}
	SEVEN   = FastFixed{fp: 7 * int64(math.Pow10(NPlaces))}
	EIGHT   = FastFixed{fp: 8 * int64(math.Pow10(NPlaces))}
	NINE    = FastFixed{fp: 9 * int64(math.Pow10(NPlaces))}
	TEN     = FastFixed{fp: 10 * int64(math.Pow10(NPlaces))}
	HUNDRED = FastFixed{fp: 100 * int64(math.Pow10(NPlaces))}
	BIG     = FastFixedNewF(99999999) // 目前允许的安全最大值
)

var errTooLarge = errors.New("significand too large")
var errFormat = errors.New("invalid encoding")

func init() {
	if int64(math.Pow10(NPlaces)) != scale {
		panic(fmt.Sprintf("scale(%d) != Pow10(nPlaces)(%d)", scale, int64(math.Pow10(NPlaces))))
	}
	if int64(len(zeros)) != NPlaces {
		panic(fmt.Sprintf("nPlaces(%d) != zeros(%s) length(%d)", NPlaces, zeros, len(zeros)))
	}
}

// NewS creates a new Fixed from a string, returning NaN if the string could not be parsed
func FastFixedNewS(s string) FastFixed {
	f, _ := NewSErr(s)
	return f
}

// NewSErr creates a new Fixed from a string, returning NaN, and error if the string could not be parsed
func NewSErr(s string) (FastFixed, error) {
	if strings.ContainsAny(s, "eE") {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return NaN, err
		}
		return FastFixedNewF(f), nil
	}
	if "NaN" == s {
		return NaN, nil
	}
	period := strings.Index(s, ".")
	var i int64
	var f int64
	var sign int64 = 1
	var err error
	if period == -1 {
		i, err = strconv.ParseInt(s, 10, 64)
		if err != nil {
			return NaN, errors.New("cannot parse")
		}
		if i < 0 {
			sign = -1
			i = i * -1
		}
	} else {
		if len(s[:period]) > 0 {
			i, err = strconv.ParseInt(s[:period], 10, 64)
			if err != nil {
				return NaN, errors.New("cannot parse")
			}
			if i < 0 || s[0] == '-' {
				sign = -1
				i = i * -1
			}
		}
		fs := s[period+1:]
		fs = fs + zeros[:max(0, NPlaces-len(fs))]
		f, err = strconv.ParseInt(fs[0:NPlaces], 10, 64)
		if err != nil {
			return NaN, errors.New("cannot parse")
		}
	}
	if float64(i) > MAX {
		return NaN, errTooLarge
	}
	return FastFixed{fp: sign * (i*scale + f)}, nil
}

// Parse creates a new Fixed from a string, returning NaN, and error if the string could not be parsed. Same as NewSErr
// but more standard naming
func Parse(s string) (FastFixed, error) {
	return NewSErr(s)
}

// MustParse creates a new Fixed from a string, and panics if the string could not be parsed
func MustParse(s string) FastFixed {
	f, err := NewSErr(s)
	if err != nil {
		panic(err)
	}
	return f
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// FastFixedNewF creates a Fixed from an float64, rounding at the 8th decimal place
func FastFixedNewF(f float64) FastFixed {
	if math.IsNaN(f) {
		return FastFixed{fp: nan}
	}
	if f >= MAX || f <= -MAX {
		return NaN
	}
	round := .5
	if f < 0 {
		round = -0.5
	}

	return FastFixed{fp: int64(f*float64(scale) + round)}
}

// NewI creates a Fixed for an integer, moving the decimal point n places to the left
// For example, NewI(123,1) becomes 12.3. If n > 7, the value is truncated
func FastFixedNewI(i int64, n uint) FastFixed {
	if n > NPlaces {
		i = i / int64(math.Pow10(int(n-NPlaces)))
		n = NPlaces
	}

	i = i * int64(math.Pow10(int(NPlaces-n)))

	return FastFixed{fp: i}
}

func (f FastFixed) IsNaN() bool {
	return f.fp == nan
}

func (f FastFixed) IsZero() bool {
	return f.Equal(ZERO)
}

// Sign returns:
//
//	-1 if f <  0
//	 0 if f == 0 or NaN
//	+1 if f >  0
func (f FastFixed) Sign() int {
	if f.IsNaN() {
		return 0
	}
	return f.Cmp(ZERO)
}

// Float converts the Fixed to a float64
func (f FastFixed) Float() float64 {
	if f.IsNaN() {
		return math.NaN()
	}
	return float64(f.fp) / float64(scale)
}

// Add adds f0 to f producing a Fixed. If either operand is NaN, NaN is returned
func (f FastFixed) Add(f0 FastFixed) FastFixed {
	if f.IsNaN() || f0.IsNaN() {
		return NaN
	}
	return FastFixed{fp: f.fp + f0.fp}
}

// Sub subtracts f0 from f producing a Fixed. If either operand is NaN, NaN is returned
func (f FastFixed) Sub(f0 FastFixed) FastFixed {
	if f.IsNaN() || f0.IsNaN() {
		return NaN
	}
	return FastFixed{fp: f.fp - f0.fp}
}

// Abs returns the absolute value of f. If f is NaN, NaN is returned
func (f FastFixed) Abs() FastFixed {
	if f.IsNaN() {
		return NaN
	}
	if f.Sign() >= 0 {
		return f
	}
	f0 := FastFixed{fp: f.fp * -1}
	return f0
}

func abs(i int64) int64 {
	if i >= 0 {
		return i
	}
	return i * -1
}

// Mul multiplies f by f0 returning a Fixed. If either operand is NaN, NaN is returned
func (f FastFixed) Mul(f0 FastFixed) FastFixed {
	if f.IsNaN() || f0.IsNaN() {
		return NaN
	}

	fp_a := f.fp / scale
	fp_b := f.fp % scale

	fp0_a := f0.fp / scale
	fp0_b := f0.fp % scale

	var result int64

	if fp0_a != 0 {
		result = fp_a*fp0_a*scale + fp_b*fp0_a
	}
	if fp0_b != 0 {
		result = result + (fp_a * fp0_b) + ((fp_b)*fp0_b)/scale
	}

	return FastFixed{fp: result}
}

// Div divides f by f0 returning a Fixed. If either operand is NaN, NaN is returned
func (f FastFixed) Div(f0 FastFixed) FastFixed {
	if f.IsNaN() || f0.IsNaN() {
		return NaN
	}
	return FastFixedNewF(f.Float() / f0.Float())
}

// Round returns a rounded (half-up, away from zero) to n decimal places
func (f FastFixed) Round(n int) FastFixed {
	if f.IsNaN() {
		return NaN
	}
	if n > 0 {
		round := .5
		if f.fp < 0 {
			round = -0.5
		}
		pow10 := math.Pow10(n)
		f0 := f.Frac()
		f0 = f0*pow10 + round
		f0 = float64(int(f0)) / pow10
		return FastFixedNewF(float64(f.Int64()) + f0)
	} else if n == 0 {
		return FastFixedNewF(float64(f.Int64()))
	} else {
		n = -n
		pow10 := math.Pow10(n)
		return FastFixedNewF(pow10 * float64(f.Int64()/int64(pow10)))
	}
}

// Round returns a rounded (half-up, away from zero) to n decimal places
// 四舍五入
func (f FastFixed) RoundUp(n int) FastFixed {
	return f.Round(n)
}

func (f FastFixed) RoundUpWithStep(n int, step float64 /*tickSize*/) FastFixed {
	if f.IsNaN() {
		return NaN
	}
	if n > 0 {
		//round := .5
		//if f.fp < 0 {
		//	round = -0.5
		//}
		pow10 := math.Pow10(n)
		f0 := f.Frac()
		f1 := f0 / step
		f2 := math.Trunc(f1)
		f4 := 1 / pow10 / step
		if f1 >= float64(f2)+f4 {
			f2++
		}
		f3 := float64(f2) * step
		return FastFixedNewF(float64(f.Int64()) + f3)
	} else if n == 0 {
		return FastFixedNewF(float64(f.Int64()))
	} else {
		n = -n
		pow10 := math.Pow10(n)
		return FastFixedNewF(pow10 * float64(f.Int64()/int64(pow10)))
	}

}

func (f FastFixed) RoundDownWithStep(n int, step float64 /*tickSize*/) FastFixed {
	if f.IsNaN() {
		return NaN
	}
	if n > 0 {
		//round := .5
		//if f.fp < 0 {
		//	round = -0.5
		//}
		pow10 := math.Pow10(n)
		f0 := f.Frac()
		f1 := f0 / step
		f2 := int(f1)
		if f1 < float64(f2)+1/pow10/step {
			f2++
		}
		f3 := float64(f2) * step
		return FastFixedNewF(float64(f.Int64()) + f3 - step)
	} else if n == 0 {
		return FastFixedNewF(float64(f.Int64()))
	} else {
		n = -n
		pow10 := math.Pow10(n)
		return FastFixedNewF(pow10 * float64(f.Int64()/int64(pow10)))
	}
}

// func (f Fixed) RoundUpWithStep2(step float64 /*tickSize*/) Fixed {
func (f FastFixed) RoundUpWithStep2(step FastFixed /*tickSize*/) FastFixed {
	if f.IsNaN() {
		return NaN
	}

	f1 := f.Div(step).Float()
	f2 := math.Ceil(f1 * 10)
	f3 := f2 / 10
	f44 := math.Trunc(f3)
	f4 := math.Floor(f3)
	fmt.Println(f44)
	//f2 := math.Floor(math.Floor(f1*10)/10)
	f5 := FastFixedNewF(f4)
	return f5.Mul(step)
	//
	//	//f0 := f.Frac()
	//	//f1 := f0 / step
	//	//f2 := int(f1)
	//	//f4 := 1 / pow10 / step
	//	//if f1 >= float64(f2)+f4 {
	//	//	f2++
	//	//}
	//	//f3 := float64(f2) * step
	//	//return NewF(float64(f.Int64()) + f3)
}

func (f FastFixed) RoundDownWithStep2(step float64 /*tickSize*/) FastFixed {
	if f.IsNaN() {
		return NaN
	}
	f1 := f.Float() / step
	f2 := math.Trunc(f1)
	f3 := FastFixedNewF(f2 * step)
	return f3
}

// Round returns a rounded (half-up, away from zero) to n decimal places
// 截断
func (f FastFixed) RoundDown(n int) FastFixed {
	if f.IsNaN() {
		return NaN
	}
	if n > 0 {
		f0 := f.Frac()
		pow10 := math.Pow10(n)
		f0 = f0 * pow10
		f0 = float64(int(f0)) / pow10
		return FastFixedNewF(float64(f.Int64()) + f0)
	} else if n == 0 {
		return FastFixedNewF(float64(f.Int64()))
	} else {
		n = -n
		pow10 := math.Pow10(n)
		return FastFixedNewF(pow10 * float64(f.Int64()/int64(pow10)))
	}
}

// Equal returns true if the f == f0. If either operand is NaN, false is returned. Use IsNaN() to test for NaN
func (f FastFixed) Equal(f0 FastFixed) bool {
	if f.IsNaN() || f0.IsNaN() {
		return false
	}
	return f.Cmp(f0) == 0
}

// GreaterThan tests Cmp() for 1
func (f FastFixed) GreaterThan(f0 FastFixed) bool {
	return f.Cmp(f0) == 1
}

// GreaterThaOrEqual tests Cmp() for 1 or 0
func (f FastFixed) GreaterThanOrEqual(f0 FastFixed) bool {
	cmp := f.Cmp(f0)
	return cmp == 1 || cmp == 0
}

// LessThan tests Cmp() for -1
func (f FastFixed) LessThan(f0 FastFixed) bool {
	return f.Cmp(f0) == -1
}

// LessThan tests Cmp() for -1 or 0
func (f FastFixed) LessThanOrEqual(f0 FastFixed) bool {
	cmp := f.Cmp(f0)
	return cmp == -1 || cmp == 0
}

// Cmp compares two Fixed. If f == f0, return 0. If f > f0, return 1. If f < f0, return -1. If both are NaN, return 0. If f is NaN, return 1. If f0 is NaN, return -1
func (f FastFixed) Cmp(f0 FastFixed) int {
	if f.IsNaN() && f0.IsNaN() {
		return 0
	}
	if f.IsNaN() {
		return 1
	}
	if f0.IsNaN() {
		return -1
	}

	if f.fp == f0.fp {
		return 0
	}
	if f.fp < f0.fp {
		return -1
	}
	return 1
}

// String converts a Fixed to a string, dropping trailing zeros
func (f FastFixed) String() string {
	s, point := f.tostr()
	if point == -1 {
		return s
	}
	index := len(s) - 1
	for ; index != point; index-- {
		if s[index] != '0' {
			return s[:index+1]
		}
	}
	return s[:point]
}

// StringN converts a Fixed to a String with a specified number of decimal places, truncating as required
func (f FastFixed) StringN(decimals int) string {

	s, point := f.tostr()

	if point == -1 {
		return s
	}
	if decimals == 0 {
		return s[:point]
	} else {
		return s[:point+decimals+1]
	}
}

func (f FastFixed) tostr() (string, int) {
	fp := f.fp
	if fp == 0 {
		return "0." + zeros, 1
	}
	if fp == nan {
		return "NaN", -1
	}

	b := make([]byte, 24)
	b = itoa(b, fp)

	return string(b), len(b) - NPlaces - 1
}

func itoa(buf []byte, val int64) []byte {
	neg := val < 0
	if neg {
		val = val * -1
	}

	i := len(buf) - 1
	idec := i - NPlaces
	for val >= 10 || i >= idec {
		buf[i] = byte(val%10 + '0')
		i--
		if i == idec {
			buf[i] = '.'
			i--
		}
		val /= 10
	}
	buf[i] = byte(val + '0')
	if neg {
		i--
		buf[i] = '-'
	}
	return buf[i:]
}

// Int64 return the integer portion of the Fixed, or 0 if NaN
func (f FastFixed) Int64() int64 {
	if f.IsNaN() {
		return 0
	}
	return f.fp / scale
}

// Frac return the fractional portion of the Fixed, or NaN if NaN
func (f FastFixed) Frac() float64 {
	if f.IsNaN() {
		return math.NaN()
	}
	return float64(f.fp%scale) / float64(scale)
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface
func (f *FastFixed) UnmarshalBinary(data []byte) error {
	fp, n := binary.Varint(data)
	if n < 0 {
		return errFormat
	}
	f.fp = fp
	return nil
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (f FastFixed) MarshalBinary() (data []byte, err error) {
	var buffer [binary.MaxVarintLen64]byte
	n := binary.PutVarint(buffer[:], f.fp)
	return buffer[:n], nil
}

// WriteTo write the Fixed to an io.Writer, returning the number of bytes written
func (f FastFixed) WriteTo(w io.ByteWriter) error {
	return writeVarint(w, f.fp)
}

// ReadFrom reads a Fixed from an io.Reader
func ReadFrom(r io.ByteReader) (FastFixed, error) {
	fp, err := binary.ReadVarint(r)
	if err != nil {
		return NaN, err
	}
	return FastFixed{fp: fp}, nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (f *FastFixed) UnmarshalJSON(bytes []byte) error {
	s := string(bytes)
	if s == "null" {
		return nil
	}
	if s == "\"NaN\"" {
		*f = NaN
		return nil
	}

	fixed, err := NewSErr(s)
	*f = fixed
	if err != nil {
		return fmt.Errorf("Error decoding string '%s': %s", s, err)
	}
	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (f FastFixed) MarshalJSON() ([]byte, error) {
	if f.IsNaN() {
		return []byte("\"NaN\""), nil
	}
	buffer := make([]byte, 24)
	return itoa(buffer, f.fp), nil
}

// The binary encoding package does not offer 'write' methods so the allocation costs in WriteTo are high
// so we duplicate the code here and implement them

// WriteUvarint encodes a uint64 onto w
func writeUvarint(w io.ByteWriter, x uint64) error {
	i := 0
	for x >= 0x80 {
		err := w.WriteByte(byte(x) | 0x80)
		if err != nil {
			return err
		}
		x >>= 7
		i++
	}
	return w.WriteByte(byte(x))
}

// WriteVarint encodes an int64 onto w
func writeVarint(w io.ByteWriter, x int64) error {
	ux := uint64(x) << 1
	if x < 0 {
		ux = ^ux
	}
	return writeUvarint(w, ux)
}
