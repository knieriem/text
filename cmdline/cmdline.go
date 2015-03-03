package cmdline

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/knieriem/text"
)

type Cmd struct {
	Map       map[string]Cmd
	Fn        func(arg []string) error
	Arg       []string
	Opt       []string
	Help      string
	Flags     string
	InitFlags func(f *flag.FlagSet)
}

type CmdLine struct {
	*cmdLineReader
	inputStack  []*cmdLineReader
	savedPrompt string

	cmdMap      map[string]Cmd
	ExtraHelp   func()
	Prompt      string
	ConsoleOut  io.Writer
	Stdout      io.Writer
	Forward     io.Writer
	Errf        func(format string, v ...interface{})
	FnNotFound  func(string)
	FnFailed    func(string, error)
	FnWrongNArg func(string)
}

type cmdLineReader struct {
	text.Scanner
	io.Closer
}

func newCmdLineReader(s text.Scanner, c io.Closer) *cmdLineReader {
	return &cmdLineReader{s, c}
}

func NewCmdLine(s text.Scanner, m map[string]Cmd) (cl *CmdLine) {
	cl = new(CmdLine)
	cl.cmdLineReader = newCmdLineReader(s, nil)
	cl.cmdMap = m
	if _, ok := m["."]; !ok {
		m["."] = Cmd{
			Arg: []string{"FILE"},
			Fn: func(arg []string) (err error) {
				f, err := os.Open(arg[1])
				if err == nil {
					cl.inputStack = append(cl.inputStack, cl.cmdLineReader)
					cl.cmdLineReader = newCmdLineReader(bufio.NewScanner(f), f)
					cl.savedPrompt = cl.Prompt
					cl.Prompt = ""
				}
				return
			},
			Help: "read commands from FILE",
		}
	}
	cl.Errf = func(string, ...interface{}) {}
	cl.FnNotFound = func(cmd string) {
		cl.Errf("%s: no such command\n", cmd)
	}
	cl.FnFailed = func(cmd string, err error) {
		cl.Errf("%s: %s\n", cmd, err)
	}
	cl.FnWrongNArg = func(cmd string) {
		cl.Errf("%s: wrong number of arguments\n", cmd)
	}
	return cl
}

func (cl *CmdLine) Process() (err error) {
	var line string

	//processLoop:
	for {
		if cl.Prompt != "" {
			fmt.Fprintf(cl.ConsoleOut, "%s", cl.Prompt)
		}
		if !cl.Scan() {
			err = cl.Err()
			if err == nil {
				if sz := len(cl.inputStack); sz != 0 {
					sz--
					cl.cmdLineReader.Close()
					cl.cmdLineReader = cl.inputStack[sz]
					cl.inputStack = cl.inputStack[:sz]
					if sz == 0 {
						cl.Prompt = cl.savedPrompt
					}
					continue
				}
			}
			break
		}
		line = cl.Text()
		if cl.Prompt != "" {
		again:
			if strings.HasPrefix(line, cl.Prompt) {
				line = line[len(cl.Prompt):]
				goto again
			}
		}
		args := text.Tokenize(line)
		if len(args) == 0 {
			if cl.Forward != nil {
				cl.fwd([]byte{'\n'})
			}
			continue
		}
		if strings.HasPrefix(args[0], "#") {
			continue
		}
		name := args[0]
		if name == "help" {
			cl.help(cl.Stdout, args[1:])
			if cl.Forward != nil {
				cl.fwd([]byte("help\n"))
			}
			continue
		}

		m := cl.cmdMap
		cmdName := name

	retry:
		cmd, ok := m[cmdName]
		if !ok {
			if iDot := strings.Index(cmdName, "."); iDot != -1 {
				if cmd, ok = m[cmdName[:iDot]]; ok {
					m = cmd.Map
					if m != nil {
						cmdName = cmdName[iDot+1:]
						goto retry
					}
				}
			}
			if cl.Forward != nil {
				cl.fwd([]byte(cl.Text() + "\n"))
			} else {
				cl.FnNotFound(name)
			}
			continue
		}
		if cmd.Map != nil {
			if cmd, ok = cmd.Map[""]; !ok {
				cl.FnNotFound(name)
				continue
			}
		}
		if cmd.InitFlags != nil {
			f := flag.NewFlagSet("", flag.ExitOnError)
			cmd.InitFlags(f)
			f.Parse(args[1:])
			args = append(args[:1], f.Args()...)
		}
		n := len(args) - 1

		nmin := 0
		narg := len(cmd.Arg)
		nopt := len(cmd.Opt)
		if narg > 0 && cmd.Arg[narg-1] == "..." {
			nmin = narg - 1
			goto checkNMin
		}
		if nopt > 1 && cmd.Opt[nopt-1] == "..." {
			nmin = narg
			goto checkNMin
		}
		nmin = narg
		if n > narg+nopt {
			cl.FnWrongNArg(name)
			continue
		}
	checkNMin:
		if n < nmin {
			cl.FnWrongNArg(name)
			continue
		}

		err = cmd.Fn(args) // run it
		if err != nil {
			cl.FnFailed(name, err)
		}
	}
	return
}

func (cl *CmdLine) fwd(line []byte) {
	_, err := cl.Forward.Write(line)
	if err != nil {
		cl.Errf("forwarding write failed: %v", err)
	}

}

func (cl *CmdLine) help(w io.Writer, args []string) {
	hasWritten := false
	cmdName := ""
	iDot := -1
	if len(args) > 0 {
		cmdName = args[0]
	}
	pfx := ""
	m := cl.cmdMap
retry:
	iDot = strings.Index(cmdName, ".")
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		v := m[name]
		if cmdName != "" {
			if name == cmdName {
				if v.Map != nil {
					pfx += cmdName + "."
					cmdName = ""
					m = v.Map
					goto retry
				}
				goto found
			}
			if iDot == -1 {
				continue
			}
			if name != cmdName[:iDot] {
				continue
			}
			if v.Map == nil {
				continue
			}
			pfx += cmdName[:iDot+1]
			cmdName = cmdName[iDot+1:]
			m = v.Map
			goto retry
		}
	found:
		flags := v.Flags
		if flags != "" {
			flags = " " + flags
		}
		if pfx != "" {
			if name == "" {
				name = pfx[:len(pfx)-1]
			} else {
				name = pfx + name
			}
		}
		fmt.Fprintln(w, "\t"+name+flags+argString(" ", v.Arg, "")+argString(" [", v.Opt, "]"))
		if v.Help != "" {
			for _, s := range strings.Split(v.Help, "\n") {
				fmt.Fprintln(w, "\t\t"+s)
			}
		}
		fmt.Fprint(w, "\n")
		hasWritten = true
	}
	if !hasWritten && len(args) > 0 {
		cl.FnNotFound(args[0])
	}
	if cl.ExtraHelp != nil {
		cl.ExtraHelp()
	}
}

func argString(pfx string, args []string, sfx string) string {
	if len(args) == 0 {
		return ""
	}
	return pfx + strings.Join(args, " ") + sfx
}
