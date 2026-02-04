package starkex

import (
	"errors"
	"fmt"
	"math/big"
)

type Signable interface {
	initMsg() error
	getHash() (string, error)
}

type Signer struct {
	NetworkId       int
	signer          Signable
	starkPrivateKey string
	hash            string
	err             error
}

func (s *Signer) SignOrder(param OrderSignParam) (string, string, error) {
	signer := new(OrderSigner)
	signer.param = param

	s.signer = signer
	sign, err := s.Sign()
	if err != nil {
		return "", sign, err
	}
	limitFeeVal := signer.GetHumanLimitFeeVal()
	return limitFeeVal, sign, err
}

func (s *Signer) SignWithdraw(param WithdrawSignParam) (string, error) {
	signer := new(WithdrawSigner)
	signer.param = param
	s.signer = signer
	return s.Sign()
}

func (s *Signer) SignTransfer(param TransferSignParam) (string, error) {
	signer := new(TransferSigner)
	signer.param = param
	s.signer = signer
	return s.Sign()
}

func (s *Signer) SetSigner(signer Signable) *Signer {
	s.signer = signer
	return s
}

func (s *Signer) SetNetworkId(networkId int) *Signer {
	s.NetworkId = networkId
	return s
}

func (s *Signer) Sign() (string, error) {
	if s.signer == nil {
		return "", errors.New("please init signer")
	}
	err := s.signer.initMsg()
	if err != nil {
		return "", err
	}
	// bench.AddOneNode("before getHash")
	hash, err := s.signer.getHash()
	// bench.AddOneNode("after getHash")
	if err != nil {
		return "", err
	}
	s.hash = hash
	r, s1 := s.doSign()
	// bench.AddOneNode("after doSign")
	if s.err != nil {
		return "", s.err
	}
	res := SerializeSignature(r, s1)
	return res, nil
	// return SerializeSignature(r, s1)
}

func (s *Signer) doSign() (*big.Int, *big.Int) {
	priKey, _ := new(big.Int).SetString(s.starkPrivateKey, 16)
	msgHash, _ := new(big.Int).SetString(s.hash, 10)
	seed := 0
	EcGen := pedersenCfg.ConstantPoints[1]
	alpha := pedersenCfg.ALPHA
	nBit := big.NewInt(0).Exp(big.NewInt(2), N_ELEMENT_BITS_ECDSA, nil)
	iter := 0
	// bench.AddOneNode("before GenerateKRfc6979")
	for {
		iter++
		k := GenerateKRfc6979(msgHash, priKey, seed)
		//	Update seed for next iteration in case the value of k is bad.
		if seed == 0 {
			seed = 1
		} else {
			seed += 1
		}
		// Cannot fail because 0 < k < EC_ORDER and EC_ORDER is prime.
		x := ecMult(k, EcGen, alpha, FIELD_PRIME)[0]
		// !(1 <= x < 2 ** N_ELEMENT_BITS_ECDSA)
		if !(x.Cmp(one) > 0 && x.Cmp(nBit) < 0) {
			continue
		}
		// msg_hash + r * priv_key
		x1 := big.NewInt(0).Add(msgHash, big.NewInt(0).Mul(x, priKey))
		// (msg_hash + r * priv_key) % EC_ORDER == 0
		if big.NewInt(0).Mod(x1, EC_ORDER).Cmp(zero) == 0 {
			continue
		}
		// w = div_mod(k, msg_hash + r * priv_key, EC_ORDER)
		w := divMod(k, x1, EC_ORDER)
		// not (1 <= w < 2 ** N_ELEMENT_BITS_ECDSA)
		if !(w.Cmp(one) > 0 && w.Cmp(nBit) < 0) {
			continue
		}
		s1 := divMod(one, w, EC_ORDER)
		fmt.Println("doSign iter ", iter)
		// bench.AddOneNode("after GenerateKRfc6979")
		return x, s1
	}
}
