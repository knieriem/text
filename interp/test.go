package interp

import (
	"errors"
	"strconv"
)

var cmdTest = Cmd{
	Help:        "test conditions",
	Arg:         []string{"ARG1", "OP", "ARG2"},
	HideFailure: true,
	Fn: func(ctx Context, arg []string) error {
		i1, err := strconv.ParseInt(arg[1], 0, 0)
		if err != nil {
			return err
		}
		i2, err := strconv.ParseInt(arg[3], 0, 0)
		if err != nil {
			return err
		}
		op := arg[2]
		switch op {
		case "-gt":
			if i1 > i2 {
				return nil
			}
		case "-lt":
			if i1 < i2 {
				return nil
			}
		case "-eq":
			if i1 == i2 {
				return nil
			}
		}
		return errors.New("false")
	},
}
