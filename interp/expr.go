package interp

import (
	"errors"
	"fmt"
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
			a, err := a1.add(sign, a2)
			if err != nil {
				return err
			}
			ctx.Println(a.value())

		case "*":
			a, err := a1.mult(a2)
			if err != nil {
				return err
			}
			ctx.Println(a.value())

		case "/":
			a, err := a1.div(a2)
			if err != nil {
				return err
			}
			ctx.Println(a.value())
		}
		return nil
	},
}

var errExprArgTypeMismatch = errors.New("argument type mismatch")

type exprVal struct {
	kind

	i int
	f float64
	t time.Duration
}

type kind int

const (
	exprValKindInt kind = iota
	exprValKindFloat
	exprValKindDuration
)

func (v *exprVal) add(sign int, v2 *exprVal) (*exprVal, error) {
	if v.hasUnit() != v2.hasUnit() {
		return nil, errExprArgTypeMismatch
	}
	switch v.kind {
	case exprValKindDuration:
		v.t += time.Duration(sign) * v2.t

	case exprValKindFloat:
		v.f += float64(sign) * v2.float()

	case exprValKindInt:
		v.i += sign * v2.i
	}
	return v, nil
}

func (v *exprVal) mult(v2 *exprVal) (*exprVal, error) {
	if v.hasUnit() && v2.hasUnit() {
		return nil, errExprArgTypeMismatch
	}
	if v2.kind == exprValKindDuration {
		v, v2 = v2, v
	}
	if v.kind == exprValKindDuration {
		switch v2.kind {
		case exprValKindInt:
			v.t = v.t * time.Duration(v2.i)
		case exprValKindFloat:
			v.t = time.Duration(float64(v.t) * v2.f)
		}
		return v, nil
	}
	if v2.kind == exprValKindFloat {
		v, v2 = v2, v
	}
	if v.kind == exprValKindFloat {
		v.f = v.f * v2.float()
	} else {
		v.i *= v2.i
	}
	return v, nil
}

func (v *exprVal) div(v2 *exprVal) (*exprVal, error) {
	if !v.hasUnit() && v2.hasUnit() {
		return nil, errExprArgTypeMismatch
	}

	if v.kind == exprValKindDuration {
		switch v2.kind {
		case exprValKindDuration:
			v.kind = exprValKindInt
			v.i = int(v.t / v2.t)
		case exprValKindInt:
			v.t /= time.Duration(v2.i)
		case exprValKindFloat:
			v.t = time.Duration(float64(v.t) / v2.f)
		}
		return v, nil
	}

	if v.kind == exprValKindFloat {
		v.f /= v2.float()
	} else {
		switch v2.kind {
		case exprValKindInt:
			v.i /= v2.i
		case exprValKindFloat:
			v.kind = v2.kind
			v.f = float64(v.i) / v2.f
		}
	}
	return v, nil
}

func (v *exprVal) value() any {
	switch v.kind {
	case exprValKindDuration:
		return v.t
	case exprValKindFloat:
		return v.f
	case exprValKindInt:
		return v.i
	}
	return nil
}

func (v *exprVal) float() float64 {
	switch v.kind {
	case exprValKindInt:
		return float64(v.i)
	case exprValKindDuration:
		return float64(v.t)
	}
	return v.f
}

func (k kind) hasUnit() bool {
	return k == exprValKindDuration
}

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
	f, err := strconv.ParseFloat(v, 64)
	if err == nil {
		return &exprVal{kind: exprValKindFloat, f: f}, nil
	}
	return nil, fmt.Errorf("cannot parse argument %q", v)
}
