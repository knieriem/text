// Package rc implements string handling that mimics the style of the Rc shell.
package rc

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// An implementation of Plan 9's tokenize (see
// http://plan9.bell-labs.com/magic/man2html/2/getfields)
//
// Tokenize is similar to strings.Fields – an input string is split
// into fields separated by whitespace. Additionally, single quotes
// are interpreted and do not appear in the output. In a quoted part
// of the string, whitespace will not create a new field, and two
// consecutive single quotes will result in one quote in the output.
func Tokenize(s string) []string {
	tokens, _, _ := new(Tokenizer).do(s, false)
	return tokens.fields()
}

type Tokenizer struct {
	buf    groupToken
	Getenv func(string) []string
	Eval   func(*CmdLine) string
}

type CmdLine struct {
	Assignments EnvMap
	Fields      []string
	Redir       Redirection
}

func (c *CmdLine) String() string {
	sep := ""
	b := new(bytes.Buffer)
	n, _ := c.Assignments.WriteTo(b)
	if n != 0 {
		sep = " "
	}
	if len(c.Fields) != 0 {
		cs := JoinCmd(c.Fields)
		if cs != "" {
			fmt.Fprint(b, sep, cs)
			sep = " "
		}
	}
	if r := &c.Redir; r.Type != "" {
		fmt.Fprint(b, sep, r.Type, r.Filename)
	}
	return b.String()
}

type Redirection struct {
	Type     string
	Filename string
}

// ParseCmdLine is similar to Tokenize in that  a string is separated into fields, and
// quoted sections are recognized. It also expands variable references, if Tokenizer.Getenv
// has been set. Any assignments given at the front of a line are parsed into an EnvMap.
// On success, a CmdLine structure is returned.
func (tok *Tokenizer) ParseCmdLine(s string) (c *CmdLine, err error) {
	tokens, nAssign, err := tok.do(s, true)
	if err != nil {
		return
	}
	if false {
		fmt.Printf("TokenizeCmd: %q\n", s)
		dump(tokens, "	")
	}
	if tok.Getenv != nil {
		for i, t := range tokens {
			tokens[i] = tok.expandEnv(t)
		}
		// filter out nil tokens
		iw := 0
		for _, t := range tokens {
			if t == nil {
				continue
			}
			tokens[iw] = t
			iw++
		}
		tokens = tokens[:iw]
	}
	tokens = flattenStringLists(tokens)

	c = new(CmdLine)
	c.Fields = tokens.fields()
	c.Redir = tokens.redirection()
	if nAssign != 0 {
		c.Assignments = make(EnvMap, nAssign)
		for _, t := range tokens[:nAssign] {
			a := t.(*assignmentToken)
			c.Assignments[a.name.String()] = a.list
		}
		c.Fields = c.Fields[nAssign:]
	}
	return
}

func (tok *Tokenizer) clone() *Tokenizer {
	t := new(Tokenizer)
	t.Getenv = tok.Getenv
	t.Eval = tok.Eval
	return t
}

type token interface {
	String() string
	setString(string)
}

type stringsReceiver interface {
	fromList(*stringListToken)
	copyTo(*stringListToken)
}

type groupToken []token
type stringToken string
type varRefToken struct {
	stringToken
	isCount bool
	flat    bool
}
type assignmentToken struct {
	stringListToken
	name token
}
type redirToken struct {
	*stringToken
}

type evalToken struct {
	open bool
	stringToken
}

func (t assignmentToken) String() string {
	return t.name.String() + "=" + t.join()
}

func (s *stringToken) setString(arg string) {
	*s = stringToken(arg)
}

func (s *stringToken) addString(arg string) {
	*s += stringToken(arg)
}

func (s *stringToken) fromList(t *stringListToken) {
	*s = stringToken(t.join())
}
func (s *stringToken) copyTo(t *stringListToken) {
	t.list = []string{string(*s)}
}

func (t *stringListToken) join() string {
	return strings.Join(t.list, " ")
}

func (t *stringListToken) addStrings(list ...string) {
	if t.list == nil {
		t.list = make([]string, len(list))
	}
	if len(list) == len(t.list) {
		for i, s := range t.list {
			t.list[i] = s + list[i]
		}
	} else if len(list) == 1 {
		for i, s := range t.list {
			t.list[i] = s + list[0]
		}
	} else if len(t.list) == 1 {
		t.list, list = list, t.list
		for i, s := range t.list {
			t.list[i] = list[0] + s
		}
	} else {
		// ...
	}
}
func (t *stringListToken) fromList(t2 *stringListToken) {
	t.list = t2.list
}
func (t *stringListToken) copyTo(t2 *stringListToken) {
	t2.list = t.list
}

func (s stringToken) String() string {
	return string(s)
}

type stringListToken struct {
	list []string
}

func (t *stringListToken) String() string { return fmt.Sprintf("(%v)", t.join()) }
func (t *stringListToken) setString(s string) {
	t.list = []string{s}
}

func flattenStringLists(list groupToken) groupToken {
	n := 0
	for _, tok := range list {
		if t, ok := tok.(*stringListToken); ok {
			n += len(t.list)
		} else {
			n += 1
		}
	}
	dest := make(groupToken, 0, n)
	for _, tok := range list {
		if t, ok := tok.(*stringListToken); ok {
			for _, s := range t.list {
				ts := new(stringToken)
				ts.setString(s)
				dest = append(dest, ts)
			}
		} else {
			dest = append(dest, tok)
		}
	}
	return dest
}

func (list groupToken) String() (s string) {
	for _, tok := range list {
		s += tok.String()
	}
	return
}
func (groupToken) setString(_ string) {}

func (list groupToken) fields() (f []string) {
	for _, t := range list {
		if _, ok := t.(*redirToken); ok {
			break
		}
		f = append(f, t.String())
	}
	return
}

func (list groupToken) redirection() Redirection {
	var r Redirection
	inRedir := false
	for _, t := range list {
		if inRedir {
			r.Filename = t.String()
			break
		}
		if _, ok := t.(*redirToken); ok {
			inRedir = true
			r.Type = t.String()
		}
	}
	return r
}

func dump(list groupToken, indent string) {
	for _, t := range list {
		if sub, ok := t.(groupToken); ok {
			fmt.Printf("%s%T\n", indent, t)
			dump(sub, indent+"\t")
		} else {
			fmt.Printf("%s%T %v\n", indent, t, t)
		}
	}
}

var argrefRE = regexp.MustCompile("^[1-9][0-9]*$")
var arridxRE = regexp.MustCompile(`\(([0-9]*)\)$`)

func (tok *Tokenizer) expandEnv(t token) token {
	switch x := t.(type) {
	case groupToken:
		for i, sub := range x {
			x[i] = tok.expandEnv(sub)
		}
		t = mergeStringTokens(x)
	case *assignmentToken:
		x.name = tok.expandEnv(x.name)
	case *evalToken:
		c, err := tok.clone().ParseCmdLine(string(x.stringToken))
		if err != nil {
			return nil
		}
		if tok.Eval == nil {
			return nil
		}
		output := tok.Eval(c)
		return &stringListToken{list: strings.Fields(output)}
	case *varRefToken:
		ref := x.String()[1:]
		i := -1
		if x.isCount {
			ref = ref[1:]
			value := tok.Getenv(ref)
			t.setString(strconv.Itoa(len(value)))
			break
		}
		if argrefRE.MatchString(ref) {
			i, _ = strconv.Atoi(ref)
			i--
			ref = "*"
		} else if si := arridxRE.FindStringSubmatchIndex(ref); len(si) == 4 {
			index := ref[si[2]:si[3]]
			if index == "0" || index == "" {
				t.setString("")
				break
			}
			i, _ = strconv.Atoi(index)
			i--
			ref = ref[:si[0]]
		}
		value := tok.Getenv(ref)
		t = new(stringToken)
		if i == -1 {
			switch len(value) {
			case 0:
				return nil
			case 1:
				t.setString(value[0])
			default:
				if x.flat {
					t.setString(strings.Join(value, " "))
					break
				}
				t = &stringListToken{list: value}
			}
		} else if len(value) <= i {
			t.setString("")
		} else {
			t.setString(value[i])
		}
	}
	return t
}

func mergeStringTokens(list groupToken) token {
	var prev stringsReceiver
	var buf *stringListToken
	anyMerges := false

	for i, t := range list {
		if prev == nil {
			if sr, ok := t.(stringsReceiver); ok {
				prev = sr
				buf = new(stringListToken)
				sr.copyTo(buf)
				continue
			}
		}

		if prev == nil {
			continue
		}

		switch x := t.(type) {
		case *stringToken:
			buf.addStrings(string(*x))
			anyMerges = true
			list[i] = nil
		case *stringListToken:
			buf.addStrings(x.list...)
			anyMerges = true
			list[i] = nil
		default:
			prev.fromList(buf)
			prev = nil
		}
	}
	if !anyMerges {
		return list
	}
	if prev != nil {
		prev.fromList(buf)
	}
	dest := make(groupToken, 0, len(list))
	for _, t := range list {
		if t != nil {
			dest = append(dest, t)
		}
	}
	if len(dest) == 1 {
		return dest[0]
	}
	return dest
}

func (tok *Tokenizer) do(s string, handleSpecial bool) (fields groupToken, nAssign int, err error) {
	var (
		field   groupToken
		quoting = false
		wasq    = false

		evalDepth = 0

		i0 = -1

		countAssign = true
		seenAssign  = false

		t token

		setText = func(text string) {
			if text == "" {
				return
			}
			if t == nil {
				if len(field) != 0 {
					if st, ok := field[len(field)-1].(*stringToken); ok {
						st.addString(text)
						return
					}
				}
				t = new(stringToken)
			}
			t.setString(text)
		}
		addField = func(iPos int) {
			if i0 == -1 {
				return
			}
			if countAssign {
				if seenAssign {
					nAssign++
					seenAssign = false
				} else {
					countAssign = false
				}
			}
			if setText(s[i0:iPos]); t != nil {
				if field == nil {
					fields = append(fields, t)
				} else {
					field = append(field, t)
				}
			}
			if field != nil {
				if len(field) == 1 {
					fields = append(fields, field[0])
				} else {
					fields = append(fields, field)
				}
			}
			field = nil
			t = nil
			i0 = -1
		}

		flushToken = func(iPos int) {
			defer func() { i0 = iPos }()
			if i0 == -1 {
				return
			}
			if setText(s[i0:iPos]); t != nil {
				field = append(field, t)
				t = nil
			}
		}
	)

	fields = tok.buf[:0]

	for i, r := range s {
		if r == '\'' {
			if !quoting {
				if wasq {
					i0--
					wasq = false
				}
				quoting = true
			} else {
				quoting = false
				wasq = true
			}
			flushToken(i)
			i0 = i + 1
			continue
		}
		wasq = false
		if quoting {
			continue
		}

		if eval, ok := t.(*evalToken); ok {
			if !eval.open {
				if r != '{' {
					err = tokenSyntaxErr(r)
					return
				}
				i0 = i + 1
				eval.open = true
			}
			if r == '`' {
				tail := s[i+1:]
				if len(tail) == 0 {
					err = tokenSyntaxErr(r)
					return
				}
				if c := tail[0]; c == '{' {
					evalDepth++
				}
			}
			if r == '}' {
				if evalDepth == 0 {
					flushToken(i)
					i0 = i + 1
				} else {
					evalDepth--
				}
			}
			continue
		}

		switch r {
		case ' ', '\t', '\r', '\n':
			addField(i)
			continue
		}
		if !handleSpecial {
			if i0 == -1 {
				i0 = i
			}
			continue
		}

		switch r {
		case '<', '>':
			if _, ok := t.(*redirToken); ok {
				break
			}
			addField(i)
			t = &redirToken{
				stringToken: new(stringToken),
			}
			i0 = i
		case '$':
			flushToken(i)
			t = new(varRefToken)
		case '^':
			if i0 == -1 {
				if fields == nil {
					err = tokenSyntaxErr(r)
					return
				}
				iLast := len(fields) - 1
				tPrev := fields[iLast]
				if g, ok := tPrev.(groupToken); ok {
					field = g
				} else {
					field = groupToken{tPrev}
				}
				fields = fields[:iLast]
			}
			flushToken(i)
			i0++
		case '#':
			if ref, ok := t.(*varRefToken); ok {
				if ref.isCount {
					err = tokenSyntaxErr(r)
					return
				}
				ref.isCount = true
				break
			}
			addField(i)
			return
		case '"':
			if ref, ok := t.(*varRefToken); ok {
				if ref.flat {
					err = tokenSyntaxErr(r)
					return
				}
				ref.flat = true
				i0 = i
				break
			}
			addField(i)
			return
		case '`':
			flushToken(i)
			t = new(evalToken)

		case '=':
			if _, ok := t.(*assignmentToken); !ok && countAssign && !seenAssign && i0 != -1 {
				seenAssign = true
				flushToken(i)
				i0 = i + 1
				a := new(assignmentToken)
				a.name = field
				field = nil
				t = a
				break
			}
			fallthrough
		default:
			if _, ok := t.(*varRefToken); ok {
				if !unicode.IsLetter(r) && r != '_' && !unicode.IsDigit(r) && r != '*' && r != '(' && r != ')' {
					flushToken(i)
					continue
				}
			}
			if i0 == -1 {
				i0 = i
			}
		}
	}
	if _, ok := t.(*evalToken); ok {
		err = errors.New("missing closing }")
		return
	}
	addField(len(s))
	return
}

func tokenSyntaxErr(r rune) error {
	return fmt.Errorf("token '%c': syntax error", r)
}
