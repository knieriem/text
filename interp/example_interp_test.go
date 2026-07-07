package interp_test

import (
	"bufio"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/knieriem/text/interp"
)

var inputScript = strings.Replace(`

fn gcd {
	a=$1 b=$2 _gcd
}
fn _gcd {
	for {
		if ~ $a $b {
			echo $a
			return
		}
		if test $a -gt $b {
			a=´{expr $a - $b}
		}
		if not {
			b=´{expr $b - $a}
		}
	}
}

pi=´{math.Pi}
printf %.8f\n $pi

for i in ..12 {
	sin=´{math.sin ´{expr ´{expr $pi * $i} / 6}}
	if ~ $i 0 {
		printf '   0π -> %.4g\n' $sin
		continue
	}
	d=´{gcd $i 6}
	num=´{expr $i / $d}
	denom=´{expr 6 / $d}
	if ~ $denom 1 {
		if ~ $num 1 {
			printf '    '
		}
		if not {
			printf '%4d' $num
		}
	}
	if not {
		printf '%2d/%d' $num $denom
	}
	printf 'π -> %.4g\n' $sin
}

`, "´", "`", -1)

func ExampleNewCmdInterp_extended() {

	cmds := interp.CmdMap{
		"math": {
			Help: "math commands",
			Map: interp.CmdMap{
				"Pi": {
					Help: "Return the value of π.",
					Fn: func(w interp.Context, _ []string) error {
						w.Printf("%g", math.Pi)
						return nil
					},
				},
				"sin": {
					Help: "Calculate the sinus of the argument in radians.",
					Arg:  []string{"RAD"},
					Fn: func(w interp.Context, arg []string) error {
						r, err := strconv.ParseFloat(arg[1], 64)
						if err != nil {
							return err
						}
						sin := math.Sin(r)
						w.Printf("%g", sin)
						return nil
					},
				},
			},
		},
	}

	r := strings.NewReader(inputScript)
	s := bufio.NewScanner(r)
	cli := interp.NewCmdInterp(s, cmds)

	err := cli.Process()
	if err != nil {
		log.Fatal(err)
	}

	// Output:
	//
	// 3.14159265
	//    0π -> 0
	//  1/6π -> 0.5
	//  1/3π -> 0.866
	//  1/2π -> 1
	//  2/3π -> 0.866
	//  5/6π -> 0.5
	//     π -> 1.225e-16
	//  7/6π -> -0.5
	//  4/3π -> -0.866
	//  3/2π -> -1
	//  5/3π -> -0.866
	// 11/6π -> -0.5
	//    2π -> -2.449e-16
}
