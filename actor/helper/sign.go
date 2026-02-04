package helper

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"hash"

	"actor/third/log"
)

// 各类签名方法，不要随意修改，会有多处使用

func MD5(str []byte) []byte {
	h := md5.New()
	if _, err := h.Write(str); err != nil {
		log.Errorf("MD5Byte error: %v", err)
	}
	res := h.Sum(nil)
	dst := make([]byte, hex.EncodedLen(len(res)))
	hex.Encode(dst, res)
	return dst
}

// HmacSha256签名, 没有指定格式
func HmacSHA256(message []byte, secretKey []byte) ([]byte, error) {
	mac := hmac.New(sha256.New, secretKey)
	_, err := mac.Write(message)
	if err != nil {
		return nil, err
	}
	signByte := mac.Sum(nil)
	return signByte, nil
}

// HmacED25519签名, 没有指定格式
func GetEd25519SigningKey(secretKey string) (ed25519.PrivateKey, error) {
	keySeed, err := base64.StdEncoding.DecodeString(secretKey)
	if err != nil {
		return nil, err
	}
	return ed25519.NewKeyFromSeed(keySeed), nil
}

// HmacED25519签名, 没有指定格式
func HmacED25519(message []byte, signingKey ed25519.PrivateKey) ([]byte, error) {
	signature := ed25519.Sign(signingKey, message)
	return signature, nil
}

// 输出 hex 格式
func EncodeHex(str []byte) []byte {
	dst := make([]byte, hex.EncodedLen(len(str)))
	hex.Encode(dst, str)
	return dst
}

// 输出Base64 格式
func EncodeBase64(msg []byte) []byte {
	buf := make([]byte, base64.StdEncoding.EncodedLen(len(msg)))
	base64.StdEncoding.Encode(buf, msg)
	return buf
}

// HmacSha256签名, 并输出Base64 格式
func HmacSHA256Base64(message []byte, secretKey []byte) (rsp []byte, err error) {
	if rsp, err = HmacSHA256(message, secretKey); err != nil {
		return
	} else {
		return EncodeBase64(rsp), nil
	}
}

// HmacED25519Base64, 并输出Base64 格式
func HmacED25519Base64(message []byte, signingKey ed25519.PrivateKey) (rsp []byte, err error) {
	if rsp, err = HmacED25519(message, signingKey); err != nil {
		return
	} else {
		return EncodeBase64(rsp), nil
	}
}

// HmacSha256签名, 并输出 hex 格式
func HmacSHA256Hex(message []byte, secretKey []byte) (rsp []byte, err error) {
	if rsp, err = HmacSHA256(message, secretKey); err != nil {
		return
	} else {
		return EncodeHex(rsp), nil
	}
}

// HmacSha256签名, 并输出 hex 格式, 自行保证dst长度
func HmacSHA256HexDst(message []byte, secretKey []byte, dst []byte) (writeCnt int, err error) {
	if res, err := HmacSHA256Dst(message, secretKey); err != nil {
		return 0, err
	} else {
		// dst2 := make([]byte, hex.EncodedLen(len(res)))
		l := hex.EncodedLen(len(res))
		dst = dst[:l]
		return l, EncodeHexDst(res, dst)
	}
}

// 输出 hex 格式
func EncodeHexDst(str []byte, dst []byte) error {
	if hex.Encode(dst, str) > 0 {
		return nil
	}
	return errors.New("EncodeHexDst error")
}

func HmacSHA256Dst(message []byte, secretKey []byte) ([]byte, error) {
	mac := hmac.New(sha256.New, secretKey)
	mac.Reset()
	_, err := mac.Write(message)
	if err != nil {
		return nil, err
	}
	return mac.Sum(nil), nil // opt, Sum没法写到一个buf, 2023-07-27
}
func MD5Dst(str []byte, dst []byte) {
	h := md5.New()
	if _, err := h.Write(str); err != nil {
		log.Errorf("MD5Byte error: %v", err)
	}
	res := h.Sum(nil)
	hex.Encode(dst, res)
}

//* 各类签名结构体。不使用interface，避免虚表

type SignerMD5 struct {
	buf  []byte
	size int
	md5  hash.Hash
}

func NewSignerMD5() *SignerMD5 {
	return &SignerMD5{
		size: 128,
		buf:  make([]byte, 128),
		md5:  md5.New(),
	}
}
func (s *SignerMD5) Sign(message []byte) []byte {
	s.md5.Reset()
	if _, err := s.md5.Write(message); err != nil {
		panic(err) // 极少出现但又不是不可能。返回err，上层需要关注处理，忽略又会错过异常提醒，所以panic
	}
	res := s.md5.Sum(nil)
	l := hex.EncodedLen(len(res))
	if l > s.size {
		s.buf = make([]byte, l)
		s.size = l
	}
	s.buf = s.buf[:l]
	hex.Encode(s.buf, res)
	return s.buf
}

type SignerHmacSHA256Hex struct {
	buf  []byte
	size int
	hmac hash.Hash
}

func NewSignerHmacSHA256Hex(secretKey []byte) *SignerHmacSHA256Hex {
	return &SignerHmacSHA256Hex{
		size: 128,
		buf:  make([]byte, 128),
		hmac: hmac.New(sha256.New, secretKey),
	}
}

func (s *SignerHmacSHA256Hex) Sign(message []byte) []byte {
	s.hmac.Reset()
	_, err := s.hmac.Write(message)
	if err != nil {
		panic(err) // 极少出现但又不是不可能。返回err，上层需要关注处理，忽略又会错过异常提醒，所以panic
	}
	res := s.hmac.Sum(nil) // opt, Sum没找到方法写到一个buf, 2023-07-27
	l := hex.EncodedLen(len(res))
	if l > s.size {
		s.buf = make([]byte, l)
		s.size = l
	}
	s.buf = s.buf[:l]
	hex.Encode(s.buf, res)
	return s.buf
}

type SignerHmacSHA256Base64 struct {
	buf  []byte
	size int
	hmac hash.Hash
}

func NewSignerHmacSHA256Base64(secretKey []byte) *SignerHmacSHA256Base64 {
	return &SignerHmacSHA256Base64{
		size: 128,
		buf:  make([]byte, 128),
		hmac: hmac.New(sha256.New, secretKey),
	}
}

func (s *SignerHmacSHA256Base64) Sign(message []byte) []byte {
	s.hmac.Reset()
	_, err := s.hmac.Write(message)
	if err != nil {
		panic(err) // 极少出现但又不是不可能。返回err，上层需要关注处理，忽略又会错过异常提醒，所以panic
	}
	res := s.hmac.Sum(nil)

	l := base64.StdEncoding.EncodedLen(len(res))
	if l > s.size {
		s.buf = make([]byte, l)
		s.size = l
	}
	s.buf = s.buf[:l]
	base64.StdEncoding.Encode(s.buf, res)
	return s.buf
}
