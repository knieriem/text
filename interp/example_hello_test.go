package interp_test

import (
	"bufio"
	"log"
	"strings"

	"github.com/knieriem/text/interp"
)

func ExampleNewCmdInterp() {

	cmds := interp.CmdMap{
		"hello": {
			Help: "prints \"Hello {arg}!\"",
			Arg:  []string{"ARG"},
			Fn: func(w interp.Context, arg []string) error {
				w.Printf("Hello %s!", arg[1])
				return nil
			},
		},
	}

	r := strings.NewReader("hello world\n")
	s := bufio.NewScanner(r)
	cli := interp.NewCmdInterp(s, cmds)

	err := cli.Process()
	if err != nil {
		log.Fatal(err)
	}

	// Output:
	//
	// Hello world!
}
