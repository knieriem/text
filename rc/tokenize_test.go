package rc

import (
	"strings"
	"testing"
)

type testSpec struct {
	input       string
	fields      []string
	assignments EnvMap
	env         EnvMap
	redir       Redirection
	mustFail    bool
}

var testEnvMap = EnvMap{
	"mammal": {"squirrel"},
	"nut":    {"hazelnut"},
	"foo":    {"bar"},
	"bar":    {""},
	"ar":     {"az"},
	"ba":     {"fo"},
	"*":      {"x", "y", "z"},
	"args":   {"x", "y"},
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
	}, {
		input: "unclosed 'quote",
		fields: []string{
			"unclosed", "quote",
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
		// this failure should be aligned better with Plan 9 rc's behaviour
		input:    "$foo=ba'z' b$ar=$ba^o $bar",
		env:      EnvMap{},
		mustFail: true,
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
		input:    "unclosed 'quote",
		mustFail: true,
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
		input: "args contains $#args elements",
		fields: []string{
			"args", "contains", "2", "elements",
		},
	}, {
		input: `$"args`,
		fields: []string{
			"x y",
		},
	}, {
		input: `a "quoted" string`,
		fields: []string{
			"a", `"quoted"`, "string",
		},
	}, {
		input: "'$args': $args",
		fields: []string{
			"$args:", "x", "y",
		},
	}, {
		input: "'$*': $*",
		fields: []string{
			"$*:", "x", "y", "z",
		},
	}, {
		input: "'empty args:' $* $notexist end",
		env: EnvMap{
			"*": nil,
		},
		fields: []string{
			"empty args:", "end",
		},
	}, {
		input: "$#none $#*",
		fields: []string{
			"0", "3",
		},
	}, {
		input:    "foo $## bar",
		mustFail: true,
	}, {
		input: "=a b",
		fields: []string{
			"=a",
			"b",
		},
	}, {
		input: "a=b=c foo d=e =f",
		fields: []string{
			"foo",
			"d=e",
			"=f",
		},
		assignments: EnvMap{
			"a": {"b=c"},
		},
	}, {
		input: "a=b$foo=c foo d=e",
		fields: []string{
			"foo",
			"d=e",
		},
		assignments: EnvMap{
			"a": {"bbar=c"},
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
	}, {
		input: "a=`{echo foo}",
		assignments: EnvMap{
			"a": {"foo"},
		},
	}, {
		input:    "a=`{echo foo",
		mustFail: true,
	}, {
		input: "a=`{echo foo}`{echo bar} b=`{echo bar baz} `{echo foo}",
		fields: []string{
			"foo",
		},
		assignments: EnvMap{
			"a": {"foobar"},
			"b": {"bar", "baz"},
		},
	}, {
		input: "echo a=`{echo foo} `{echo bar}",
		fields: []string{
			"echo",
			"a=foo",
			"bar",
		},
	}, {
		input: "a=`{echo foo `{echo ba}r} baz",
		assignments: EnvMap{
			"a": {"foo", "bar"},
		},
		fields: []string{
			"baz",
		},
	}, {
		input: "a=`{echo foo `{echo bar}} baz",
		assignments: EnvMap{
			"a": {"foo", "bar"},
		},
		fields: []string{
			"baz",
		},
	}, {
		input:    "a=`{echo foo `{echo bar} baz",
		mustFail: true,
	}, {
		input:    "a=`echo foo `{echo bar} baz}",
		mustFail: true,
	}, {
		input: "$undef$foo",
		fields: []string{
			"bar",
		},
	},
}

var bugsFoundByFuzz = []testSpec{
	{
		input:    "0= ^=",
		mustFail: true,
	},
}

func TestTokenize(t *testing.T) {
	for i, test := range append(commonTests, tokenizeTests...) {
		compareStringSlices(t, test.fields, Tokenize(test.input), "field", i)
	}
}

func TestTokenizeCmd(t *testing.T) {
	tests := append(commonTests, tokenizeCmdTests...)
	tests = append(tests, bugsFoundByFuzz...)
	for i, test := range tests {
		getenv := func(name string) []string {
			if test.env != nil {
				return test.env[name]
			}
			return testEnvMap[name]
		}
		var cmd *CmdLine
		var err error
		for st, err1 := range EvalSteps(test.input, getenv) {
			r := st.Result
			if r == nil {
				cmd = &st.Cmd
				err = err1
				break
			}
			arg := st.Cmd.Fields
			if len(arg) == 0 {
				continue
			}
			switch arg[0] {
			case "echo":
				r.Output = strings.Join(arg[1:], " ")
				continue
			}
		}
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

func FuzzEvalSteps(f *testing.F) {
	for _, test := range append(commonTests, tokenizeCmdTests...) {
		f.Add(test.input)
	}
	f.Fuzz(func(t *testing.T, input string) {
		defer func() {
			if r := recover(); r != nil {
				f.Failed()
				t.Errorf("fuzzing %q: %v", input, r)
				panic(r)
			}
		}()
		for st, _ := range EvalSteps(input, nil) {
			r := st.Result
			if r == nil {
				break
			}
			arg := st.Cmd.Fields
			if len(arg) == 0 {
				continue
			}
			switch arg[0] {
			case "echo":
				r.Output = strings.Join(arg[1:], " ")
				continue
			}
		}
	})
}
