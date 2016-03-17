// Package rc implements string handling that mimics the style of the Rc shell.
package rc

import (
	"fmt"
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
}

type CmdLine struct {
	Assignments EnvMap
	Fields      []string
	Redir       Redirection
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
	//	fmt.Printf("TokenizeCmd: %q\n", s)
	//	dump(tokens, "	")
	if tok.Getenv != nil {
		for i, t := range tokens {
			tokens[i] = tok.expandEnv(t)
		}
	}
	c = new(CmdLine)
	c.Fields = tokens.fields()
	c.Redir = tokens.redirection()
	if nAssign != 0 {
		c.Assignments = make(EnvMap, nAssign)
		for _, t := range tokens[:nAssign] {
			a := t.(*assignmentToken)
			c.Assignments[a.name.String()] = []string{string(a.stringToken)[1:]}
		}
		c.Fields = c.Fields[nAssign:]
	}
	return
}

type token interface {
	String() string
	setString(string)
}

type stringAdder interface {
	addString(string)
}

type groupToken []token
type stringToken string
type varRefToken struct {
	stringToken
}
type assignmentToken struct {
	stringToken
	name token
}
type redirToken struct {
	*stringToken
}

func (t assignmentToken) String() string {
	return t.name.String() + string(t.stringToken)
}

func (s *stringToken) setString(arg string) {
	*s = stringToken(arg)
}
func (s *stringToken) addString(arg string) {
	*s += stringToken(arg)
}

func (s stringToken) String() string {
	return string(s)
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

func (tok *Tokenizer) expandEnv(t token) token {
	switch x := t.(type) {
	case groupToken:
		for i, sub := range x {
			x[i] = tok.expandEnv(sub)
		}
		t = mergeStringTokens(x)
	case *assignmentToken:
		x.name = tok.expandEnv(x.name)
	case *varRefToken:
		value := tok.Getenv(x.String()[1:])
		if len(value) == 0 {
			value = []string{""}
		}
		t = new(stringToken)
		t.setString(value[0])
	}
	return t
}

func mergeStringTokens(list groupToken) token {
	var prev stringAdder
	anyMerges := false

	for i, t := range list {
		if s, ok := t.(stringAdder); ok {
			if prev == nil {
				prev = s
				continue
			}
		}
		if s, ok := t.(*stringToken); ok {
			prev.addString(string(*s))
			anyMerges = true
			list[i] = nil
		} else {
			prev = nil
		}
	}
	if !anyMerges {
		return list
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

		i0 = -1

		countAssign = true
		seenAssign  = false

		t token

		setText = func(text string) {
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
		case '=':
			if seenAssign {
				err = tokenSyntaxErr(r)
				return
			}
			seenAssign = true
			flushToken(i)
			a := new(assignmentToken)
			a.name = field
			field = nil
			t = a
		case '#':
			addField(i)
			return
		default:
			if _, ok := t.(*varRefToken); ok {
				if !unicode.IsLetter(r) && r != '_' && !unicode.IsDigit(r) {
					flushToken(i)
					continue
				}
			}
			if i0 == -1 {
				i0 = i
			}
		}
	}
	addField(len(s))
	return
}

func tokenSyntaxErr(r rune) error {
	return fmt.Errorf("token '%c': syntax error", r)
}
