// 工具函数 适合所有策略的工具函数放在这里
package helper

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"net"
	"os"
	"regexp"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/shopspring/decimal"

	"actor/config"
	"actor/helper/helper_ding"
	"actor/third/cmap"
	"actor/third/fixed"
	"actor/third/log"
	"actor/tools"
	"github.com/go-redis/redis/v8"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/valyala/fastjson/fastfloat"
)

/*------------------------------------------------------------------------------------------------------------------*/
var ipRegExp *regexp.Regexp

func init() {
	var ipRegex string = `\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`
	var err error
	ipRegExp, err = regexp.Compile(ipRegex)
	if err != nil {
		panic("正则表达式编译失败:")
	}
}

// string转换为byte 性能很高
func StringToBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(
		&struct {
			string
			Cap int
		}{s, len(s)},
	))
}

// BytesToString与pool一起时，如果相关的变量会被return返回等类似异步场，则不能用BytesToString。因为pool内存会被回收，发生变化
// 局部变量 直接用此方法 速度快
// 如果变量涉及到异步使用 要用string()强制转换 防止内存被修改
func BytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func BytesToFloat64(bytes []byte) float64 {
	return fastfloat.ParseBestEffort(BytesToString(bytes))
}

func BytesToInt64(bytes []byte) int64 {
	return fastfloat.ParseInt64BestEffort(BytesToString(bytes))
}

/*------------------------------------------------------------------------------------------------------------------*/

// 部分交易所返回精度为int位小数 转换为 ticksize stepsize 需要处理
// 3 => 0.001  5 => 0.00001
func ConvIntToFixed(n int64) float64 {
	switch n {
	case 0:
		return 1.0
	case 1:
		return 0.1
	case 2:
		return 0.01
	case 3:
		return 0.001
	case 4:
		return 0.0001
	case 5:
		return 0.00001
	case 6:
		return 0.000001
	case 7:
		return 0.0000001
	case 8:
		return 0.00000001
	case 9:
		return 0.000000001
	case 10:
		return 0.0000000001
	case 11:
		return 0.00000000001
	case 12:
		return 0.000000000001
	default:
		return math.Pow(0.1, float64(n))
	}
}

/*------------------------------------------------------------------------------------------------------------------*/

// FixAmount 按照精度调整数量 往下取整step
// 数量按精度调整只能往下不能往上
func FixAmount(amount fixed.Fixed, stepsize float64) fixed.Fixed {
	// fmt.Println(amount, amount.Float(), amount.Float()/stepsize)
	// t := int64(amount.Float() / stepsize)
	// a := fixed.NewF(stepsize).Mul(fixed.NewI(t, 0))
	// fmt.Println(stepsize, fixed.NewF(stepsize))
	// fmt.Println(t, fixed.NewI(t, 0))
	// fmt.Println(a.Float())

	ss := fixed.NewF(stepsize)
	t := amount.Div(ss).Int64()
	a := ss.Mul(fixed.NewI(t, 0))
	return a
}

// FixPrice 按照精度调整价格 就近取整step
// 价格只需要用float64 不需要和策略缓存保持严格一致
func FixPrice(price float64, ticksize float64) fixed.Fixed {
	tsize := fixed.NewF(ticksize)
	tf := fixed.NewF(price / ticksize).Int64()
	t := fixed.NewI(tf, 0)
	a1 := tsize.Mul(t)
	a2 := tsize.Mul(t.Add(fixed.NewI(1, 0)))
	d1 := a1.Sub(fixed.NewF(price)).Abs()
	d2 := a2.Sub(fixed.NewF(price)).Abs()
	var a fixed.Fixed
	if d1.GreaterThanOrEqual(d2) {
		a = a2
	} else {
		a = a1
	}
	//fmt.Println(t.Float(), a)
	return a
}

// FixPriceWithPrecision 按照精度调整价格 就近取整step
func FixPriceWithPrecision(price float64, ticksize float64, digits int) fixed.Fixed {
	// 先按 ticksize 修正价格
	a := FixPrice(price, ticksize)

	// 再按有效数字修正
	x := a.Float()
	if x == 0.0 {
		return a
	}
	d := math.Ceil(math.Log10(math.Abs(x)))
	pow := digits - int(d)
	factor := math.Pow10(pow)
	x = math.Round(x*float64(factor)) / float64(factor)

	return fixed.NewF(x)
}

/*------------------------------------------------------------------------------------------------------------------*/
// PassTime 统计延迟情况
type PassTime struct {
	sync.RWMutex
	Max    int64
	Min    int64
	PassUs []int64
	Sum    int64
	Avg    int64
}

func (p *PassTime) Reset() {
	p.Lock()
	if p.PassUs == nil {
		p.PassUs = make([]int64, 0, 9)
	} else {
		p.PassUs = p.PassUs[:0]
	}
	p.Max = 0
	p.Min = math.MaxInt64
	p.Unlock()
}

func (p *PassTime) UpdateSince(startUs int64) {
	if startUs == 0 {
		return
	}
	if DEBUGMODE && MustMicros(startUs) {
	}
	p.Update(time.Now().UnixMicro(), startUs)
}
func (p *PassTime) Update(nowUs int64, startUs int64) {
	if startUs == 0 || nowUs == 0 {
		return
	}
	if DEBUGMODE && MustMicros(startUs) && MustMicros(nowUs) {
	}
	passUs := nowUs - startUs
	p.Lock()
	defer p.Unlock()
	p.PassUs = append(p.PassUs, passUs) // opt 不应不断新增
	p.Sum += passUs

	if len(p.PassUs) > 9 {
		p.Sum -= p.PassUs[0]
		p.PassUs = p.PassUs[1:] // opt 不应不断新增
	}
	if p.Max < passUs {
		p.Max = passUs
	}
	if p.Min > passUs {
		p.Min = passUs
	}
	lenPass := len(p.PassUs)
	if lenPass == 0 {
		p.Avg = 0
	} else {
		p.Avg = p.Sum / int64(lenPass)
	}
}

func (p *PassTime) String() string {
	p.RLock()
	defer p.RUnlock()
	sum, max, min := int64(0), int64(0), int64(math.MaxInt64)

	for _, v := range p.PassUs {
		sum += v
		if max < v {
			max = v
		}
		if min > v {
			min = v
		}
	}
	num := int64(len(p.PassUs))
	var ave int64
	if num == 0 {
		ave = int64(0)
	} else {
		ave = sum / num
	}
	return fmt.Sprintf("(ms): max:%d, min:%d | max:%d, min:%d, ave:%d",
		p.Max, p.Min, max, min, ave)
}

func (p *PassTime) GetDelay() int64 {
	p.RLock()
	defer p.RUnlock()
	return p.Avg
}

/*------------------------------------------------------------------------------------------------------------------*/

// Timeit 耗时测试工具函数
type Timeit struct {
	st time.Time
	ed time.Time
}

func (t *Timeit) Start() {
	t.st = time.Now()
}

func (t *Timeit) End() int64 {
	t.ed = time.Now()
	delay := t.ed.Sub(t.st).Nanoseconds()
	delay_us := t.ed.Sub(t.st).Microseconds()
	fmt.Println(delay, " ns  ", delay_us, " us  ")
	return delay
}

/*------------------------------------------------------------------------------------------------------------------*/

// GetRandLetter 获取随机字母 todo 此处速度可以优化
func GetRandLetter() string {
	var s string
	var temp byte
	var result bytes.Buffer
	temp = byte(65 + rand.Intn(91-65))
	result.WriteByte(temp)
	s = result.String()
	return s
}

// GetRandNum 获取随机数字 todo 此处速度可以优化
func GetRandNum() string {
	return ""
}

// 解压缩函数
func GZipDecompress(input []byte) ([]byte, error) {
	buf := bytes.NewBuffer(input)
	reader, gzipErr := gzip.NewReader(buf)
	if gzipErr != nil {
		return nil, gzipErr
	}
	defer reader.Close()

	result, readErr := ioutil.ReadAll(reader)
	if readErr != nil {
		return nil, readErr
	}

	return result, nil
}

// 获取文件修改时间 返回unix时间戳
func GetFileModTime(path string) int64 {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return 0
	}

	return fi.ModTime().Unix()
}

/* -------------------------------------------------------------------------------- */

// AesEncrypt 加密函数
func AesEncrypt(orig string, key string) string {
	// 转成字节数组
	origData := []byte(orig)
	k := []byte(key)
	// 分组秘钥
	// NewCipher该函数限制了输入k的长度必须为16, 24或者32
	block, _ := aes.NewCipher(k)
	// 获取秘钥块的长度
	blockSize := block.BlockSize()
	// 补全码
	origData = PKCS7Padding(origData, blockSize)
	// 加密模式
	blockMode := cipher.NewCBCEncrypter(block, k[:blockSize])
	// 创建数组
	cryted := make([]byte, len(origData))
	// 加密
	blockMode.CryptBlocks(cryted, origData)
	return base64.StdEncoding.EncodeToString(cryted)
}

// AesDecrypt  解密函数
func AesDecrypt(cryted string, key string) string {

	// 转成字节数组
	crytedByte, _ := base64.StdEncoding.DecodeString(cryted)
	k := []byte(key)
	// 分组秘钥
	block, _ := aes.NewCipher(k)
	// 获取秘钥块的长度
	blockSize := block.BlockSize()
	// 加密模式
	blockMode := cipher.NewCBCDecrypter(block, k[:blockSize])
	// 创建数组
	orig := make([]byte, len(crytedByte))
	// 解密
	blockMode.CryptBlocks(orig, crytedByte)
	// 去补全码
	orig = PKCS7UnPadding(orig)
	return string(orig)
}

// PKCS7Padding 补码AES加密数据块分组长度必须为128bit(byte[16])，密钥长度可以是128bit(byte[16])、192bit(byte[24])、256bit(byte[32])中的任意一个。
func PKCS7Padding(ciphertext []byte, blocksize int) []byte {
	padding := blocksize - len(ciphertext)%blocksize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

// PKCS7UnPadding 去码
func PKCS7UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

/* -------------------------------------------------------------------------------- */

// MemOptimize 内存优化方案 策略启动时候调用 仅限策略程序 同一机器下禁止多个程序使用此内存优化方案
func MemOptimize() {

	// 获取内存 设置gc规则
	v, _ := mem.VirtualMemory()
	fmt.Printf("[总内存] %v mb  [当前可用] %v mb  [使用比例] %.4f%%\n", v.Total/1e6, v.Available/1e6, v.UsedPercent)
	// 计算触发gc的上限 最小100mb 最大16gb
	mem_thre := int64(v.Available - 150e6)
	if mem_thre < 100e6 {
		fmt.Println("内存过小 只能使用自动gc")
		return
	}
	if mem_thre > 16e9 {
		// 关闭gc
		debug.SetGCPercent(-1)
		mem_thre = 16e9
		fmt.Printf("本程序内存上限为 %v mb", mem_thre/1e6)
		debug.SetMemoryLimit(mem_thre)
		return
	}
}

/* -------------------------------------------------------------------------------- */

// 只有所有盘口涉及该品种的交易对精度都不高才允许启动
const canFastList = "" +
	"btc|eth|" +
	"lina|tomo|mask|arpa|edu|arb|xrp|sol|ltc|cfx|inj|sand|bnb|doge|trx|phb|mana|alpha|near|gmt|dydx|crv|ape|bch|waves|grt|" +
	"ada|sui|matic|gala|agix|rndr|ldo|eos|neo|high|ftm|avax|apt|id|ldo|op|bel|link|fil|dot|fet|key|atom|magic|etc|kava|axs|" +
	"apt|stx|mtl|ach|sushi|chz|ocean|omg|nkn|xem|rlc|stg|imx|blur|vet|celo|ont|sxp|xmr|algo|ctsi|rdnt|aave|snx|stmx|hook|icp|" +
	"woo|uni|theta|tru|bnb|rune|celr|joe|knc|qtum|zec|zil|ckb|rose|jasmy|lqty|iota|ant|flow|gal|lit|ssv|mkr|lpt|1inch|xlm|" +
	"gmx|audio|xtz|yfi|dent|hft|ankr|c98|amb|gtc|trb|flm|unfi|comp|coti|trx|band|football|chr|mina|matic|alice|rad|iost|ens|" +
	"ren|ogn|klay|lrc|rsr|zrx|api3|ksm|rvn|ar|dgb|dusk|hot|bat|fxs|bal|bat|idex|qnt|one|bnx|reef|storj|ata|astr|skl|cvx|dar|" +
	"bake|uma|iotx|zen|sfp|tlm|ctk|perp|defi"

// 自动判断是否允许使用快速高精度计算库
// CheckCanUseFastDecimal 此选项仅取决于交易对 按白名单机制执行
func CheckCanUseFastDecimal(pair string) {
	for _, v := range strings.Split(canFastList, "|") {
		pair_, err := StringPairToPair(pair)
		if err == nil {
			if strings.ToLower(pair_.Base) == v {
				fixed.TurnOnFastDecimal()
				fmt.Println("自动启用快速高精度")
				log.Errorf("自动启用快速高精度")
				break
			}
		}
	}
}

/* -------------------------------------------------------------------------------- */

func GetSortedStringBytesOfMap(params map[string]interface{}) []byte {
	cnt := len(params)
	keys := make([]string, 0, cnt)
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	for i, k := range keys {
		v := params[k]
		buf.WriteString(k)
		buf.WriteString("=")
		switch v.(type) {
		case map[string]interface{}, []interface{}, []map[string]interface{}:
			m, err := json.Marshal(v)
			if err != nil {
				log.Errorf("GetSortedStringBytesOfMap marshal err: %v, params: %v", err, params)
				return nil
			}
			_, err = buf.Write(m)
			if err != nil {
				log.Errorf("GetSortedStringBytesOfMap writer err: %v, params: %v", err, params)
				return nil
			}
		default:
			buf.WriteString(fmt.Sprintf("%v", v))
		}
		if i != cnt-1 {
			buf.WriteString("&")
		}
	}
	return buf.Bytes()
}

func GetMinFromSlice(d []float64) (int, float64) {
	if len(d) == 0 {
		return 0, 0
	} else {
		Index := 0
		Value := d[0]
		for k, v := range d {
			if v < Value {
				Value = v
				Index = k
			}
		}
		return Index, Value
	}
}

func GetMaxFromSlice(d []float64) (int, float64) {
	if len(d) == 0 {
		return 0, 0
	} else {
		Index := 0
		Value := d[0]
		for k, v := range d {
			if v > Value {
				Value = v
				Index = k
			}
		}
		return Index, Value
	}
}

func FindMinIndex(nums []int64) int {
	if len(nums) == 0 {
		return -1 // 如果切片为空，则返回 -1
	}

	minIndex := 0              // 初始化最小元素所在位置为第一个元素的位置
	minValue := nums[minIndex] // 初始化最小值为第一个元素的值

	// 遍历切片并比较每个元素与当前最小值的大小
	for i, num := range nums {
		if num < minValue {
			minIndex = i   // 更新最小元素所在位置
			minValue = num // 更新最小值
		}
	}

	return minIndex // 返回最小元素所在位置
}

func CleanChan[T string](c <-chan T) {
	for {
		select {
		case <-c:
		default:
			return
		}
	}
}

func CopySymbolInfo(dst, src *ExchangeInfo) {
	dst.Pair = src.Pair
	dst.Symbol = src.Symbol
	dst.TickSize = src.TickSize
	dst.StepSize = src.StepSize
	dst.ContractSize = src.ContractSize
	dst.MaxOrderValue = src.MaxOrderValue
	dst.MaxOrderAmount = src.MaxOrderAmount
	dst.MaxLimitOrderValue = src.MaxLimitOrderValue
	dst.MaxLimitOrderAmount = src.MaxLimitOrderAmount
	dst.MinOrderValue = src.MinOrderValue
	dst.MinOrderAmount = src.MinOrderAmount
	dst.MaxLeverage = src.MaxLeverage
	dst.Multi = src.Multi
	dst.Status = src.Status
	dst.StarkExBaseAssetId = src.StarkExBaseAssetId
	dst.StarkExQuoteAssetId = src.StarkExQuoteAssetId
	dst.MaxPosAmount = src.MaxPosAmount
	dst.MaxPosValue = src.MaxPosValue
	dst.RiskLimit = src.RiskLimit
	dst.MaxLeverage = src.MaxLeverage
	dst.Extra = src.Extra
	dst.Extras = src.Extras

	dst.StarkExBaseAssetId = src.StarkExBaseAssetId
	dst.StarkExQuoteAssetId = src.StarkExQuoteAssetId
	dst.StarkExBaseResolution = src.StarkExBaseResolution
	dst.StarkExQuoteResolution = src.StarkExQuoteResolution
}

/* -------------------------------------------------------------------------------- */
// 读取fileName信息，填充S2P(Symbol To Pair) P2S。
// 返回 pair对应的ExchangeInfo、全体ExchangeInfo、是否成功(一个失败都为失败)

/**
1. 读取本地json，json过期则从redis读取；
2. redis不过期，读取返回。redis过期，则尝试获取锁，
    2.1 如果锁空闲或者没有或被占用超过30秒，尝试上锁，上锁成功远程拉取info更新。
    2.2 不空闲，等待10秒，重复2
重试上述步骤10次，直到成功，写入本地json；最终失败则异常停机。
*/

// 判断是否是外部环境（Windows, macOS, 或 WSL）
func IsOuterEnv() bool {
	goos := runtime.GOOS

	// 如果是 Windows 或 macOS
	if goos == "windows" || goos == "darwin" {
		return true
	}

	// 检查 WSL 特有的环境变量
	if _, exists := os.LookupEnv("WSL_DISTRO_NAME"); exists {
		return true
	}
	if _, exists := os.LookupEnv("WSL_INTEROP"); exists {
		return true
	}

	// 检查 /proc/version 文件内容是否包含 "Microsoft"
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		log.Infof("failed to read /proc/version: %v", err)
		return false
	}
	if strings.Contains(string(data), "WSL") || strings.Contains(string(data), "microsoft") {
		return true
	}

	return false
}

func doGetExchangeInfos(fileName string, fetchInfo func(string) ([]ExchangeInfo, error)) ([]ExchangeInfo, error) {
	return nil, nil
	if IsOuterEnv() {
		log.Infof("Detected outer environment, skipping Redis and fetching ExchangeInfo directly.")
		return fetchInfo(fileName)
	}

	modTs := GetFileModTime(fileName)
	if time.Now().Unix()-modTs < 300 {
		log.Warnf("不需要更新 ExchangeInfo 从文件加载...")
		line, err := os.ReadFile(fileName)
		if err != nil {
			log.Errorf("读取文件失败 需要更新 ExchangeInfo")
			return nil, err
		}
		var result []ExchangeInfo
		err = json.Unmarshal(line, &result)
		if err != nil {
			log.Errorf("解析文件失败 需要更新 ExchangeInfo")
			return nil, err
		}
		return result, nil
	}
	log.Warnf("本地ExchangeInfo文件过久，需要更新或从redis获取. %s", fileName)

	// 创建 Redis 客户端
	// client := redis.NewClusterClient(&redis.ClusterOptions{
	// Addrs:    []string{"ave.gvneuq.clustercfg.memorydb.ap-northeast-1.amazonaws.com:6379", "172.17.15.225:6379"},
	// Password: "",
	// })

	client := redis.NewClient(&redis.Options{
		Addr:     config.REDIS_ADDR,
		Password: config.REDIS_PWD,
		// 仅适用exchangeInfo场景，其他不一定适合
		PoolSize:     2,
		MinIdleConns: 1,
		DialTimeout:  time.Second * 2,
		ReadTimeout:  time.Second * 2,
		WriteTimeout: time.Second * 2,
	})

	defer client.Close()

	lockKey := fileName + "_lock"
	contentKey := fmt.Sprintf("%s_%s_%s", fileName, AppName, BuildTime)

	for tried := 0; tried < 3; tried++ {

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		// 使用客户端执行 GET 命令读取数据
		val, err := client.Get(ctx, contentKey).Result()
		if err == nil {
			var result []ExchangeInfo
			err = json.Unmarshal([]byte(val), &result)
			if err == nil {
				log.Infof("succ get exchange info from redis")
				StoreExchangeInfos(fileName, result)
				return result, nil
			}
			log.Errorf("[utf_ign]解析redis返回内容失败 需要更新 ExchangeInfo. %s", fileName)
		} else if err == redis.Nil || strings.Index(err.Error(), "MOVED") >= 0 {
			// key not exist
		} else {
			// 除了 key not exist 的其他错误
			log.Errorf("[utf_ign]redis 读取失败. %s, %v", contentKey, err)
			time.Sleep(3 * time.Second)
			continue
		}

		startTime := time.Now().Unix()
		{
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			ok, err := client.SetNX(ctx, lockKey, 1, 30*time.Second).Result()
			if err != nil || !ok {
				log.Warnf("redis上锁失败 %v %v", ok, err)
				time.Sleep(3 * time.Second)
				continue
			}
		}

		infos, err := fetchInfo(fileName)
		ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel2()
		if err != nil || len(infos) == 0 {
			log.Errorf("fetchInfo failed. %v, %v", len(infos), err)
			client.Del(ctx2, lockKey).Result()
			time.Sleep(3 * time.Second)
			continue
		}
		jsonData, err := json.Marshal(infos)
		if err != nil {
			log.Errorf("json.Marshal err: %v", err)
			client.Del(ctx2, lockKey).Result()
			time.Sleep(3 * time.Second)
			continue
		}
		if time.Now().Unix()-startTime > 30 {
			log.Warnf("更新成功，但超过30s,  %d", time.Now().Unix()-startTime)
		}
		if err := os.WriteFile(fileName, jsonData, 0644); err != nil {
			log.Errorf("failed to store exchange info file %v", err.Error())
		}
		val, err = client.Set(ctx2, contentKey, jsonData, 5*time.Minute).Result()
		if err != nil {
			log.Errorf("写入 local file 数据出错: %v", err)
		}
		client.Del(ctx2, lockKey).Result()
		return infos, nil
	}
	return nil, errors.New("未知错误")
}
func StoreExchangeInfos(fileName string, infos []ExchangeInfo) error {
	jsonData, err := json.Marshal(infos)
	if err != nil {
		log.Errorf("json.Marshal err: %v", err)
		return err
	}
	err = os.WriteFile(fileName, jsonData, 0644)
	if err != nil {
		log.Errorf("failed to store exchangeinfos to local file: %v", err)
		return err
	}
	return nil
}

func SaveStringToFile(fileName string, content []byte) {
	// 将数据写入文件
	log.Debugf("write to file %v", fileName)
	err := os.WriteFile(fileName, content, 0644)
	if err != nil {
		log.Errorf("写文件错误: %v", err)
	}
}

func TryGetExchangeInfosFromFileAndRedis(fileName string, pair Pair, exchangeInfoS2P, exchangeInfoP2S cmap.ConcurrentMap[string, *ExchangeInfo], fetchInfo func(fileName string) ([]ExchangeInfo, error)) (pairInfo ExchangeInfo, infos []ExchangeInfo, ok bool) {
	result, err := doGetExchangeInfos(fileName, fetchInfo)
	var res ExchangeInfo
	if err != nil {
		return res, result, false
	}
	for _, infoT := range result {
		var info ExchangeInfo
		info = infoT
		exchangeInfoS2P.Set(info.Symbol, &info)
		exchangeInfoP2S.Set(info.Pair.String(), &info)
		if info.Pair.Equal(pair) {
			res = info
		}
	}
	if res.Symbol == "" {
		log.Errorf("没找到交易对信息 %s 需要更新 ExchangeInfo. %s", pair, fileName)
		return res, result, false
	}
	return res, result, true
}
func GetFileSlotForReqExchangeInfo(exname string) string {
	f := doGetFileSlotForReqExchangeInfo(exname)
	tried := 0
	for ; f == ""; tried++ {
		if tried > 10 {
			log.Error("finaly failed to request free file slot")
			return ""
		}
		time.Sleep(time.Second * 3)
		f = doGetFileSlotForReqExchangeInfo(exname)
	}
	return f
}
func doGetFileSlotForReqExchangeInfo(exname string) string {
	basePath := fmt.Sprintf("/tmp/bqrs.exchange/%s", exname)
	f := fmt.Sprintf("%s/%d", basePath, time.Now().UnixNano())
	_, err := os.Stat(basePath)
	if os.IsNotExist(err) {
		err = os.MkdirAll(basePath, 0755)
		if err != nil {
			log.Errorf("failed to create temp folder: [%v]\n", err)
			return ""
		}
	}
	files, err := ioutil.ReadDir(basePath)
	if err != nil {
		log.Error(err)
		return ""
	}
	if len(files) > 5 {
		log.Error("perhaps failed to clean file slot. ", basePath, " ", len(files))
	}
	l := 0
	for _, fi := range files {
		if fi.ModTime().Add(time.Minute*5).Compare(time.Now()) == 1 {
			l++
		}
	}
	if l > 2 {
		return ""
	}
	_, err = os.Create(f)
	if err != nil {
		log.Errorf("failed to create tmp file: [%v]\n", err)
		return ""
	}
	return f
}

// todo remove
// func TryGetExchangeInfosFromFile(fileName string, pair Pair, exchangeInfoS2P map[string]ExchangeInfo, exchangeInfoP2S map[string]ExchangeInfo) (pairInfo ExchangeInfo, infos []ExchangeInfo, ok bool) {
// 	pairInfo, infos, ok = TryGetExchangeInfosPtrFromFile(fileName, pair, exchangeInfoS2P, p2s)
// 	for key, v := range s2p {
// 		exchangeInfoS2P[key] = *v
// 	}
// 	for key, v := range p2s {
// 		exchangeInfoP2S[key] = *v
// 	}
// 	return
// }

// todo rename after all refactored
func TryGetExchangeInfosPtrFromFile(fileName string, pair Pair, exchangeInfoS2P, exchangeInfoP2S cmap.ConcurrentMap[string, *ExchangeInfo]) (pairInfo ExchangeInfo, infos []ExchangeInfo, ok bool) {
	modTs := GetFileModTime(fileName)
	if time.Now().Unix()-modTs > 300 {
		log.Warnf("需要更新 ExchangeInfo %s", fileName)
		return ExchangeInfo{}, nil, false
	} else {
		log.Warnf("不需要更新 ExchangeInfo 从文件加载..., %s", fileName)

		var result []ExchangeInfo
		line, err := os.ReadFile(fileName)
		if err != nil {
			log.Errorf("[utf_ign]读取文件失败 需要更新 ExchangeInfo. %s", fileName)
			return ExchangeInfo{}, nil, false
		}
		err = json.Unmarshal(line, &result)
		if err != nil {
			log.Errorf("[utf_ign]解析文件失败 需要更新 ExchangeInfo, %s", fileName)
			return ExchangeInfo{}, nil, false
		}
		// 更新主交易对交易规则信息
		var res ExchangeInfo
		for i := range result {
			info := result[i]
			exchangeInfoS2P.Set(info.Symbol, &info)
			exchangeInfoP2S.Set(info.Pair.String(), &info)
			if info.Pair.Equal(pair) {
				res = info
			}
		}
		if res.Symbol == "" {
			log.Errorf("没找到交易对信息 %s 需要更新 ExchangeInfo", pair)
			return res, result, false
		}
		return res, result, true
	}
}

func Trim10Multiple(baseCoin string) string {
	re := regexp.MustCompile("^10{2,7}")
	baseCoin = re.ReplaceAllString(baseCoin, "")
	re = regexp.MustCompile("10{2,7}$")
	baseCoin = re.ReplaceAllString(baseCoin, "")
	return baseCoin
}

// 根据 isLongSide 确保返回正负值
func EnsureSignedAmount(amount float64, isLongSide bool) float64 {
	if isLongSide {
		return math.Abs(amount)
	} else {
		return -math.Abs(amount)
	}
}

// skip 1是获取自己，2是获取调用者，3逐级向上
func GetCallerInfo(skip int) (fileName, funcName string, lineNo int) {

	// pc, file, lineNo
	pc, file, lineNo, ok := runtime.Caller(skip)
	if !ok {
		// panic("can't get caller info")
		helper_ding.DingingSendWarning("can't get caller info")
	}
	vals := strings.Split(runtime.FuncForPC(pc).Name(), ".")
	funcName = vals[len(vals)-1]
	fs := strings.Split(file, "/")
	if len(fs) >= 2 {
		file = fs[len(fs)-2] + "/" + fs[len(fs)-1]
	}
	return file, funcName, lineNo
	// return fmt.Sprintf("FuncName:%s, line:%d", funcName, lineNo)
	// fileName := path.Base(file) // Base函数返回路径的最后一个元素
	// return fmt.Sprintf("FuncName:%s, file:%s, line:%d ", funcName, fileName, lineNo)
}
func LogErrorThenCall(msg string, callee func(msg string)) {
	file, funcName, lineNo := GetCallerInfo(2)
	log.Errorf("[%s:%s:%d] %s", file, funcName, lineNo, msg)
	callee(msg)
}
func LogWarnThenCall(msg string, callee func(msg string)) {
	file, funcName, lineNo := GetCallerInfo(2)
	log.Errorf("[%s:%s:%d] %s", file, funcName, lineNo, msg)
	callee(msg)
}

/* -------------------------------------------------------------------------------- */
// BoolToString 简单但是性能很高
func BoolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func shouldIgnoreIpQuery(host string) bool {
	// 正则表达式判断host若是ip，忽略
	if ipRegExp.Match(StringToBytes(host)) {
		return true
	}
	switch host {
	// 通过系统hosts文件绑定ip的，忽略
	case "api.kucoin.com", "openapi-v2.kucoin.com", "api-futures.kucoin.com", //
		"push-private.kucoin.com", "push-private.futures.kucoin.com", "ws-api.kucoin.com", //
		"ws-api-spot.kucoin.com", "ws-api-futures.kucoin.com":
		return true
	case "www.okx.com", "ws.okx.com", "okx.com":
		return true
	}

	return false

}

// 创建本地文件表明要查询
func TouchIpToQuery(addr string) bool {
	return false // 禁用，addr出现未定义格式 2024-4-24
	vals := strings.Split(addr, ":")
	if shouldIgnoreIpQuery(vals[0]) {
		return false
	}
	port, err := strconv.ParseInt(vals[1], 10, 64)
	if err != nil {
		switch vals[1] {
		case "http", "ws":
			port = 80
		case "https", "wss":
			port = 443
		}
	}
	if port == 0 {
		log.Error("wrong addr", addr)
		return false
	}
	// 如果./lion不存在，则创建
	err = os.Mkdir("./lion", 0755)

	addr = fmt.Sprintf("%s:%d", vals[0], port)
	fileName := "./lion/" + addr

	_, err = os.Stat(fileName)
	if err != nil {
		if os.IsNotExist(err) {
			_, err := os.Create(fileName)
			if err != nil {
				log.Error("create failed:", err)
				return false
			}
			return true
		}
		log.Error("检查文件状态失败:", err)
		return false
	}

	// 文件存在，更新访问和修改时间为当前时间
	err = os.Chtimes(fileName, time.Now(), time.Time{})
	if err != nil {
		log.Error("更新文件时间失败:", err)
		return false
	}
	return true

}

// 选择ip进行链接。
//
// @params
//
//	addr: host+port组合, port不可省略, 可以是数字和协议。 例如 fapi.binance.com:443
//	excludeIpsAsPossible: 尽量排除这些ip, 黑名单； nil时表示使用白名单 fastest ip
//
// @return:	ip port
func SelectIp(addr string, excludeIpsAsPossible []string) (string, int) {
	if excludeIpsAsPossible == nil {
		return GetFastIp(addr)
	}
	vals := strings.Split(addr, ":")
	host := vals[0]
	port, err := strconv.ParseInt(vals[1], 10, 64)
	if err != nil {
		switch vals[1] {
		case "http", "ws":
			port = 80
			addr = fmt.Sprintf("%s:%s", vals[0], "80")
		case "https", "wss":
			port = 443
		}
	}
	if port == 0 {
		log.Error("wrong addr", addr)
		return "", 0
	}

	// 创建上下文对象，并设置超时时间为 5 秒
	ctx3, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 创建 Resolver 对象
	resolver := net.DefaultResolver

	// 在指定的上下文中执行 LookupIP
	ipList, err := resolver.LookupIP(ctx3, "ip4", host)

	if err != nil {
		log.Error("解析 IP 地址出错:", err, addr)
	} else {
		for _, addr := range ipList {
			if ipv4 := addr.To4(); ipv4 != nil {
				if !tools.Contains[string](excludeIpsAsPossible, ipv4.String()) {
					return ipv4.String(), int(port)
				}
			}
		}
		// 全部在exclude里面，随机选一个
		idx := rand.Intn(len(ipList))
		return ipList[idx].String(), int(port)
	}

	return "", 0
}

// @params addr host+port组合, port不可省略, 可以是数字和协议。 例如 fapi.binance.com:443
// @return ip port
func GetFastIp(addr string) (string, int) {
	vals := strings.Split(addr, ":")
	port, err := strconv.ParseInt(vals[1], 10, 64)
	if err != nil {
		switch vals[1] {
		case "http", "ws":
			port = 80
			addr = fmt.Sprintf("%s:%s", vals[0], "80")
		case "https", "wss":
			port = 443
		}
	}
	if port == 0 {
		log.Error("wrong addr", addr)
		return "", 0
	}
	addr = fmt.Sprintf("%s:%d", vals[0], port)
	fileName := "./lion/" + addr
	content, err := os.ReadFile(fileName)
	if err != nil {
		log.Error("[utf_ign] read failed", err)
		return "", 0
	}
	// 方案1 唯一ip
	// vals = strings.Split(BytesToString(content), ";")
	// if BytesToString(content) == "" || len(vals) < 3 {
	// 	return "", 0
	// }
	// lastUpdateTimeSec, err := strconv.ParseInt(vals[2], 10, 64)
	// if err != nil {
	// 	log.Error("[utf_ign] parse failed", content, err)
	// 	return "", 0
	// }
	// if lastUpdateTimeSec+24*60*60 < time.Now().Unix() {
	// 	log.Error("too long not updated", addr)
	// 	return "", 0
	// }

	// connectTime, err := strconv.ParseInt(vals[1], 10, 64)
	// if err != nil {
	// 	log.Error("[utf_ign] parse failed", content, err)
	// 	return "", 0
	// }
	// if connectTime >= 999_999 {
	// 	log.Error("[utf_ign] wrong content", content)
	// 	return "", 0
	// }
	// return vals[0], int(port)

	// 方案2 随机ip
	if BytesToString(content) == "" {
		return "", 0
	}
	vals = strings.Split(BytesToString(content), ";")

	// todo 为了兼容旧版，dino 升级完后，去除这个判断。2023-11-9写入，预计2023-12-9可以去除
	if strings.LastIndex(vals[len(vals)-1], ":") >= 0 {
		return vals[0], int(port)
	}

	return vals[rand.Intn(len(vals))], int(port)
}

/* -------------------------------------------------------------------------------- */
// 带有前后空格的字符串切片 去掉空格
func TrimStringSlice(ss []string) []string {
	for i, str := range ss {
		ss[i] = strings.TrimSpace(str)
	}
	return ss
}

// 判断ts是否毫秒时间戳，会panic，慎用，一般只在utf用
func MustSecs(ts int64) bool {
	const _START = 1702298857 // 2023-12-11 20:47:37 utc8
	const _END = 2017918057   // 2033-12-11 20:47:37 utc8. 10年后有彩蛋
	if ts < _START || ts > _END {
		log.Error("not secs. ts: ", ts)
		PrintStackstrace(log.RootLogger)
		panic("not secs. ")
	}
	return true
}

// 判断ts是否毫秒时间戳，会panic，慎用，一般只在utf用
func MustMillis(ts int64) bool {
	const _START = 1702298857000 // 2023-12-11 20:47:37 utc8
	const _END = 2017918057000   // 2033-12-11 20:47:37 utc8. 10年后有彩蛋
	if ts < _START || ts > _END {
		log.Error("not millis. ts: ", ts)
		PrintStackstrace(log.RootLogger)
		panic("not millis. ")
	}
	return true
}

// 判断ts是否微秒时间戳，会panic，慎用，一般只在utf用
func MustMicros(ts int64) bool {
	const _START = 1702298857000000 // 2023-12-11 20:47:37 utc8
	const _END = 2017918057000000   // 2033-12-11 20:47:37 utc8. 10年后有彩蛋
	if ts < _START || ts > _END {
		log.Error("not micros. ts: ", ts)
		PrintStackstrace(log.RootLogger)
		panic("not micros. ")
	}
	return true
}

// 判断ts是否纳秒秒时间戳，会panic，慎用，一般只在utf用
func MustNanos(ts int64) bool {
	const _START = 1702298857000000000 // 2023-12-11 20:47:37 utc8
	const _END = 2017918057000000000   // 2033-12-11 20:47:37 utc8. 10年后有彩蛋
	if ts < _START || ts > _END {
		log.Error("not nanos. ts: ", ts)
		PrintStackstrace(log.RootLogger)
		panic("not nanos. ")
	}
	return true
}

/* -------------------------------------------------------------------------------- */
// 返回 >=x且能整除y的最小值
func Ceil(x, y int64) int64 {
	return int64(math.Ceil(float64(x)/float64(y))) * y
}

func SuitableLevel(levels []int, target int) (res int, ok bool) {
	return tools.FirstMatch[int](levels, target, func(item, target int) bool { return item >= target })
}

/* -------------------------------------------------------------------------------- */

// 计算两个数的最大公约数
func gcd(a, b fixed.Fixed) fixed.Fixed {
	if b.Equal(fixed.ZERO) {
		return a
	}
	a1 := decimal.NewFromFloat(a.Float())
	b1 := decimal.NewFromFloat(b.Float())
	tmp := a1.Mod(b1)
	f, _ := tmp.Float64()
	return gcd(b, fixed.NewF(f))
}

// 计算两个数的最小公倍数
// 一个典型使用场景 盘口A B 找到一个同时满足两个盘口下单精度要求的最小精度
func Lcm(a, b fixed.Fixed) fixed.Fixed {
	return a.Mul(b).Div(gcd(a, b))
}

func EnsureWriterUnlocked(unlocked *bool, lock *sync.RWMutex) {
	if *unlocked {
		return
	}
	lock.Unlock()
}

func KeepOnlyNumbers(input string) string {
	re := regexp.MustCompile(`[^\d]`)
	return re.ReplaceAllString(input, "")
}

/* -------------------------------------------------------------------------------- */

// GetFloatSign 计算 float64的符号
func GetFloatSign(x float64) float64 {
	if x > 0 {
		return 1
	} else if x < 0 {
		return -1
	} else {
		return 0
	}
}

// 输入一个string数组，找出最短长度n，获取每个字符串的[0,n)子字符串就能识别确定string数组里面每个字符串
// return 0 表示只有一个，能直接识别; -1 表示不能识别，有重叠
func UniqueIdentifyShortest(arr []string) int {
	shortest := 0
	if arr == nil || len(arr) <= 1 {
		return shortest
	}
	for {
	START:
		shortest++
		existMap := make(map[string]bool)
		for _, item := range arr {
			if shortest > len(item) {
				return -1
			}
			s := item[0:shortest]
			// fmt.Println(s, " ", item, " ", shortest)
			if _, exist := existMap[s]; exist {
				goto START
			} else {
				existMap[s] = true
			}
		}
		return shortest
	}

}

// 不管s是否bytes引用，都复制一次。不可高频调用
func EnsureClone(in string) string {
	s := "XXX_" + string(in)
	return strings.Replace(s, "XXX_", "", 1)
}

func RichString(s interface{}) string {
	b, err := json.MarshalIndent(s, "", " ")
	if err != nil {
		return fmt.Sprintf("failed to marshal bc. %v", err)
	}
	return string(b)
}

// 返回 m bytes单位
func GetRSS() (float64, error) {
	data, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return 0, err
	}

	// fmt.Println("data ", string(data))
	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return 0, fmt.Errorf("Unexpected format in /proc/self/statm")
	}

	pages, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, err
	}
	// fmt.Println("pages ", pages)

	pageSize := uint64(os.Getpagesize())
	rss := float64(pages*pageSize) / 1024.0 / 1024
	return rss, nil
}

func MustGetFloat64FromString(v string) float64 {
	r, err := strconv.ParseFloat(v, 64)
	if err != nil {
		panic(fmt.Sprintf("failed to convert. %s, %v", v, err))
	}
	return r
}
func MustGetFloat64FixedCappedFromString(v string) fixed.Fixed {
	r, err := strconv.ParseFloat(v, 64)
	if err != nil {
		panic(fmt.Sprintf("failed to convert. %s, %v", v, err))
	}
	res := fixed.BIG
	if r <= res.Float() {
		res = fixed.NewF(r)
	}
	return res
}
func MustGetFloat64FixedCapped(r float64) fixed.Fixed {
	res := fixed.BIG
	if r <= res.Float() {
		res = fixed.NewF(r)
	}
	return res
}

func MustGetInt64FromString(v string) int64 {
	r, err := strconv.Atoi(v)
	if err != nil {
		panic(fmt.Sprintf("failed to convert. %s, %v", v, err))
	}
	return int64(r)
}

func MustGetIntFromString(v string) int {
	r, err := strconv.Atoi(v)
	if err != nil {
		panic(fmt.Sprintf("failed to convert. %s, %v", v, err))
	}
	return r
}

func CloseSafe[Type any](c chan<- Type) {
	if c == nil {
		return
	}
	defer func() {
		if err := recover(); err != nil {
			if DEBUGMODE {
				var buf [4096]byte
				n := runtime.Stack(buf[:], false)
				log.Warnf("recover panic in SafeClose() ==> %s\n", string(buf[:n]))
			}
		}
	}()
	close(c)
}

func Itoa[T int64 | int](val T) string {
	return strconv.FormatInt(int64(val), 10)
}

func TsToHumanMillis(tsms int64) int64 {
	if tsms == 0 {
		return 0
	}
	_ = DEBUGMODE && MustMillis(tsms)
	t := time.UnixMilli(tsms)
	year := t.Year()
	month := int(t.Month())
	day := t.Day()
	hour := t.Hour()
	minute := t.Minute()
	second := t.Second()
	millis := tsms % 1000

	var human int64 = millis
	var scale int64 = 1000
	human += int64(second) * scale
	scale *= 100
	human += int64(minute) * scale
	scale *= 100
	human += int64(hour) * scale
	scale *= 100
	human += int64(day) * scale
	scale *= 100
	human += int64(month) * scale
	scale *= 100
	human += int64(year) * scale

	return human
}

func TsToHumanSec(tssec int64) int64 {
	if tssec == 0 {
		return 0
	}
	_ = DEBUGMODE && MustSecs(tssec)
	t := time.Unix(tssec, 0)
	year := t.Year()
	month := int(t.Month())
	day := t.Day()
	hour := t.Hour()
	minute := t.Minute()
	second := t.Second()

	var human int64 = int64(second)
	var scale int64 = 100
	human += int64(minute) * scale
	scale *= 100
	human += int64(hour) * scale
	scale *= 100
	human += int64(day) * scale
	scale *= 100
	human += int64(month) * scale
	scale *= 100
	human += int64(year) * scale

	return human
}

// 性能比int版本慢一倍
func TssecToReadableStr(tssec int64) string {
	t := time.Unix(tssec, 0)
	return t.Format("20060102150405")
}

func PrintStackstrace(logger log.Logger) {

	var buf [4096]byte
	n := runtime.Stack(buf[:], false)
	if logger != nil {
		logger.Errorf("stackstrace ==> %s\n", string(buf[:n]))
	} else {
		fmt.Printf("stackstrace ==> %s\n", string(buf[:n]))
	}
}

// 用法  defer LogPanic(recover())
// func LogPanic(err error) {
// if err := recover(); err != nil {
// 	if len(apiErr) > 0 {
// 		apiErr[0].HandlerError = fmt.Errorf("%v", err)
// 	}
// 	c.logger.Error(err)
// 	var buf [4096]byte
// 	n := runtime.Stack(buf[:], false)
// 	c.logger.Errorf("==> %s\n", string(buf[:n]))
// }
// }
