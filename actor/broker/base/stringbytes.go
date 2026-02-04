package base

import (
	"actor/helper"
	"github.com/mailru/easyjson/jwriter"
)

type StringBytes struct {
	Buf []byte
}

func (s StringBytes) MarshalEasyJSON(w *jwriter.Writer) {
	w.String(helper.BytesToString(s.Buf))
}
func (s StringBytes) String() string {
	return helper.BytesToString(s.Buf)
}

func (s StringBytes) Set(src string) {
	s.Buf = s.Buf[:0]
	s.Buf = append(s.Buf, src...)
}

func AllocStringBytes(size int) StringBytes {
	s := StringBytes{}
	s.Buf = make([]byte, 0, size)
	return s
}
