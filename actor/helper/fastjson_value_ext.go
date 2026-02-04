package helper

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/valyala/fastjson"
)

func MustGetFloat64(val *fastjson.Value, keys ...string) float64 {
	v := val.Get(keys...)
	if v == nil {
		panic(fmt.Sprintf("not exist: %v, %v ", val, keys))
	}
	if v.Type() != fastjson.TypeNumber {
		if r, err := v.StringBytes(); err != nil {
			panic("neither float64 nor string")
		} else {
			return BytesToFloat64(r)
		}
	}
	if r, err := v.Float64(); err != nil {
		panic(err)
	} else {
		return r
	}
}

// 不存在返回0
func GetFloat64(v *fastjson.Value, keys ...string) float64 {
	v = v.Get(keys...)
	if v == nil {
		return 0.0
	}
	if v.Type() != fastjson.TypeNumber {
		if r, err := v.StringBytes(); err != nil {
			panic("neither float64 nor string")
		} else {
			return BytesToFloat64(r)
		}
	}
	if r, err := v.Float64(); err != nil {
		panic(err)
	} else {
		return r
	}
}
func MustGetInt64FromBytes(v *fastjson.Value, keys ...string) int64 {
	v = v.Get(keys...)
	if v == nil || v.Type() != fastjson.TypeString {
		panic(fmt.Sprintf("not string or exist: %v ", v))
	}
	if r, err := v.StringBytes(); err != nil {
		panic(err)
	} else {
		return BytesToInt64(r)
	}
}
func MustGetIntFromBytes(v *fastjson.Value, keys ...string) int {
	v = v.Get(keys...)
	if v == nil || v.Type() != fastjson.TypeString {
		panic(fmt.Sprintf("not string or exist: %v ", v))
	}
	if r, err := v.StringBytes(); err != nil {
		panic(err)
	} else {
		return int(BytesToInt64(r))
	}
}
func GetInt64FromBytes(v *fastjson.Value, keys ...string) int64 {
	v = v.Get(keys...)
	if v == nil {
		return 0
	}
	if v.Type() == fastjson.TypeNull {
		return 0
	}
	if v.Type() != fastjson.TypeString {
		panic(fmt.Sprintf("not string or exist: %v ", v))
	}
	if r, err := v.StringBytes(); err != nil {
		panic(err)
	} else {
		return BytesToInt64(r)
	}
}

func GetFloat64FromBytes(v *fastjson.Value, keys ...string) float64 {
	v = v.Get(keys...)
	if v == nil {
		return 0.0
	}
	if v.Type() == fastjson.TypeNull {
		return 0.0
	}
	if v.Type() != fastjson.TypeString {
		panic(fmt.Sprintf("not string or exist: %v ", v))
	}
	if r, err := v.StringBytes(); err != nil {
		panic(err)
	} else {
		return BytesToFloat64(r)
	}
}
func MustGetFloat64FromBytes(v *fastjson.Value, keys ...string) float64 {
	v = v.Get(keys...)
	if v == nil || v.Type() != fastjson.TypeString {
		panic(fmt.Sprintf("not string or exist: %v ", v))
	}
	if r, err := v.StringBytes(); err != nil {
		panic(err)
	} else {
		return BytesToFloat64(r)
	}
}

// 不会复制string，注意[]byte可能被释放
func MustGetShadowStringFromBytes(v *fastjson.Value, keys ...string) string {
	v = v.Get(keys...)
	if v == nil || v.Type() != fastjson.TypeString {
		panic(fmt.Sprintf("not string or exist: %v ", v))
	}
	if r, err := v.StringBytes(); err != nil {
		panic(err)
	} else {
		return BytesToString(r)
	}
}

// 会复制string，不担心v中底层[]byte被释放
func MustGetStringFromBytes(v *fastjson.Value, keys ...string) string {
	v = v.Get(keys...)
	if v == nil || v.Type() != fastjson.TypeString {
		panic(fmt.Sprintf("not string or exist: %v ", v))
	}
	if r, err := v.StringBytes(); err != nil {
		panic(err)
	} else {
		return string(r)
	}
}
func MustGetStringLowerFromBytes(v *fastjson.Value, keys ...string) string {
	return strings.ToLower(MustGetShadowStringFromBytes(v, keys...))
}
func MustGetStringUpperFromBytes(v *fastjson.Value, keys ...string) string {
	return strings.ToUpper(MustGetShadowStringFromBytes(v, keys...))
}

func GetStringFromBytes(v *fastjson.Value, keys ...string) string {
	v = v.Get(keys...)
	if v == nil || v.Type() != fastjson.TypeString {
		return ""
	}
	if r, err := v.StringBytes(); err != nil {
		return ""
	} else {
		return string(r)
	}
}

func GetShadowStringFromBytes(v *fastjson.Value, keys ...string) string {
	v = v.Get(keys...)
	if v == nil || v.Type() != fastjson.TypeString {
		return ""
	}
	if v.Type() == fastjson.TypeNull {
		return ""
	}
	if r, err := v.StringBytes(); err != nil {
		return ""
	} else {
		return BytesToString(r)
	}
}

func MustGetArray(v *fastjson.Value, keys ...string) []*fastjson.Value {
	if v.Exists(keys...) {
		return GetArray(v, keys...)
	}
	panic(fmt.Sprintf("not array or exist: %v", v))

	// res := v.GetArray(keys...)
	// if res == nil {
	// panic(fmt.Sprintf("not array or exist: %v ", v))
	// }
	// return res
}

func GetArray(v *fastjson.Value, keys ...string) []*fastjson.Value {
	res := v.GetArray(keys...)
	if res == nil {
		return []*fastjson.Value{}
	}
	return res
}

func MustGetStringBytes(v *fastjson.Value, keys ...string) []byte {
	v = v.Get(keys...)
	if v == nil || v.Type() != fastjson.TypeString {
		panic(fmt.Sprintf("not string or exist: %v ", v))
	}
	if r, err := v.StringBytes(); err != nil {
		panic(err)
	} else {
		return r
	}
}

func GetInt(v *fastjson.Value, keys ...string) int {
	v = v.Get(keys...)
	if v == nil {
		return 0
	}
	if v.Type() != fastjson.TypeNumber {
		if r, err := v.StringBytes(); err != nil {
			panic("neither int nor string")
		} else {
			return int(BytesToInt64(r))
		}
	}
	if r, err := v.Int(); err != nil {
		panic(err)
	} else {
		return r
	}
}

// 不存在返回0
func GetInt64(v *fastjson.Value, keys ...string) int64 {
	v = v.Get(keys...)
	if v == nil {
		return 0
	}
	if v.Type() != fastjson.TypeNumber {
		if r, err := v.StringBytes(); err != nil {
			panic("neither int64 nor string")
		} else {
			return BytesToInt64(r)
		}
	}
	if r, err := v.Int64(); err != nil {
		panic(err)
	} else {
		return r
	}
}

var re = regexp.MustCompile(`^0\.0+`)

func MustGetInt64(val *fastjson.Value, keys ...string) int64 {
	v := val.Get(keys...)
	if v == nil {
		panic(fmt.Sprintf("not exist: %v, %v ", val, keys))
	}
	if v.Type() != fastjson.TypeNumber {
		r, err := v.StringBytes()
		if err != nil {
			panic("neither int64 nor string")
		} else {
			res := BytesToInt64(r)
			if res == 0 && (BytesToString(r) != "0" && !re.MatchString(BytesToString(r))) {
				panic("not number type string. " + BytesToString(r))
			}
			return res
		}
	}
	if r, err := v.Int64(); err != nil {
		panic(err)
	} else {
		return r
	}
}

func MustGetInt(val *fastjson.Value, keys ...string) int {
	v := val.Get(keys...)
	if v == nil {
		panic(fmt.Sprintf("not exist: %v, %v ", val, keys))
	}
	if v.Type() != fastjson.TypeNumber {
		if r, err := v.StringBytes(); err != nil {
			panic("neither int64 nor string")
		} else {
			r2, err2 := strconv.Atoi(BytesToString(r))
			if err2 != nil {
				panic(err2)
			} else {
				if r2 == 0 && (BytesToString(r) != "0" && !re.MatchString(BytesToString(r))) {
					panic("not number type string. " + BytesToString(r))
				}
				return r2
			}
		}
	}
	if r, err := v.Int(); err != nil {
		panic(err)
	} else {
		return r
	}
}

func MustGetBool(v *fastjson.Value, keys ...string) bool {
	v = v.Get(keys...)
	if v == nil {
		panic(fmt.Sprintf("not bool or exist: %v ", v))
	}
	switch v.Type() {
	case fastjson.TypeTrue:
		return true
	case fastjson.TypeFalse:
		return false
	default:
		panic("not bool")
	}
}

func GetBool(v *fastjson.Value, keys ...string) bool {
	v = v.Get(keys...)
	if v == nil {
		return false
	}
	switch v.Type() {
	case fastjson.TypeTrue:
		return true
	}
	return false
}
