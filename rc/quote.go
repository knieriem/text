package rc

import (
	"strings"
	"unicode/utf8"
)

func NeedsQuote(r rune) bool {
	if r <= ' ' {
		return true
	}
	if r >= utf8.RuneSelf {
		return false
	}
	if strings.IndexByte("`^#*[]=|\\?${}()'<>&;", byte(r)) != -1 {
		return true
	}
	return false
}

func Quote(s string) string {
	for _, r := range s {
		if NeedsQuote(r) {
			return "'" + strings.Replace(s, "'", "''", -1) + "'"
		}
	}
	return s
}

func Join(list []string) (js string) {
	if len(list) == 0 {
		return
	}
	js = list[0]
	for _, s := range list[1:] {
		js += " " + Quote(s)
	}
	return
}
