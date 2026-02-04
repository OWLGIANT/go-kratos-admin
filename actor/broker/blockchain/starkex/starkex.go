package starkex

func NewSigner(starkPrivateKey string) *Signer {
	s := new(Signer)
	s.starkPrivateKey = starkPrivateKey

	return s
}

func WithdrawSign(starkPrivateKey string, param WithdrawSignParam) (string, error) {
	return NewSigner(starkPrivateKey).SignWithdraw(param)
}

func TransferSign(starkPrivateKey string, param TransferSignParam) (string, error) {
	return NewSigner(starkPrivateKey).SignTransfer(param)
}

func OrderSign(starkPrivateKey string, param OrderSignParam) (string, string, error) {
	s := NewSigner(starkPrivateKey)
	// bench.AddOneNode("after new signer")
	fee, sign, err := s.SignOrder(param)
	// bench.AddOneNode("after sign order")
	return fee, sign, err
}
