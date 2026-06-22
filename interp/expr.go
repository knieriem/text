package interp

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"
)

var expr = Cmd{
	Help: "evaluate expressions",
	Arg:  []string{"ARG1", "OP", "ARG2"},
	Fn: func(ctx Context, arg []string) error {
		a1, err := parseExprVal(arg[1])
		if err != nil {
			return err
		}
		a2, err := parseExprVal(arg[3])
		if err != nil {
			return err
		}
		op := arg[2]
		sign := 1
		switch op {
		case "-":
			sign = -1
			fallthrough
		case "+":
			if a1.kind != a2.kind {
				log.Println("arg", arg[1], arg[3], a1, a2)
				return errExprArgTypeMismatch
			}
			switch a1.kind {
			case exprValKindDuration:
				ctx.Println(a1.t + time.Duration(sign)*a2.t)
			case exprValKindInt:
				ctx.Println(a1.i + sign*a2.i)
			}
		case "*":
			if a1.kind == exprValKindDuration {
				if a2.kind != exprValKindInt {
					return errExprArgTypeMismatch
				}
				ctx.Println(a1.t * time.Duration(a2.i))
			}
			if a2.kind == exprValKindDuration {
				if a1.kind != exprValKindInt {
					return errExprArgTypeMismatch
				}
				ctx.Println(a2.t * time.Duration(a1.i))
			}
			if a1.kind == exprValKindInt {
				if a2.kind != exprValKindInt {
					return errExprArgTypeMismatch
				}
				ctx.Println(a1.i * a2.i)
			}
		}
		return nil
	},
}

var errExprArgTypeMismatch = errors.New("argument type mismatch")

type exprVal struct {
	kind int

	i int
	t time.Duration
}

const (
	exprValKindInt = iota
	exprValKindDuration
)

func parseExprVal(v string) (*exprVal, error) {
	if v != "0" {
		d, err := time.ParseDuration(v)
		if err == nil {
			return &exprVal{kind: exprValKindDuration, t: d}, nil
		}
	}
	i, err := strconv.ParseInt(v, 0, 0)
	if err == nil {
		return &exprVal{kind: exprValKindInt, i: int(i)}, nil
	}
	return nil, fmt.Errorf("cannot parse argument %q", v)
}
