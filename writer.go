package text

import (
	"io"
)

type Writer interface {
	io.Writer
	Printf(format string, arg ...interface{}) (n int, err error)
	Println(arg ...interface{}) (n int, err error)
	PrintSlice([]string) (n int, err error)
}
