package tools // beast tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"actor/helper/helper_ding"
	"actor/limit"
	"actor/third/log"
)

// 基础工具集合，没有很多的业务依赖，将来可能独立成一个多项目共用的项目

// 泛型函数，判断slice是否包含一个元素
func Contains[T comparable](slice []T, item T) bool {
	if slice == nil {
		return false
	}
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

func FirstMatch[T comparable](slice []T, target T, match func(item, target T) bool) (T, bool) {
	if slice == nil {
		var t T
		return t, false
	}
	for _, v := range slice {
		if match(v, target) {
			return v, true
		}
	}
	var t T
	return t, false
}

const (
	AreaHK      = "HK"
	AreaSG      = "SG"
	AreaIE      = "IE"
	AreaJP      = "JP"
	AreaKR      = "KR"
	AreaUnKnown = "UNKNOWN_AREA"
)

// 避免循环依赖，先放这里
type Area struct {
	Area string `json:"area"`
}

var _AREAS = []string{AreaHK, AreaSG, AreaIE, AreaJP, AreaKR}

// todo 增加ip要加这里，日后cpp版本放入 beast_area.json获取
var SUBNET_AREA = map[string]string{
	"172.21":     AreaJP,
	"172.17":     AreaJP,
	"172.16":     AreaJP,
	"10.100.100": AreaJP,
	"172.18":     AreaSG,
	"172.20":     AreaSG,
	"172.14":     AreaKR,
	"192.168":    AreaHK,
}

func MustGetAreaFromIp(ip string) string {
	for pre, area := range SUBNET_AREA {
		if strings.HasPrefix(ip, pre) {
			return area
		}
	}
	// 免得上层处理error,直接panic，很低概率
	panic("无法获取area. " + ip)
}

// GetAreaFromIp 通过私有ip获取所在地区
func GetAreaFromIp(priIp string) string {
	for pre, area := range SUBNET_AREA {
		if strings.HasPrefix(priIp, pre) {
			return area
		}
	}
	return AreaUnKnown
}

// 通过内网ip前缀判断机器位置
func MustGetServerAreaDeprecatedIp() string {
	ips := limit.GetMyIP()
	areas := make([]string, 0)
	for _, ip := range ips {
		for pref, area := range SUBNET_AREA {
			if strings.HasPrefix(ip, pref) {
				areas = append(areas, area)
			}
		}
	}
	if len(areas) != 1 {
		log.Errorf("failed to get localtion. ips: %v", ips)
		helper_ding.DingingSendSerious(fmt.Sprintf("failed to get server area. ips: %s, %s", ips, helper_ding.AlertCarryInfo))
		log.Sync()
		time.Sleep(time.Second * 3)
		panic("failed to get location.")
	}
	return areas[0]
}

// 通过文件获取
func MustGetServerArea() string {
	content, err := os.ReadFile("./beast_area.json")
	if err != nil {
		log.Errorf("[utf_ign]读取area 文件失败 需要更新 %s. ip:%v", err.Error(), limit.GetMyIP())
		panic("[utf_ign]读取beast_area 文件失败 需要更新 ")
	}
	infos := Area{}
	err = json.Unmarshal(content, &infos)
	if err != nil {
		log.Errorf("[utf_ign]读取area 文件失败 需要更新 %s. ip: %v", err.Error(), limit.GetMyIP())
		panic("[utf_ign]读取beast_area 文件失败 需要更新 ")
	}

	area := strings.TrimSpace(infos.Area)
	for _, a := range _AREAS {
		if a == area {
			return a
		}
	}
	log.Errorf("[utf_ign]读取area 文件失败 需要更新 或者不支持改地区 %s", content)
	panic(fmt.Sprintf("[utf_ign]读取area 文件失败 需要更新 或者不支持改地区 %s", content))
}

// 通过出口IP获取本机器位置
func MustGetServerAreaDepreacted() string {

	// 获取本机位置
	client := http.Client{
		Timeout: 10 * time.Second, // 设置超时时间
	}
	var location string
	var msg string
	r, e := client.Get("http://ip-api.com/json")
	if e != nil {
		log.Errorf("获取本机位置失败:%v", e)
	} else {
		if r.StatusCode == 200 {
			body, _ := io.ReadAll(r.Body)
			msg = string(body)
			log.Infof("当前服务器位置msg:%v", msg)
			var m map[string]string
			json.Unmarshal(body, &m)
			if strings.Contains(m["city"], "Hong Kong") {
				location = AreaHK
			}
			if strings.Contains(m["city"], "Singapore") {
				location = AreaSG
			}
			if strings.Contains(m["countryCode"], "IE") {
				location = AreaIE
			}
			if strings.Contains(m["city"], "Tokyo") {
				location = AreaJP
			}
			if strings.Contains(m["countryCode"], "KR") {
				location = AreaKR
			}
			log.Infof("当前服务器位置:%v", location)
		}
		defer r.Body.Close()
	}
	if location == "" {
		log.Errorf("failed to get localtion. %s", msg)
		helper_ding.DingingSendSerious(fmt.Sprintf("failed to get server area. %s, %s", msg, helper_ding.AlertCarryInfo))
		log.Sync()
		time.Sleep(time.Second * 3)
		panic("failed to get location.")
	}
	return location
}

// 是否是ip格式字符串
func IsIpFormat(str string) bool {
	ipRegex := `\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`
	ipRegExp, err := regexp.Compile(ipRegex)
	if err != nil {
		log.Error("正则表达式编译失败:", err)
		return false
	}
	return ipRegExp.Match([]byte(str))
}

// 将ip转为一个int64 id，不足3位补充为0，最左0用9。
// 不可高频使用
func ConvertIpToId(ip string) int64 {
	if !IsIpFormat(ip) {
		log.Errorf("ip:%s is not a valid ip format.", ip)
		return 0
	}
	vals := strings.Split(ip, ".")
	str := strings.Builder{}
	str.Grow(12)
	for _, v := range vals {
		l := len(v)
		for i := 3; i > l; i-- {
			str.WriteString("0")
		}
		str.WriteString(v)
	}
	strToCon := str.String()
	if strings.HasPrefix(strToCon, "0") {
		str2 := strings.Builder{}
		str2.Grow(12)
		str2.WriteString("9")
		str2.WriteString(strToCon[1:])
		strToCon = str2.String()
	}
	if id, err :=
		strconv.ParseInt(strToCon, 10, 64); err == nil {
		return id
	} else {
		log.Errorf("failed to convert ip:%s to id. %s", ip, err.Error())
		return 0
	}
}

func MustConvertIpToId(ip string) int64 {
	r := ConvertIpToId(ip)
	if r == 0 {
		panic("ip:" + ip + " is not a valid ip format.")
	}
	return r
}
func SetField(objPtr interface{}, fieldName string, value interface{}) error {
	structValue := reflect.ValueOf(objPtr).Elem()
	fieldValue := structValue.FieldByName(fieldName)
	if !fieldValue.IsValid() {
		return fmt.Errorf("No such field: %s in obj", fieldName)
	}

	if !fieldValue.CanSet() {
		return fmt.Errorf("Cannot set field value for: %s", fieldName)
	}

	val := reflect.ValueOf(value)
	if fieldValue.Type() != val.Type() {
		return fmt.Errorf("Provided value type does not match field type")
	}

	fieldValue.Set(val)
	return nil
}

func GetStructName(structPtr interface{}) string {
	structType := reflect.TypeOf(structPtr)
	if structType.Kind() == reflect.Ptr {
		structType = structType.Elem()
	}
	return structType.Name()
}
func GetAllFields(structPtr interface{}) {
	val := reflect.ValueOf(structPtr).Elem()
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		fieldName := typ.Field(i).Name
		fmt.Printf("Field %s\n", fieldName)
	}
}
func GetAllMethods(structPtr interface{}) {
	structType := reflect.TypeOf(structPtr)

	for i := 0; i < structType.NumMethod(); i++ {
		method := structType.Method(i)
		fmt.Println("Method:", method.Name)
	}
}

func HasExportedMethod(structPtr interface{}, methodName string) (reflect.Method, bool) {
	structType := reflect.TypeOf(structPtr)
	for i := 0; i < structType.NumMethod(); i++ {
		method := structType.Method(i)
		// fmt.Println("Method:", method.Name)
		if methodName == method.Name {
			return method, true
		}
	}
	return reflect.Method{}, false
	// {
	//
	// structValue := reflect.ValueOf(structPtr)
	// fieldValue := structValue.MethodByName(methodName)
	// ok := fieldValue.IsValid()
	// log.Debugf("here checking, %v, %v", methodName, ok)
	// return ok
	// }
	// method 2
	// {
	// structType := reflect.TypeOf(structPtr)
	// method, ok := structType.MethodByName(methodName)
	// log.Debugf("here checking, %v, %v, %v", methodName, method, ok)
	// return ok && method.IsExported()
	// }
}

// func GetField(s interface{}, fielName string) (interface{}, bool ){
// structType := reflect.ValueOf(s)
// if structType.Kind() == reflect.Ptr {
// structType = structType.Elem()
// }
// for i := 0; i < structType.NumField(); i++ {
// field := structType.Field(i)
// if field. == fieldName {
// return field. true
// }
// }
// return false
// }

func GetStructFieldValue(s interface{}, fieldName string) interface{} {
	// r := reflect.ValueOf(s)
	r := reflect.ValueOf(s)
	if r.Kind() == reflect.Ptr {
		r = r.Elem()
	}
	f := reflect.Indirect(r).FieldByName(fieldName)

	if !f.IsValid() {
		return nil
	}

	return f.Interface()
}

// 判断结构体中是否包含指定类型的字段
func HasField(s interface{}, fieldType reflect.Type) bool {
	structType := reflect.TypeOf(s)
	if structType.Kind() == reflect.Ptr {
		structType = structType.Elem()
	}
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)

		if field.Type == fieldType {
			return true
		}
	}
	return false
}

// 抽干 c channel的内容
func DryChan[T any](c chan T) {
	for {
		select {
		case <-c:
		default:
			return
		}
	}
}

func DryChanWithCb[T any](c chan T, cb func(T)) {
	for {
		select {
		case m := <-c:
			cb(m)
		default:
			return
		}
	}
}
