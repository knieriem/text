package rc

import (
	"testing"
)

type testSpec struct {
	input       string
	fields      []string
	assignments EnvMap
	redir       Redirection
	mustFail    bool
}

var testEnvMap = EnvMap{
	"mammal": {"squirrel"},
	"nut":    {"hazelnut"},
	"foo":    {"bar"},
	"ar":     {"az"},
	"ba":     {"fo"},
}

var commonTests = []testSpec{
	{
		input: "jumps over",
		fields: []string{
			"jumps", "over",
		},
	}, {
		input: "th''''e 'quick br'own 'fox'",
		fields: []string{
			"th'e", "quick brown", "fox",
		},
	},
}

var tokenizeTests = []testSpec{
	{
		input: "a $m'a'm$mal ea^t's 'a $nut",
		fields: []string{
			"a", "$mam$mal", "ea^ts a", "$nut",
		},
	},
}

var tokenizeCmdTests = []testSpec{
	{
		input: "$foo=ba'z' b$ar=$ba^o $bar",
		fields: []string{
			"",
		},
		assignments: EnvMap{
			"bar": {"baz"},
			"baz": {"foo"},
		},
	}, {
		input: "$foo=ba'z' b#foo",
		fields: []string{
			"b",
		},
		assignments: EnvMap{
			"bar": {"baz"},
		},
	}, {
		input: "$foo=$ba:o $ba/o fo/o",
		fields: []string{
			"fo/o", "fo/o",
		},
		assignments: EnvMap{
			"bar": {"fo:o"},
		},
	}, {
		input: "#foo",
	}, {
		input: "a #foo",
		fields: []string{
			"a",
		},
	}, {
		input: "'$a' $mammal eats a $nut",
		fields: []string{
			"$a", "squirrel", "eats", "a", "hazelnut",
		},
	}, {
		input: "a=b=c foo",
		fields: []string{
			"foo",
		},
		assignments: EnvMap{
			"a": {"b=c"},
		},
	}, {
		input:    "^a",
		mustFail: true,
	}, {
		input: "a b > c",
		fields: []string{
			"a", "b",
		},
		redir: Redirection{Type: ">", Filename: "c"},
	}, {
		input: "a b< c",
		fields: []string{
			"a", "b",
		},
		redir: Redirection{Type: "<", Filename: "c"},
	},
}

func TestTokenize(t *testing.T) {
	for i, test := range append(commonTests, tokenizeTests...) {
		compareStringSlices(t, test.fields, Tokenize(test.input), "field", i)
	}
}

func TestTokenizeCmd(t *testing.T) {
	tok := new(Tokenizer)
	tok.Getenv = func(name string) []string {
		return testEnvMap[name]
	}
	for i, test := range append(commonTests, tokenizeCmdTests...) {
		cmd, err := tok.ParseCmdLine(test.input)
		if err != nil {
			if !test.mustFail {
				t.Error(err)
			}
			continue
		} else if test.mustFail {
			t.Errorf("[%d] should have failed", i)
			continue
		}
		compareStringSlices(t, test.fields, cmd.Fields, "field", i)
		if n1, n2 := len(test.assignments), len(cmd.Assignments); n1 != n2 {
			t.Errorf("[%d] number of assignments don't match: %d != %d", i, n1, n2)
			continue
		}
		if r1, r2 := test.redir, cmd.Redir; r1.Type != r2.Type || r1.Filename != r2.Filename {
			t.Errorf("[%d] redirection doesn't match: %v != %v", i, r1, r2)
			continue
		}
		for name, val1 := range test.assignments {
			val2, ok := cmd.Assignments[name]
			if !ok {
				t.Errorf("[%d] assignment not present: %s", i, name)
				continue
			}
			compareStringSlices(t, val1, val2, "assignment value", i)
		}
	}
}

func compareStringSlices(t *testing.T, want, have []string, context string, iTest int) {
	if len(want) != len(have) {
		t.Errorf("[%d] %s count: %d != %d", iTest, context, len(want), len(have))
		return
	}
	for i, v := range want {
		if v != have[i] {
			t.Errorf("[%d] %s mismatch: %q != %q", iTest, context, v, have[i])
			return
		}
	}
}
