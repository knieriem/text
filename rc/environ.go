package rc

import (
	"bytes"
	"fmt"
	"io"
	"sort"
)

// An EnvMap contains environment variables.
type EnvMap map[string][]string

// Insert copies all elements from src into m.
func (m EnvMap) Insert(src EnvMap) {
	for name, val := range src {
		m[name] = val
	}
}

type EnvStack []EnvMap

// Get the value of a variable from the topmost EnvMap of s.
func (s EnvStack) Get(name string) (value []string) {
	for i := s.iLast(); i >= 0; i-- {
		v, ok := s[i][name]
		if ok {
			value = v
			break
		}
	}
	return
}

// Set the value of a variable in the topmost EnvMap of s.
func (s *EnvStack) Set(name string, value []string) {
	if i := s.iLast(); i >= 0 {
		(*s)[i][name] = value
	}
}

// Push pushes m onto the EnvStack s.
func (s *EnvStack) Push(m EnvMap) {
	if m == nil {
		m = make(EnvMap, 8)
	}
	*s = append(*s, m)
}

// Pop removes the topmost EnvMap from s.
func (s *EnvStack) Pop() {
	*s = (*s)[:s.iLast()]

}

func (s EnvStack) iLast() int {
	return len(s) - 1
}

// Insert copies all elements from m into the topmost EnvMap of s.
func (s EnvStack) Insert(m EnvMap) {
	i := s.iLast()
	dest := s[i]
	if dest == nil {
		s[i] = m
	} else {
		dest.Insert(m)
	}
}

// String returns the EnvMap formatted as a string of
// assignments in alphabetically sorted order.
func (m EnvMap) String() string {
	if len(m) == 0 {
		return ""
	}
	b := new(bytes.Buffer)
	m.WriteTo(b)
	return b.String()
}

// WriteTo writes the EnvMap formatted as a string of
// assignments in alphabetically sorted order.
func (m EnvMap) WriteTo(w io.Writer) (n int64, err error) {
	if len(m) == 0 {
		return 0, nil
	}
	varNames := make([]string, 0, len(m))
	for k := range m {
		varNames = append(varNames, k)
	}
	sort.Strings(varNames)

	nw := int64(0)
	sep := ""
	for _, name := range varNames {
		values := m[name]
		val := ""
		if len(values) != 0 {
			val = values[0]
		}
		n, err := fmt.Fprintf(w, "%s%s=%s", sep, Quote(name), Quote(val))
		if err != nil {
			return nw, err
		}
		nw += int64(n)
		sep = " "
	}
	return nw, nil
}
