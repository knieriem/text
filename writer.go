package text

import (
	"io"
)

type Writer interface {
	io.Writer
	Printf(format string, arg ...any) (n int, err error)
	Println(arg ...any) (n int, err error)
	PrintSlice([]string) (n int, err error)
}
