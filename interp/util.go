package interp

import (
	"strconv"
	"strings"
)

func Atob(v string) (b byte, err error) {
	i64, err := strconv.ParseUint(v, 0, 8)
	if err == nil {
		b = byte(i64)
	}
	return
}

func Argbytes(arg []string) (data []byte, err error) {
	data = make([]byte, 0, len(arg))
	for _, a := range arg {
		if strings.HasPrefix(a, `"`) && strings.HasSuffix(a, `"`) {
			s, err := strconv.Unquote(a)
			if err != nil {
				return nil, err
			}
			data = append(data, []byte(s)...)
		} else {
			b, err := Atob(a)
			if err != nil {
				return nil, err
			}
			data = append(data, b)
		}
	}
	return data, nil
}
