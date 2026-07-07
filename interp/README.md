# interp

Package _interp_ provides a command line interpreter (also REPL, read-eval-print-loop) inspired by the _Plan9 rc_ shell.

The implementation provides:

- **variables:** `a=1`, `echo $a`, `a=2 echo $a`
- **functions**
- **command substitution:** `` `{command} ``
- **conditionals:** `if`, `if not`, `!`
- **for loops**: `for i in RANGE|LIST`, `for N`, `for DURATION`
- **output redirection:**  `super > super_frame.txt`
- sourcing: script files can be included using `. script_file.txt`
- builtin commands
- extensible by user providable commands that process a
  string argument list and return an `error`

## Example Usage:

```Go
package main

import (
	"bufio"
	"log"
	"strings"

	"github.com/knieriem/text/interp"
)

func main() {

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
```

## Example: Recursive Fibonacci Algorithm

```rc
fn fib {
	if ~ $1 0 1 {
		echo $1
		return
	}
	expr `{fib `{expr $1 - 1}} + `{fib `{expr $1 - 2}}
}

# calculate the first n Fibonacci numbers
n=18
for i in ...$n {
	echo $i: `{fib $i}
}
```

## Custom functions

Functions can be defined using the builtin `fn` command (`help builtin` shows a list of builtin commands).
For example, to define a function that prints its arguments:

```rc
fn p {
	echo narg: $#*
	echo args: $*
}
```

Then call it in the shell just using `p` as command.
Note that for the indentation in the function block (`{}`) it is recommended to
use _TAB_ characters, i.e. one TAB per indentation level.

Refer to function arguments using `$1`, `$2` like in a Unix shell.

To get a list of currently defined functions, type `fn`.


## Loading a script from a text file

A script can be loaded from a file using the builtin `.` _source_ command.

    . custom_script.txt

If the script filename contains spaces, use single quotes:

    . 'custom script.txt'

## Builtin commands

Output of `help builtin`:

```
    ! [CMD ...]
        Invert the exit status of the following command.

    . FILE
        Read commands from FILE.

    binary
        binary encoding commands
        See `help binary' for details.

    break
        Exit the current loop.

    cat FILE
        Print the contents of FILE.

    continue
        Exit the current loop.

    echo [ARG ...]
        Print arguments.

    exit
        Terminate the command line processor.

    expr ARG1 OP ARG2
        evaluate expressions

    false
        Return an exit status indicating failure

    flag f +-
        Set a flag as in Plan 9's rc:
            e    Exit if a simple command (not part of an if-condition) fails.
            x    Print each simple command before executing it.

    fn [NAME CMD ...]
        Define a function, or display its definition. CMD can be
        a single command, or a block enclosed in '{' and '}':
            fn a {
                cmdb
                cmdc
            }

    for ARG ...
        A for loop may be constructed in one of these ways:
        
            for {
                # commands
            }
            Repeat the loop body continuously.
        
            for N CmdOrBlock
                Repeat a command N times.
        
            for DURATION CmdOrBlock
                Repeat a command for a specified duration.
        
            for VAR in LIST CmdOrBlock
                Loop over each element of LIST, updating VAR correspondingly,
                and execute the command (or block) at each loop run.
        
            for VAR in RANGE CmdOrBlock
                Repeat a command as often as defined by the RANGE expression,
                updating VAR to the current value.
                RANGE may be
                    inclusive: [i0] ".." [n-1]    (e.g. -2..4 => -2 -1 0 1 2 3)
                    exclusive: [i0] "..." [n]     (e.g. ...3  => 0 1 2)
        
                The start index may be ommitted, defaulting to 0.
                The end index may be ommitted, resulting in an unbounded loop.
        
            CmdOrBlock can be either a simple command, or a "{" starting a
            loop body block, similar as in a function definition.

    if CMD ...

    printf FMT [ARG ...]
        Print arguments with fmt.Printf style formatting.

    return
        Return from the current function.

    shift [N]
        Delete the first n (default: 1) elements of $*

    sleep DURATION
        Sleep for the specified duration.

    test ARG1 OP ARG2
        test conditions

    unbind NAME
        Unbind a function.

    ~ SUBJECT PATTERN ...
        Returns success if subject matches any pattern.
```

## About

The package was created in 2014 providing a simple REPL,
with nearly no builtin commands, to be able to create simple tools
that provide a command line with application specific, custom commands;
reading commands from a file (i.e. sourcing via `.`) was possible already.
As this proved useful, functions and variables, `if` and `if not`, `repeat` (a simpler `for`) got implemented.
Some years later, `break` and `return` commands got added, as well as _rc_-style `flag`.

In 2026, `` `{command} `` substitution was implemented,
which defines a milestone, as it supports processing the output of
commands and use the result in the same script.
With `$v(n)` syntax, individual fields of command output can be referenced,
just like in _rc_.
Within the same update cycle, `for` got added as a replacement for the now
hidden `repeat`, providing loop variables and looping over lists or integer ranges.
Also, with command substition being available, `expr` was added to perform simple
arithmetic.
