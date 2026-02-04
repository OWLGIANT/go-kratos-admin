package helper

import (
	"math"
	"strings"
)

// 速度很慢，不一定准确，先别用
func Base32Encode(decimalNumber int64) string {
	const charset = "0123456789ABCDEFGHIJKLMNOPQRSTUV"
	var result strings.Builder

	for decimalNumber > 0 {
		remainder := decimalNumber % 32
		decimalNumber /= 32
		result.WriteByte(charset[remainder])
	}

	// Reverse the result
	runes := []rune(result.String())
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}

	return string(runes)
}

func Base32Decode(base32Number string) int64 {
	const charset = "0123456789ABCDEFGHIJKLMNOPQRSTUV"
	base32Map := make(map[rune]int)

	for i, char := range charset {
		base32Map[char] = i
	}

	var result int64
	for _, char := range base32Number {
		result = result*32 + int64(base32Map[char])
	}

	return result
}

const num2char = "0123456789abcdefghijklmnopqrstuvwxyz"

// 10进制数转换   n 表示进制， 16 or 36
func NumToBHex(num, n int64) string {
	num_str := ""
	for num != 0 {
		yu := num % n
		num_str = string(num2char[yu]) + num_str
		num = num / n
	}
	return strings.ToUpper(num_str)
}

// 36进制数转换   n 表示进制， 16 or 36
func BHex2Num(str string, n int64) int {
	str = strings.ToLower(str)
	v := 0.0
	length := len(str)
	for i := 0; i < length; i++ {
		s := string(str[i])
		index := strings.Index(num2char, s)
		v += float64(index) * math.Pow(float64(n), float64(length-1-i)) // 倒序
	}
	return int(v)
}
