package stringutil

import (
	"errors"
	"regexp"
)

var (
	fncallREStr = `\pL[\pL\pN]*\(`
	chainingRE  = regexp.MustCompile(`\) *\.(` + fncallREStr + `)`)
	fncallRE    = regexp.MustCompile(fncallREStr + `$`)
)

// ConvertMethodChain converts all, possibly nested,
// (x).method(y) expressions found in s into method(x, y) expressions.
func ConvertMethodChain(s, argsep string) (string, error) {
	for {
		loc := chainingRE.FindStringSubmatchIndex(s)
		if loc == nil {
			return s, nil
		}
		icb := loc[0]
		iob := FindOpeningBracket(s, '(', icb)
		if iob == -1 {
			return "", errors.New("missing opening brace")
		}
		i0 := iob
		identLoc := fncallRE.FindStringIndex(s[:iob+1])
		object := s[i0+1 : icb]
		if identLoc != nil {
			i0 = identLoc[0]
			object = s[i0 : icb+1]
		}
		iFncall := loc[2]
		s = s[:i0] + s[iFncall:loc[1]] + object + argsep + s[loc[1]:]
	}
}
