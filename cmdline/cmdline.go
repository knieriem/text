package cmdline

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/knieriem/text"
	"github.com/knieriem/text/rc"
)

const (
	defaultGroup = "ZZY__Other commands"
)

type Cmd struct {
	Map         map[string]Cmd
	Fn          func(arg []string) error
	Arg         []string
	Opt         []string
	Help        string
	Hidden      bool
	Group       string
	Flags       string
	InitFlags   func(f *flag.FlagSet)
	ignoreEnv   bool
	hideFailure bool
}

type CmdLine struct {
	*cmdLineReader
	cur         stackEntry
	inputStack  []stackEntry
	lastOk      bool
	savedPrompt string
	tok         *rc.Tokenizer
	envStack    rc.EnvStack

	cmdMap       map[string]Cmd
	funcMap      map[string]string
	ExtraHelp    func()
	DefaultGroup string
	Prompt       string
	ConsoleOut   io.Writer
	Stdout       io.Writer
	Forward      io.Writer
	Errf         func(format string, v ...interface{})
	FnNotFound   func(string)
	FnFailed     func(string, error)
	FnWrongNArg  func(string)

	cIntr chan int
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
	cl.cur.lineReader = cl.cmdLineReader
	cl.funcMap = make(map[string]string)
	cl.cmdMap = m
	builtinCmdMap := map[string]Cmd{
		".": {
			Arg: []string{"FILE"},
			Fn: func(arg []string) (err error) {
				f, err := os.Open(arg[1])
				if err == nil {
					cl.pushStack(f, 0, nil)
				}
				return
			},
			Help:      "Read commands from FILE.",
			ignoreEnv: true,
		},
		"if": {
			Arg: []string{"CMD", "..."},
			Fn: func(arg []string) (err error) {
				cmd, err := cl.parseCmd(arg[len(arg)-1:])
				if err != nil {
					return
				}
				if arg[1] == "not" {
					if cl.cur.cond.result == nil {
						err = errors.New("`if not' does not follow `if'")
						return
					}
					if !*cl.cur.cond.result {
						cl.pushStringStack(cmd)
					}
					return
				}
				cond := rc.Join(arg[1:len(arg)-1]) + "\n" + "_testcond\n"
				cl.pushStringStack(cond)
				cl.cur.cond.cmd = cmd
				return nil
			},
		},
		"_testcond": {
			Hidden: true,
			Fn: func([]string) (err error) {
				cond := &cl.cur.cond
				cmd := cond.cmd
				if cmd == "" {
					return
				}
				cond.cmd = ""
				ok := cl.lastOk
				cl.inputStack[len(cl.inputStack)-1].cond.result = &ok
				if ok {
					cl.pushStringStack(cmd)
				}
				return nil
			},
		},
		"!": {
			hideFailure: true,
			Opt:         []string{"CMD", "..."},
			Fn: func(arg []string) (err error) {
				if len(arg) == 1 {
					return errors.New("false")
				}
				cmd := rc.Join(arg[1:]) + "\n" + "_!\n"
				cl.pushStringStack(cmd)
				return nil
			},
		},
		"_!": {
			hideFailure: true,
			Fn: func([]string) (err error) {
				if cl.lastOk {
					err = errors.New("false")
				}
				return
			},
		},
		"fn": {
			Opt: []string{"NAME", "CMD", "..."},
			Fn: func(arg []string) error {
				switch len(arg) {
				case 1:
					for name := range cl.funcMap {
						cl.dumpFunc(name)
					}
					return nil
				case 2:
					cl.dumpFunc(arg[1])
					return nil
				}
				return cl.parseFunc(arg[1], arg[2:])
			},
			Help: `Define a function, or display its definition. CMD can be
a single command, or a block enclosed in '{' and '}':
	fn a {
		cmdb
		cmdc
	}`,
		},
		"unbind": {
			Arg: []string{"NAME"},
			Fn: func(arg []string) (err error) {
				if _, ok := cl.funcMap[arg[1]]; !ok {
					err = errors.New("function not found")
					return
				}
				delete(cl.funcMap, arg[1])
				return
			},
			Help: "Unbind a function.",
		},
		"repeat": {
			Arg: []string{"N", "CMD"},
			Opt: []string{"ARG", "..."},
			Fn: func(arg []string) error {
				return cl.repeatCmd(arg[1:])
			},
			Help: "Repeat a command N times.",
		},
		"sleep": {
			Fn: func(arg []string) (err error) {
				tArg, err := time.ParseDuration(arg[1])
				if err != nil {
					return
				}
				t := time.NewTimer(tArg)
				select {
				case <-t.C:
				case <-cl.cIntr:
					t.Stop()
					err = ErrInterrupt
				}
				return
			},
			Arg:  []string{"DURATION"},
			Help: "Sleep for the specified duration.",
		},
	}
	if _, ok := m["builtin"]; !ok {
		m["builtin"] = Cmd{
			Map:  builtinCmdMap,
			Help: "Built-in commands.\nMay be called without the `builtin.' prefix.",
		}
	}
	for name, cmd := range builtinCmdMap {
		if _, ok := m[name]; !ok {
			cmd.Hidden = true
			m[name] = cmd
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
	cl.cIntr = make(chan int, 0)
	cl.tok = new(rc.Tokenizer)
	cl.envStack.Push(nil)
	cl.tok.Getenv = cl.envStack.Get
	return cl
}

func (cl *CmdLine) Interrupt(timeout time.Duration) (ok bool) {
	t := time.NewTimer(timeout)
	select {
	case <-t.C:
	case cl.cIntr <- 1:
		t.Stop()
		ok = true
	}
	return
}

type stackEntry struct {
	lineReader *cmdLineReader
	nRepeat    int
	rewind     func() io.ReadCloser
	popEnv     bool
	cond       struct {
		cmd    string
		result *bool
	}
}

func (cl *CmdLine) pushStack(rc io.ReadCloser, nRepeat int, rewind func() io.ReadCloser) {
	cl.inputStack = append(cl.inputStack, cl.cur)
	cl.cur = stackEntry{
		lineReader: newCmdLineReader(bufio.NewScanner(rc), rc),
		nRepeat:    nRepeat,
		rewind:     rewind,
	}
	cl.cmdLineReader = cl.cur.lineReader
	if cl.Prompt != "" {
		cl.savedPrompt = cl.Prompt
		cl.Prompt = ""
	}
}

func (cl *CmdLine) pushStringStack(cmds string) {
	cl.pushStack(ioutil.NopCloser(strings.NewReader(cmds)), 0, nil)
}

func (cl *CmdLine) popStack() {
	if cl.cur.popEnv {
		cl.envStack.Pop()
	}
	sz := len(cl.inputStack)
	sz--
	cl.cmdLineReader.Close()
	cl.cur = cl.inputStack[sz]
	cl.cmdLineReader = cl.cur.lineReader
	cl.inputStack = cl.inputStack[:sz]
	if sz == 0 {
		cl.Prompt = cl.savedPrompt
	}
}

func (cl *CmdLine) popStackAll() {
	for len(cl.inputStack) > 0 {
		cl.popStack()
	}
}

var ErrInterrupt = errors.New("interrupted")

func (cl *CmdLine) Process() (err error) {
	var line string

	ready := make(chan bool)

	//processLoop:
	for {
		if cl.Prompt != "" {
			fmt.Fprintf(cl.ConsoleOut, "%s", cl.Prompt)
		}
		go func() {
			ready <- cl.Scan()
		}()
		scanOk := false
	selAgain:
		select {
		case scanOk = <-ready:
		case <-cl.cIntr:
			if len(cl.inputStack) == 0 {
				err = ErrInterrupt
				return
			} else {
				cl.Errf("%v\n", ErrInterrupt)
				cl.popStackAll()
				if cl.Prompt != "" {
					fmt.Fprintf(cl.ConsoleOut, "%s", cl.Prompt)
				}
				goto selAgain
			}
		}
		if !scanOk {
			err = cl.Err()
			if err == nil {
				if sz := len(cl.inputStack); sz != 0 {
					if cl.cur.nRepeat > 1 {
						cl.cur.nRepeat--
						rc := cl.cur.rewind()
						cl.cur.lineReader = newCmdLineReader(bufio.NewScanner(rc), rc)
						cl.cmdLineReader = cl.cur.lineReader
						continue
					}
					cl.popStack()
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
		c, err := cl.tok.ParseCmdLine(line)
		if err != nil {
			cl.Errf("%v", err)
			continue
		}
		args := c.Fields
		if len(args) == 0 {
			if a := c.Assignments; len(a) != 0 {
				cl.envStack.Insert(a)
				continue
			}
			if cl.Forward != nil {
				cl.fwd([]byte{'\n'})
			}
			continue
		}
		privEnv := false
		if len(c.Assignments) != 0 {
			privEnv = true
		}

		name := args[0]
		if body, ok := cl.funcMap[name]; ok {
			if privEnv {
				cl.envStack.Push(c.Assignments)
			}
			cl.pushStringStack(body)
			cl.cur.popEnv = privEnv
			continue
		}
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
				cl.lastOk = false
			}
			continue
		}
		if cmd.Map != nil {
			if cmd, ok = cmd.Map[""]; !ok {
				cl.FnNotFound(name)
				cl.lastOk = false
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
			cl.lastOk = false
			continue
		}
	checkNMin:
		if n < nmin {
			cl.FnWrongNArg(name)
			cl.lastOk = false
			continue
		}
		if privEnv {
			if !cmd.ignoreEnv {
				cl.envStack.Push(c.Assignments)
			}
		}
		err = cmd.Fn(args) // run it
		cl.lastOk = err == nil
		cl.cur.cond.result = nil
		if cmd.hideFailure {
			err = nil
		}
		if privEnv {
			cl.envStack.Pop()
		}
		if err != nil {
			cl.FnFailed(name, err)
			if err == ErrInterrupt {
				cl.popStackAll()
			}
		}
	}
	return
}

func (cl *CmdLine) fwd(line []byte) {
	_, err := cl.Forward.Write(line)
	if err != nil {
		cl.Errf("forwarding write failed: %v\n", err)
	}

}

func (cl *CmdLine) scanBlock() (block string, err error) {
	for {
		if !cl.Scan() {
			err = cl.Err()
			if err == nil {
				err = errors.New("unexpected EOF")
			}
			return
		}
		s := strings.TrimSpace(cl.Text())
		if s == "}" {
			break
		}
		block += "\t" + s + "\n"
	}
	return
}

func (cl *CmdLine) dumpFunc(name string) {
	body, ok := cl.funcMap[name]
	if !ok {
		return
	}
	fmt.Fprintln(os.Stdout, "fn", name, "{")
	fmt.Fprint(os.Stdout, body)
	fmt.Fprintln(os.Stdout, "}")
	return
}

func (cl *CmdLine) parseFunc(name string, args []string) (err error) {
	cmd, err := cl.parseCmd(args)
	if err != nil {
		return
	}
	cl.funcMap[name] = cmd
	return
}

func (cl *CmdLine) parseCmd(f []string) (cmd string, err error) {
	if f[0] != "{" {
		cmd = "\t" + rc.Join(f) + "\n"
		return
	}
	cmd, err = cl.scanBlock()
	if err != nil {
		err = errors.New("error while parsing function body: " + err.Error())
	}
	return
}

func (cl *CmdLine) repeatCmd(arg []string) (err error) {
	i, err := strconv.ParseUint(arg[0], 10, 0)
	if err != nil {
		return
	}
	if i == 0 {
		return
	}
	cmd, err := cl.parseCmd(arg[1:])
	if err != nil {
		return
	}
	rewind := func() io.ReadCloser {
		return ioutil.NopCloser(strings.NewReader(cmd))
	}
	cl.pushStack(rewind(), int(i), rewind)
	return

}

func (cl *CmdLine) help(w io.Writer, args []string) {
	outmap := make(map[string]map[string]Cmd, 8)
	hasWritten := false
	cmdName := ""
	iDot := -1
	if len(args) > 0 {
		cmdName = args[0]
	}
	isDir := len(args) == 0
	pfx := ""
	m := cl.cmdMap
retry:
	iDot = strings.Index(cmdName, ".")

	for name, v := range m {
		if cmdName != "" {
			if name == cmdName {
				if v.Map != nil {
					pfx += cmdName + "."
					cmdName = ""
					isDir = true
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
		if pfx != "" {
			if name == "" {
				name = pfx[:len(pfx)-1]
			} else {
				name = pfx + name
			}
		}
		group := v.Group
		if group == "" {
			if cl.DefaultGroup == "" {
				group = defaultGroup
			} else {
				group = cl.DefaultGroup
			}
		}
		gm, ok := outmap[group]
		if !ok {
			gm = make(map[string]Cmd, 8)
			outmap[group] = gm
		}
		gm[name] = v
	}

	gNames := make([]string, 0, len(outmap))
	for name := range outmap {
		gNames = append(gNames, name)
	}
	sort.Strings(gNames)
	for _, gmName := range gNames {
		gm := outmap[gmName]
		if i := strings.Index(gmName, "__"); i != -1 {
			gmName = gmName[i+2:]
		}
		if len(gNames) != 1 {
			fmt.Fprintln(w, "["+gmName+"]\n")
		}

		names := make([]string, 0, len(gm))
		for name := range gm {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			v := gm[name]
			if v.Hidden && isDir {
				continue
			}
			flags := v.Flags
			if flags != "" {
				flags = " " + flags
			}
			fmt.Fprintln(w, "\t"+name+flags+argString(" ", v.Arg, "")+argString(" [", v.Opt, "]"))
			if v.Help != "" {
				for _, s := range strings.Split(v.Help, "\n") {
					fmt.Fprintln(w, "\t\t"+s)
				}
			}
			if v.Map != nil {
				fmt.Fprintf(w, "\t\tSee `help %s' for details.\n", name)
			}
			fmt.Fprint(w, "\n")
			hasWritten = true
		}
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

func (cl *CmdLine) Getenv(name string) string {
	list := cl.envStack.Get(name)
	if len(list) != 0 {
		return list[0]
	}
	return ""
}

func (cl *CmdLine) Setenv(name, value string) {
	cl.envStack.Set(name, []string{value})
}
