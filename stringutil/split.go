// Package stringutil contains utility functions working on string arguments.
package stringutil

import (
	"strings"
)

// DelimitedBlockAttr specifies the delimiters and other attributes
// of delimited blocks.
type DelimitedBlockAttr struct {
	Begin byte // the opening delimiter
	End   byte // the closing delimiter

	// Escape, if non-zero, defines a character that
	// might be used to escape a delimiter.
	Escape byte

	// Opaque should be set to true if the contents of a
	// block shouldn't be examined for further occurences
	// of delimited blocks.
	Opaque bool
}

// RootLevelSplit slices s into substrings separated by sep on the topmost level
// of a hierarchy of delimited blocks; it returns a slice of the substrings between
// those separators. If sep is empty, or if s does not contain sep, Split returns s
// as the only element of a slice.
func RootLevelSplit(s, sep string, blockAttrs []*DelimitedBlockAttr) []string {
	var stk []*DelimitedBlockAttr
	var cur *DelimitedBlockAttr
	iStk := -1
	iCont := 0
	var list []string

	if blockAttrs == nil {
		blockAttrs = DefaultBlockAttrs
	}

	i0 := 0
	for i := range s {
		b := s[i]
		if i < iCont {
			continue
		}
		if cur != nil {
			if b == cur.End {
				stk = stk[:iStk]
				iStk--
				if iStk >= 0 {
					cur = stk[iStk]
				} else {
					cur = nil
				}
				continue
			} else if cur.Escape != 0 && b == cur.Escape {
				iCont = i + 2
				continue
			}
			if cur.Opaque {
				continue
			}
		}
		if iStk == -1 {
			if strings.HasPrefix(s[i:], sep) {
				list = append(list, s[i0:i])
				i0 = i + len(sep)
				iCont = i0
				continue
			}
		}
		for _, attr := range blockAttrs {
			if b == attr.Begin {
				stk = append(stk, attr)
				cur = attr
				iStk++
			}
		}
	}
	return append(list, s[i0:])
}

// DefaultBlockAttrs defines a list of block delimiters and attributes,
// that are used in case the blockAttrs argument to RootLevelSplit is nil.
var DefaultBlockAttrs = []*DelimitedBlockAttr{
	{Begin: '(', End: ')'},
	{Begin: '[', End: ']'},
	{Begin: '{', End: '}'},
	{Begin: '`', End: '`'},
	{Begin: '"', End: '"', Escape: '\\', Opaque: true},
}
