package main

// #include <stdlib.h>
import "C"
import (
	"encoding/json"
	"github.com/redraskal/r6-dissect/dissect"
	"github.com/rs/zerolog"
	"os"
)

func marshalToString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{\"error\":\"something went wrong during json Marshal\"}"
	}
	return string(b)
}

func convertForExport(v any) string {
	type export struct {
		Data any `json:"data"`
	}
	return marshalToString(export{
		Data: v,
	})
}

func convertErrorForExport(err error) string {
	type export struct {
		Error string `json:"error"`
	}
	return marshalToString(export{
		Error: err.Error(),
	})
}

//export dissect_read
func dissect_read(input *C.char) *C.char {
	zerolog.SetGlobalLevel(zerolog.Disabled)
	path := C.GoString(input)
	f, err := os.Open(path)
	if err != nil {
		res := convertErrorForExport(err)
		return C.CString(res)
	}
	defer f.Close()
	s, err := f.Stat()
	if err != nil {
		res := convertErrorForExport(err)
		return C.CString(res)
	}
	if s.IsDir() {
		m, err := dissect.NewMatchReader(f)
		if err != nil {
			res := convertErrorForExport(err)
			return C.CString(res)
		}
		if err := m.Read(); err != nil {
			res := convertErrorForExport(err)
			return C.CString(res)
		}
		j := m.Data()
		if err != nil {
			res := convertErrorForExport(err)
			return C.CString(res)
		}
		res := convertForExport(j)
		return C.CString(res)
	} else {
		r, err := dissect.NewReader(f)
		if err != nil {
			res := convertErrorForExport(err)
			return C.CString(res)
		}
		if err := r.Read(); !dissect.Ok(err) {
			res := convertErrorForExport(err)
			return C.CString(res)
		}
		type round struct {
			dissect.Header
			MatchFeedback []dissect.MatchUpdate      `json:"matchFeedback"`
			PlayerStats   []dissect.PlayerRoundStats `json:"stats"`
		}
		res := convertForExport(round{
			r.Header,
			r.MatchFeedback,
			r.PlayerStats(),
		})
		return C.CString(res)
	}
}

func main() {}
