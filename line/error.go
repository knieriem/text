package line

import (
	"fmt"
	"sort"
)

type Error interface {
	error
	Line() int
}

type ErrorList struct {
	Filename string
	List     []error
}

func (el *ErrorList) Error() (s string) {
	if len(el.List) != 0 {
		err := el.List[0]
		if e, ok := err.(Error); ok {
			s = fmt.Sprintf("%d: %s", e.Line(), e.Error())
		} else {
			s = err.Error()
		}
	}
	return
}

func (e *ErrorList) Add(err error) {
	e.List = append(e.List, err)
}

func (e *ErrorList) AddMsg(line int, msg string) {
	e.List = append(e.List, &message{msg: msg, line: line})
}

func (e *ErrorList) AddError(line int, err error) {
	e.List = append(e.List, &lineError{error: err, line: line})
}

func (list *ErrorList) Sort() {
	sort.Sort(list)
}

type message struct {
	msg  string
	line int
}

func (m *message) Error() string {
	return m.msg
}

func (m *message) Line() int {
	return m.line
}

type lineError struct {
	error
	line int
}

func (e *lineError) Line() int {
	return e.line
}

// implementation of sort.Interface
func (e *ErrorList) Len() int {
	return len(e.List)
}

func (e *ErrorList) Less(i, j int) bool {
	line1 := line(e.List[i])
	line2 := line(e.List[j])
	return line1 < line2
}

func (e *ErrorList) Swap(i, j int) {
	e.List[i], e.List[j] = e.List[j], e.List[i]
}

func line(err error) (l int) {
	switch e := err.(type) {
	case Error:
		l = e.Line()
	default:
		l = -1
	}
	return
}
