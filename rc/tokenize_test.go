package rc

import (
	"testing"
)

type testSpec struct {
	input  string
	fields []string
}

var tokenizeTests = []testSpec{
	{
		input: "jumps over",
		fields: []string{
			"jumps", "over",
		},
	},
	{
		input: "th''''e 'quick br'own 'fox'",
		fields: []string{
			"th'e", "quick brown", "fox",
		},
	},
}

func TestTokenize(t *testing.T) {
	for i, test := range tokenizeTests {
		fields := Tokenize(test.input)
		for j, f := range fields {
			if f2 := test.fields[j]; f != f2 {
				t.Errorf("[%d] mismatch: %q != %q", i, f, f2)
			}
		}
	}
}
