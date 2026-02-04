package helper

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func BenchmarkSignHmacSHA256(b *testing.B) {
	secretKey := StringToBytes("abcde4bZhXoSW8Dgd6AUp0PH3kOeAu6NlGtEncWzWPU=")
	msg := StringToBytes("12345678901234567890123456789011")
	for i := 0; i < b.N; i++ {
		HmacSHA256(msg, []byte(secretKey))
	}
}
func BenchmarkSignHmacED25519(b *testing.B) {
	ed25519Key, err := GetEd25519SigningKey("abcde4bZhXoSW8Dgd6AUp0PH3kOeAu6NlGtEncWzWPU=")
	if err != nil {
		panic(err)
	}
	msg := StringToBytes("12345678901234567890123456789011")
	for i := 0; i < b.N; i++ {
		HmacED25519(msg, ed25519Key)
	}
}
func BenchmarkSignEIP712(b *testing.B) {
	signingKey, err := crypto.HexToECDSA(hex.EncodeToString([]byte("abcde4bZhXoSW8Dgd6AUp0PH3kOeAu6N")))
	if err != nil {
		panic(err)
	}
	msg := StringToBytes("12345678901234567890123456789011")
	sig, _ := crypto.Sign(msg, signingKey)
	fmt.Println("sig:", len(sig), len(hex.EncodeToString(sig)), hex.EncodeToString(sig))
	for i := 0; i < b.N; i++ {
		crypto.Sign(msg, signingKey)
	}
}
