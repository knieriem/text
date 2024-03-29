package interp

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode"

	gioutil "github.com/knieriem/g/ioutil"
	"github.com/knieriem/text"
	"github.com/knieriem/text/rc"
)

const (
	defaultGroup = "ZZY__Other commands"
)

type Cmd struct {
	Map         CmdMap
	Fn          func(_ Context, arg []string) error
	Arg         []string
	Opt         []string
	Help        string
	Hidden      bool
	Group       string
	Flags       string
	InitFlags   func(f *flag.FlagSet)
	ignoreEnv   bool
	HideFailure bool
	weakStatus  bool
	isCompound  bool
}

type CmdMap map[string]*Cmd

type Context interface {
	text.Writer
	context.Context
	Getenv(string) string
}

type icontext struct {
	text.Writer
	context.Context
	getenv func(string) string
}

func (ictx *icontext) Getenv(s string) string {
	return ictx.getenv(s)
}

type CmdLine struct {
	*cmdLineReader
	cur         stackEntry
	inputStack  []stackEntry
	lastOk      bool
	savedPrompt string
	tok         *rc.Tokenizer
	env         *Env
	tplMap      *templateMap

	cmdMap  CmdMap
	builtin CmdMap
	funcMap map[string]string
	InitRc  io.ReadCloser
	flags   struct {
		e bool
		x bool
	}
	ExtraHelp    func()
	DefaultGroup string
	Prompt       string
	WritePrompt  func(string) error

	// Stdout is used for writing normal output.
	// It is initialized with os.Stdout.
	//
	// Deprecated: Instead of setting Stdout directly,
	// use the WithStdout option.
	Stdout io.Writer
	errOut io.Writer

	Forward     io.Writer
	printCmd    func(*rc.CmdLine)
	handleError func(err error)
	Open        func(filename string) (io.ReadCloser, error)
	cmdHook     CmdHookFunc

	cIntr         chan struct{}
	exitFlag      bool
	OpenRedirFile func(name string, flag int, perm os.FileMode) (RedirFile, error)
	redirFileMap  map[string]RedirFile
}

type RedirFile interface {
	io.WriteCloser
	io.Seeker
	Truncate(size int64) error
}

type cmdLineReader struct {
	text.Scanner
	io.Closer
}

func newCmdLineReader(s text.Scanner, c io.Closer) *cmdLineReader {
	return &cmdLineReader{s, c}
}

type Option func(cl *CmdLine)

func WithStdout(w io.Writer) Option {
	return func(cl *CmdLine) {
		cl.Stdout = w
	}
}

func WithStderr(w io.Writer) Option {
	return func(cl *CmdLine) {
		cl.errOut = w
	}
}

func WithEnv(e *Env) Option {
	return func(cl *CmdLine) {
		cl.env = e
	}
}

type Env struct {
	stack rc.EnvStack
}

func NewEnv() *Env {
	env := new(Env)
	env.stack.Push(rc.EnvMap{
		"prefix": nil,
		"OFS":    []string{" "},
		"0":      []string{"rc"},
	})
	return env
}

func (env *Env) Getenv(name string) string {
	list := env.stack.Get(name)
	if len(list) != 0 {
		return list[0]
	}
	return ""
}

func (env *Env) Setenv(name, value string) {
	env.stack.Set(name, []string{value})
}

type CmdHookFunc func(Context)

// WithCmdHook registers a function that is called each time
// before a command is called. The context value in the first
// function argument of the hook function is the same the
// command will see. The command hook might be used, for example,
// to configure a service underlying multiple commands in cases
// where doing it different is not easily possible.
func WithCmdHook(f CmdHookFunc) Option {
	return func(cl *CmdLine) {
		cl.cmdHook = f
	}
}

func NewCmdInterp(s text.Scanner, m CmdMap, opts ...Option) (cl *CmdLine) {
	cl = new(CmdLine)
	cl.cmdLineReader = newCmdLineReader(s, nil)
	cl.cur.lineReader = cl.cmdLineReader
	cl.funcMap = make(map[string]string)
	cl.cmdMap = m
	cl.builtin = CmdMap{
		".": {
			Arg: []string{"FILE"},
			Fn: func(ctx Context, arg []string) (err error) {
				f, err := cl.Open(arg[1])
				if err == nil {
					cl.pushStack(f, nil, nil, extractWriter(ctx))
				}
				return
			},
			Help:      "Read commands from FILE.",
			ignoreEnv: true,
		},
		"echo": {
			Opt: []string{"ARG", "..."},
			Fn: func(w Context, arg []string) (err error) {
				arg2 := make([]string, 0, len(arg))
				for _, a := range arg[1:] {
					if a != "" {
						arg2 = append(arg2, a)
					}
				}
				_, err = w.PrintSlice(arg2)
				return
			},
			Help: "Print arguments.",
		},
		"cat": {
			Arg: []string{"FILE"},
			Fn: func(w Context, arg []string) (err error) {
				f, err := cl.Open(arg[1])
				if err != nil {
					return err
				}
				_, err = io.Copy(w, f)
				f.Close()
				return err
			},
			Help: "Print the contents of FILE.",
		},
		"if": {
			isCompound: true,
			Arg:        []string{"CMD", "..."},
			Fn: func(ctx Context, arg []string) (err error) {
				cmd, err := cl.ParseCmd(arg[len(arg)-1:])
				if err != nil {
					return
				}
				w := extractWriter(ctx)
				if arg[1] == "not" {
					if cl.cur.cond.result == nil {
						err = errors.New("`if not' does not follow `if'")
						return
					}
					if !*cl.cur.cond.result {
						cl.pushStringStack(cmd, w)
					}
					return
				}
				cond := rc.JoinCmd(arg[1:len(arg)-1]) + "\n" + "_testcond\n"
				cl.pushStringStack(cond, w)
				cl.cur.cond.cmd = cmd
				cl.cur.isCompound = true
				return nil
			},
		},
		"_testcond": {
			Hidden: true,
			Fn: func(ctx Context, _ []string) (err error) {
				cond := &cl.cur.cond
				cmd := cond.cmd
				if cmd == "" {
					return
				}
				cond.cmd = ""
				ok := cl.lastOk
				cl.inputStack[len(cl.inputStack)-1].cond.result = &ok
				if ok {
					cl.pushStringStack(cmd, extractWriter(ctx))
				}
				return nil
			},
		},
		"!": {
			isCompound:  true,
			HideFailure: true,
			Opt:         []string{"CMD", "..."},
			Fn: func(ctx Context, arg []string) (err error) {
				if len(arg) == 1 {
					return errors.New("false")
				}
				cmd := rc.JoinCmd(arg[1:]) + "\n" + "_!\n"
				cplx := cl.cur.isCompound
				cl.pushStringStack(cmd, extractWriter(ctx))
				cl.cur.isCompound = cplx
				return nil
			},
		},
		"_!": {
			Hidden:      true,
			HideFailure: true,
			Fn: func(Context, []string) (err error) {
				if cl.lastOk {
					err = errors.New("false")
				}
				return
			},
		},
		"~": {
			HideFailure: true,
			Arg:         []string{"SUBJECT", "PATTERN", "..."},
			Fn: func(w Context, arg []string) error {
				subject := arg[1]
				for _, pat := range arg[2:] {
					match, err := path.Match(pat, subject)
					if err != nil {
						return err
					}
					if match {
						return nil
					}
				}
				return errors.New("no match")
			},
			Help: `Returns success if subject matches any pattern.`,
		},

		"flag": {
			Arg: []string{"f", "+-"},
			Fn: func(ctx Context, arg []string) (err error) {
				f := arg[1]
				v := arg[2] == "+"
				switch f {
				default:
					return fmt.Errorf("unknown flag %q", f)
				case "e":
					cl.flags.e = v
				case "x":
					cl.flags.x = v
				}
				return nil
			},
			Help: `Set a flag as in Plan 9's rc:
	e	exit if a simple command (not part of an if-condition) fails`,
		},
		"fn": {
			isCompound: true,
			Opt:        []string{"NAME", "CMD", "..."},
			Fn: func(w Context, arg []string) error {
				switch len(arg) {
				case 1:
					for name := range cl.funcMap {
						cl.dumpFunc(w, name)
					}
					return nil
				case 2:
					cl.dumpFunc(w, arg[1])
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
		"shift": {
			Opt: []string{"N"},
			Fn: func(_ Context, arg []string) error {
				i := 1
				if len(arg) == 2 {
					u, err := strconv.ParseUint(arg[1], 10, 0)
					if err != nil {
						return err
					}
					i = int(u)
				}
				args := cl.env.stack.Get("*")
				if i > len(args) {
					i = len(args)
				}
				cl.env.stack.Set("*", args[i:])
				return nil
			},
			Help: "Delete the first n (default: 1) elements of $*",
		},
		"unbind": {
			Arg: []string{"NAME"},
			Fn: func(_ Context, arg []string) (err error) {
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
			Arg: []string{"{N|T}", "CMD"},
			Opt: []string{"ARG", "..."},
			Fn: func(ctx Context, arg []string) error {
				return cl.repeatCmd(extractWriter(ctx), arg[1:])
			},
			Help: "Repeat a command N times, or for a specified duration T.",
		},
		"return": {
			Fn: func(_ Context, _ []string) error {
				return cl.returnFromFunc()
			},
			weakStatus: true,
			Help:       "Return from the current function.",
		},
		"break": {
			Fn: func(_ Context, _ []string) error {
				return cl.breakLoop()
			},
			weakStatus: true,
			Help:       "Exit the current loop.",
		},
		"false": {
			Fn: func(_ Context, _ []string) error {
				return errors.New("false")
			},
			HideFailure: true,
			Help:        "Return an exit status indicating failure",
		},
		"sleep": {
			Fn: func(ctx Context, arg []string) (err error) {
				tArg, err := time.ParseDuration(arg[1])
				if err != nil {
					return
				}
				t := time.NewTimer(tArg)
				select {
				case <-t.C:
				case <-ctx.Done():
					t.Stop()
					err = ErrInterrupt
				}
				return
			},
			Arg:  []string{"DURATION"},
			Help: "Sleep for the specified duration.",
		},
		"exit": {
			Fn: func(Context, []string) error {
				cl.exitFlag = true
				return nil
			},
			Help: "Terminate the command line processor.",
		},
	}
	if _, ok := m["builtin"]; !ok {
		m["builtin"] = &Cmd{
			Map:  cl.builtin,
			Help: "Built-in commands.\nMay be called without the `builtin.' prefix.",
		}
	}

	cl.Stdout = os.Stdout
	cl.errOut = os.Stderr
	cl.Open = func(filename string) (io.ReadCloser, error) {
		return os.Open(filename)
	}
	cl.OpenRedirFile = func(name string, flag int, perm os.FileMode) (RedirFile, error) {
		return os.OpenFile(name, flag, perm)
	}
	cl.WritePrompt = func(prompt string) error {
		if prompt == "" {
			return nil
		}
		_, err := cl.Stdout.Write([]byte(prompt))
		return err
	}
	cl.printCmd = func(cmd *rc.CmdLine) {
		fmt.Fprintf(cl.Stdout, "%% %v\n", cmd)
	}
	cl.handleError = func(err error) {
		fmt.Fprintln(cl.errOut, err)
	}
	cl.cIntr = make(chan struct{})
	cl.tok = new(rc.Tokenizer)

	for _, option := range opts {
		option(cl)
	}
	if cl.env == nil {
		cl.env = NewEnv()
	}
	cl.tok.Getenv = func(key string) []string {
		return cl.env.stack.Get(key)
	}
	cl.lastOk = true
	return cl
}

func extractWriter(ctx Context) text.Writer {
	return ctx.(*icontext).Writer
}

func (cl *CmdLine) cleanup() {
	for _, file := range cl.redirFileMap {
		file.Close()
	}
}

func (cl *CmdLine) redirect(op string, filename string) (text.Writer, error) {
	var err error

	if m := cl.redirFileMap; m == nil {
		cl.redirFileMap = make(map[string]RedirFile, 16)
	}
	file := cl.redirFileMap[filename]
	owflags := os.O_CREATE | os.O_RDWR
	switch op {
	case ">":
		if file != nil {
			file.Seek(0, 0)
			file.Truncate(0)
			goto opened
		}
		owflags |= os.O_TRUNC
	case ">>":
		if file != nil {
			goto opened
		}
		owflags |= os.O_APPEND
	default:
		return nil, errors.New("redirection type not supported")
	}
	file, err = cl.OpenRedirFile(filename, owflags, 0644)
	if err != nil {
		return nil, err
	}
	cl.redirFileMap[filename] = file
opened:
	w := cl.newWriter(file)
	return w, nil
}

func (cl *CmdLine) Interrupt(timeout time.Duration) (ok bool) {
	t := time.NewTimer(timeout)
	select {
	case <-t.C:
		return
	case cl.cIntr <- struct{}{}:
	}
	t.Stop()
	ok = true
	return
}

type stackEntry struct {
	lineReader *cmdLineReader
	repetition *repetition
	rewind     func() io.ReadCloser

	w          text.Writer
	popEnv     bool
	savedArgs  []string
	isFunc     bool
	isCompound bool
	cond       struct {
		cmd    string
		result *bool
	}
}

func (stk *stackEntry) isLoop() bool {
	return stk.repetition != nil
}

func (cl *CmdLine) pushStack(rc io.ReadCloser, rpt *repetition, rewind func() io.ReadCloser, w text.Writer) {
	cl.inputStack = append(cl.inputStack, cl.cur)
	cl.cur = stackEntry{
		lineReader: newCmdLineReader(bufio.NewScanner(rc), rc),
		repetition: rpt,
		rewind:     rewind,
		w:          w,
	}
	cl.cmdLineReader = cl.cur.lineReader
	if cl.Prompt != "" {
		cl.savedPrompt = cl.Prompt
		cl.Prompt = ""
	}
}

func (cl *CmdLine) pushStringStack(cmds string, w text.Writer) {
	cl.pushStack(ioutil.NopCloser(strings.NewReader(cmds)), nil, nil, w)
}

func (cl *CmdLine) popStack() {
	if cl.cur.popEnv {
		cl.env.stack.Pop()
	}
	if a := cl.cur.savedArgs; a != nil {
		cl.env.stack.Set("*", a)
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

func (cl *CmdLine) breakLoop() error {
	isLoop := cl.cur.isLoop()
	for {
		if len(cl.inputStack) == 0 || cl.cur.isFunc {
			if !isLoop {
				if cl.cur.isFunc {
					cl.popStackAll()
				}
				return errors.New("not within a loop")
			}
			return nil
		}
		cl.popStack()
		if isLoop {
			return nil
		}
		isLoop = cl.cur.isLoop()
	}
}

func (cl *CmdLine) returnFromFunc() error {
	for {
		if cl.cur.isFunc {
			cl.popStack()
			return nil
		}
		if len(cl.inputStack) == 0 {
			return errors.New("not within a function")
		}
		cl.popStack()
	}
}

var ErrInterrupt = errors.New("interrupted")
var ErrLastCmdFailed = errors.New("last command failed")

var ErrWrongNArg = errors.New("wrong number of arguments")
var ErrNotFound = errors.New("no such command")

type FnError struct {
	Fn  string
	err error
}

func (e *FnError) Error() string {
	return fmt.Sprintf("%s: %v", e.Fn, e.err)
}
func (e *FnError) Unwrap() error {
	return e.err
}

func (cl *CmdLine) setError(err error) {
	if h := cl.handleError; h != nil {
		h(err)
	}
}
func (cl *CmdLine) setFnError(fnName string, err error) {
	if h := cl.handleError; h != nil {
		if fnName != "" {
			err = &FnError{Fn: fnName, err: err}
		}
		h(err)
	}
	cl.lastOk = false
	if cl.flags.e && !cl.cur.isCompound {
		cl.exitFlag = true
	}
}

func (cl *CmdLine) Process() error {
	var line string

	cl.tplMap = newTemplateMap(16)
	cl.cur.w = cl.newWriter(cl.Stdout)
	ready := make(chan bool)

	defer cl.cleanup()

	if cl.InitRc != nil {
		cl.pushStack(cl.InitRc, nil, nil, cl.cur.w)
	}

	var ictx *icontext
	for {
		if cl.exitFlag {
			break
		}
		cl.WritePrompt(cl.Prompt)
		go func() {
			ready <- cl.Scan()
		}()
		scanOk := false
	selAgain:
		if ictx == nil {
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			go func() {
				<-cl.cIntr
				cancel()
			}()
			ictx = new(icontext)
			ictx.Context = ctx
			ictx.getenv = cl.env.Getenv
		}
		select {
		case <-ictx.Done():
			ictx = nil
			if len(cl.inputStack) == 0 {
				return ErrInterrupt
			} else {
				cl.setError(ErrInterrupt)
				cl.popStackAll()
				cl.WritePrompt(cl.Prompt)
				goto selAgain
			}
		default:
		}
		select {
		case <-ictx.Done():
			ictx = nil
			if len(cl.inputStack) == 0 {
				return ErrInterrupt
			} else {
				cl.setError(ErrInterrupt)
				cl.popStackAll()
				cl.WritePrompt(cl.Prompt)
				goto selAgain
			}
		case scanOk = <-ready:
		}

		if !scanOk {
			err := cl.Err()
			if err == nil {
				if sz := len(cl.inputStack); sz != 0 {
					if !cl.cur.repetition.done() {
						rc := cl.cur.rewind()
						cl.cur.lineReader = newCmdLineReader(bufio.NewScanner(rc), rc)
						cl.cmdLineReader = cl.cur.lineReader
						continue
					}
					cl.popStack()
					continue
				}
				if !cl.lastOk {
					err = ErrLastCmdFailed
				}
			}
			return err
		}
		line = cl.Text()
		if cl.Prompt != "" {
		again:
			if strings.HasPrefix(line, cl.Prompt) {
				line = line[len(cl.Prompt):]
				goto again
			}
		}
		w := cl.cur.w
		c, err := cl.tok.ParseCmdLine(line)
		if err != nil {
			cl.setFnError("", err)
			continue
		}
		if c.Redir.Type != "" {
			w, err = cl.redirect(c.Redir.Type, c.Redir.Filename)
			if err != nil {
				cl.setFnError("", err)
				continue
			}
		}
		args := c.Fields
		if len(args) == 0 {
			if a := c.Assignments; len(a) != 0 {
				if cl.flags.x {
					cl.printCmd(c)
				}
				cl.env.stack.Insert(a)
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
				cl.env.stack.Push(c.Assignments)
			}
			cl.pushStringStack(body, w)
			if privEnv {
				cl.cur.popEnv = true
			} else {
				cl.cur.savedArgs = cl.env.stack.Get("*")
			}
			cl.env.stack.Set("*", args[1:])
			cl.cur.isFunc = true
			if cl.flags.x {
				cl.printCmd(c)
			}
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
		isRoot := true
		cmdName := name

	retry:
		cmd, ok := m[cmdName]
		if !ok && isRoot {
			cmd, ok = cl.builtin[cmdName]
		}
		if !ok {
			if iDot := strings.Index(cmdName, "."); iDot != -1 {
				if cmd, ok = m[cmdName[:iDot]]; ok {
					m = cmd.Map
					if m != nil {
						cmdName = cmdName[iDot+1:]
						isRoot = false
						goto retry
					}
				}
			}
			if cl.Forward != nil {
				cl.fwd([]byte(rc.JoinCmd(args) + "\n"))
			} else {
				cl.setFnError(name, ErrNotFound)
			}
			continue
		}
		if cmd.Map != nil {
			if cmd, ok = cmd.Map[""]; !ok {
				cl.setFnError(name, ErrNotFound)
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
			cl.setFnError(name, ErrWrongNArg)
			continue
		}
	checkNMin:
		if n < nmin {
			cl.setFnError(name, ErrWrongNArg)
			continue
		}
		if privEnv {
			if !cmd.ignoreEnv {
				cl.env.stack.Push(c.Assignments)
			}
		}
		ictx.Writer = w
		if cl.cmdHook != nil {
			cl.cmdHook(ictx)
		}
		if cl.flags.x && !cmd.Hidden && !cmd.isCompound {
			cl.printCmd(c)
		}
		err = cmd.Fn(ictx, args)
		select {
		case <-ictx.Done():
			if err == nil {
				err = ErrInterrupt
			}
			ictx = nil
		default:
		}
		if !cmd.weakStatus {
			cl.lastOk = err == nil
		}
		cl.cur.cond.result = nil
		if cmd.HideFailure {
			err = nil
		}
		if privEnv {
			cl.env.stack.Pop()
		}
		if err != nil {
			if errors.Is(err, context.Canceled) || err == ErrInterrupt {
				err = ErrInterrupt
				cl.popStackAll()
			}
			cl.setFnError(name, err)
		}
	}
	if cl.flags.e {
		if !cl.lastOk {
			return ErrLastCmdFailed
		}
	}
	return nil
}

func (cl *CmdLine) fwd(line []byte) {
	_, err := cl.Forward.Write(line)
	if err != nil {
		cl.setError(fmt.Errorf("forwarding write failed: %w", err))
	}

}

func (cl *CmdLine) scanBlock() (block string, err error) {
	for {
		cl.WritePrompt("")
		if !cl.Scan() {
			err = cl.Err()
			if err == nil {
				err = errors.New("unexpected EOF")
			}
			return
		}
		s := strings.TrimRightFunc(cl.Text(), unicode.IsSpace)
		if s == "}" {
			break
		}
		s = strings.TrimPrefix(s, "\t")
		block += s + "\n"
	}
	return
}

func (cl *CmdLine) dumpFunc(_ text.Writer, name string) {
	body, ok := cl.funcMap[name]
	if !ok {
		return
	}
	fmt.Fprintln(cl.Stdout, "fn", name, "{")
	inw := gioutil.NewIndentWriter(cl.Stdout, []byte{'\t'})
	fmt.Fprint(inw, body)
	fmt.Fprintln(cl.Stdout, "}")
}

func (cl *CmdLine) parseFunc(name string, args []string) (err error) {
	cmd, err := cl.ParseCmd(args)
	if err != nil {
		return
	}
	cl.funcMap[name] = cmd
	return
}

func (cl *CmdLine) ParseCmd(f []string) (cmd string, err error) {
	if f[0] != "{" {
		cmd = "\t" + rc.JoinCmd(f) + "\n"
		return
	}
	cmd, err = cl.scanBlock()
	if err != nil {
		err = errors.New("error while parsing function body: " + err.Error())
	}
	return
}

type repetition struct {
	n   int
	end time.Time
}

func (r *repetition) done() bool {
	if r == nil {
		return true
	}
	if r.n > 1 {
		r.n--
		return false
	}
	if r.n == 0 {
		if !r.end.IsZero() {
			return time.Now().After(r.end)
		}
	}
	return true
}

func (cl *CmdLine) repeatCmd(w text.Writer, arg []string) (err error) {
	var i uint64
	var d time.Duration

	d, err = time.ParseDuration(arg[0])
	if err != nil {
		i, err = strconv.ParseUint(arg[0], 10, 0)
		if err != nil {
			return
		}
	}
	if i == 0 && d == 0 {
		return
	}
	cmd, err := cl.ParseCmd(arg[1:])
	if err != nil {
		return
	}
	rewind := func() io.ReadCloser {
		return ioutil.NopCloser(strings.NewReader(cmd))
	}
	r := &repetition{
		n:   int(i),
		end: time.Now().Add(d),
	}
	cl.pushStack(rewind(), r, rewind, w)
	return

}

func (cl *CmdLine) help(w io.Writer, args []string) {
	outmap := make(map[string]CmdMap, 8)
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
			gm = make(CmdMap, 8)
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
		cl.setFnError(args[0], ErrNotFound)
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

type writer struct {
	io.Writer
	fieldSep func() string
	prefix   func() string
}

func (cl *CmdLine) newWriter(w io.Writer) *writer {
	var b bytes.Buffer
	get := func(name string) string {
		q := cl.env.Getenv(name)
		q = strings.Replace(q, `"`, `\"`, -1)
		s, err := strconv.Unquote(`"` + q + `"`)
		if err != nil {
			return "getenv: unquote: err.Error()"
		}
		return s
	}
	return &writer{
		Writer: w,
		fieldSep: func() string {
			return get("OFS")
		},
		prefix: func() string {
			t, err := cl.tplMap.Get("$prefix", get("prefix"))
			if err != nil {
				return "<" + err.Error() + ">"
			}
			b.Reset()
			err = t.Execute(&b, nil)
			if err != nil {
				return "<" + err.Error() + ">"
			}
			return b.String()
		},
	}
}

func (w *writer) Printf(format string, arg ...interface{}) (n int, err error) {
	s := fmt.Sprintf(format, arg...)
	return w.print(s + "\n")
}

func (w *writer) Println(arg ...interface{}) (n int, err error) {
	return w.print(fmt.Sprintln(arg...))
}

func (w *writer) PrintSlice(args []string) (n int, err error) {
	return w.print(strings.Join(args, w.fieldSep()) + "\n")
}

func (w *writer) print(s string) (n int, err error) {
	return w.Write([]byte(w.prefix() + s))
}

type templateMap struct {
	t0   time.Time
	m    map[string]*template.Template
	nMax int
}

func newTemplateMap(nMax int) *templateMap {
	return &templateMap{
		t0:   time.Now(),
		m:    make(map[string]*template.Template, nMax),
		nMax: nMax,
	}
}

func (tm *templateMap) Get(name, def string) (*template.Template, error) {
	t, ok := tm.m[def]
	if ok {
		return t, nil
	}
	t = template.New(name)
	t.Funcs(template.FuncMap{
		"div": func(dividend, divisor int64) int64 {
			return dividend / divisor
		},
		"now": func() time.Time {
			return time.Now()
		},
		"t0": func() time.Time {
			return tm.t0
		},
	})
	t, err := t.Parse(def)
	if err != nil {
		return nil, err
	}
	tm.m[def] = t
	return t, nil
}
