package interp

import (
	"fmt"
	"strconv"
	"time"
)

var cmdPrintf = Cmd{
	Arg: []string{"FMT"},
	Opt: []string{"ARG", "..."},
	Fn: func(w Context, arg []string) (err error) {
		format := arg[1]
		arg = arg[2:]
		args := make([]any, len(arg))
		for i, a := range arg {
			args[i] = printfArg(a)
		}
		fu, err := strconv.Unquote(format)
		if err == nil {
			format = fu
		} else {
			fu, err = strconv.Unquote(`"` + format + `"`)
			if err == nil {
				format = fu
			}
		}
		_, err = fmt.Fprintf(w, format, args...)
		return
	},
	Help: "Print arguments with fmt.Printf style formatting.",
}

type printfArg string

func (a printfArg) Format(f fmt.State, verb rune) {
	arg := string(a)

	s := arg
	if su, err := strconv.Unquote(arg); err == nil {
		s = su
	}

	switch verb {
	case 'd', 'x', 'X', 'b', 'o', 'O', 'c', 'q':
		if i64, err := strconv.ParseInt(arg, 0, 0); err == nil {
			fmt.Fprintf(f, fmt.FormatString(f, verb), int(i64))
			return
		}
	case 'f', 'F', 'e', 'E', 'g', 'G':
		if fl, err := strconv.ParseFloat(arg, 64); err == nil {
			fmt.Fprintf(f, fmt.FormatString(f, verb), fl)
			return
		}
	case 'v':
		d, err := time.ParseDuration(arg)
		if err == nil {
			fmt.Fprintf(f, fmt.FormatString(f, verb), d)
			return
		}
		fallthrough
	case 's':
		fmt.Fprintf(f, fmt.FormatString(f, verb), s)
		return
	}
	fmt.Fprintf(f, fmt.FormatString(f, verb), arg)
}
