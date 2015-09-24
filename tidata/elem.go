package tidata

import (
	"errors"
	"fmt"
	"strings"
)

type Elem struct {
	Text     string
	Children []Elem
	LineNum  int
}

func (e *Elem) String() string {
	if e == nil {
		return "<nil>"
	}
	return e.pfxString("")
}
func (e *Elem) pfxString(pfx string) string {
	s := pfx + e.Text + "\n"
	subPfx := pfx + "\t"

	for i := range e.Children {
		s += e.Children[i].pfxString(subPfx)
	}
	return s
}

func (e Elem) Value() (val string) {
	if i := strings.IndexAny(e.Text, " \t"); i != -1 {
		val = e.Text[i+1:]
	}
	return
}

func (e Elem) Key() (key string) {
	key = e.Text
	if i := strings.IndexAny(e.Text, " \t"); i != -1 {
		key = key[:i]
	}
	return
}

// Find the first occurance of ‘key’ in the list of childs,
// on success, return the corresponding slice index
// and a pointer to the Elem. Otherwise, return nil.
//
func (el *Elem) Lookup(key string) (i int, e *Elem) {
	var c Elem

	pfxTab := key + "\t"
	pfxSpace := key + " "
	for i, c = range el.Children {
		if key == c.Text || strings.HasPrefix(c.Text, pfxTab) || strings.HasPrefix(c.Text, pfxSpace) {
			e = &c
			break
		}
	}
	return
}

func (el *Elem) Match(key string) bool {
	if strings.HasPrefix(el.Text, key+"\t") || key == el.Text {
		return true
	}
	return false
}

// Create a map from an Elem's slice of children. Each key of a
// child will be used as a key into the map, a pointer to the
// child's Elem as value.
//
func (el *Elem) MapChildren() (m map[string]*Elem, err error) {
	m = make(map[string]*Elem, len(el.Children))

	for i := range el.Children {
		c := &el.Children[i]
		key := c.Text
		if i := strings.Index(key, "\t"); i != -1 {
			key = key[:i]
		}
		if _, ok := m[key]; ok {
			err = fmt.Errorf("tidata: duplicate keys: \"%s.%s\"\n", el.Text, key)
		} else {
			m[key] = c
		}
	}
	return
}
