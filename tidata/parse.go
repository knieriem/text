// A parser for tab indented trees of structured data.
//
//	Doc	:=	{ Elem | \n | Comment }
//
//	Elem	:=	N*\t Key [ \t Value ] \n
//			{ (N+1)*\t Elem \n }
//
//	Comment	:=	{ \t } CommentPfx \n
//
// Whitespace surrounding a Key/Value pair will be stripped. The comment prefix
// can be configured.
package tidata

import (
	"strings"

	"github.com/knieriem/text"
	"github.com/knieriem/text/line"
)

type Reader struct {
	CommentPrefix        string
	CommentPrefixEscaped string
	TrimPrefix           string
	StripUtf8BOM         bool

	s    text.Scanner
	errC chan error
}

func NewReader(s text.Scanner) *Reader {
	return &Reader{s: s}
}

type input struct {
	insert  bool // if false: report current list of elements to parent
	line    string
	lineNum int
}

// Parse a whole file into atree structure of Elems and return a pointer
// to the root Elem.
func (r *Reader) ReadAll() (top *Elem, err error) {

	sub := make(chan input)
	rsub := make(chan []Elem)
	r.errC = make(chan error, 4)
	go r.handleLevel(sub, rsub)
	defer func() {
		if err != nil {
			l := new(line.ErrorList)
			l.Add(err)
			err = l
		}
		close(sub)
	}()

	nTrimPrefix := len(r.TrimPrefix)

	for n := 1; r.s.Scan(); n++ {
		line := r.s.Text()
		if n == 1 && r.StripUtf8BOM {
			if strings.HasPrefix(line, "\uFEFF") {
				line = line[3:]
			}
		}

		if nTrimPrefix != 0 {
			if strings.HasPrefix(line, r.TrimPrefix) {
				line = line[nTrimPrefix:]
			}
		}
		if len(line) > 0 {
			select {
			case sub <- input{insert: true, line: line, lineNum: n}:
			case err = <-r.errC:
				if err != nil {
					return
				}
			}
		}
	}
	err = r.s.Err()
	if err != nil {
		return
	}
	sub <- input{}
	top = new(Elem)
	top.Children = <-rsub

	return
}

func (r *Reader) handleLevel(inCh <-chan input, ret chan<- []Elem) {
	var (
		list = make([]Elem, 0, 16)
		el   *Elem

		sub  chan input
		rsub chan []Elem
	)

	requestChildren := func() []Elem {
		sub <- input{}
		return <-rsub
	}

	for in := range inCh {
		if !in.insert {
			// if there is a current element, update
			// the list of its children
			if el != nil && sub != nil {
				el.Children = requestChildren()
			}
			// report current list of elements to parent
			ret <- list
			list = list[len(list):]
			el = nil
			continue
		}
		if len(in.line) > 0 {
			if in.line[0] == '\t' {
				if el == nil {
					r.errC <- line.NewMsg(in.lineNum, "wrong depth")
				}
				if len(list) > 0 {
					// input is not for me, propagate it to sub handler
					if sub == nil {
						sub = make(chan input)
						rsub = make(chan []Elem)
						go r.handleLevel(sub, rsub)
					}
					sub <- input{insert: true, line: in.line[1:], lineNum: in.lineNum}
				}
				continue
			}
			// escaped comment?
			if r.CommentPrefix != "" {
				if strings.HasPrefix(in.line, r.CommentPrefixEscaped) {
					in.line = in.line[1:]
				} else if strings.HasPrefix(in.line, r.CommentPrefix) { // comment?
					continue
				}
			}
		}
		if el != nil && sub != nil {
			// update the current element's list of children
			el.Children = requestChildren()
		}
		// create new element from input
		s := in.line
		if n := len(s); n != 0 {
			c0, cLast := in.line[0], in.line[n-1]
			if c0 == ' ' {
				r.errC <- line.NewMsg(in.lineNum, "extra space character near start of line")
			} else if cLast == ' ' || cLast == '\t' {
				r.errC <- line.NewMsg(in.lineNum, "extra white-space at the end of the line")
			}
		}
		list = append(list, Elem{Text: strings.TrimSpace(in.line), LineNum: in.lineNum})
		el = &list[len(list)-1]
	}

	if sub != nil {
		close(sub)
	} else {
		close(r.errC)
	}
	close(ret)
}
