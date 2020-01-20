package stringutil

import (
	"strings"
	"testing"
)

type splitTest struct {
	src      string
	sep      string
	expected []string
}

var splitTests = []*splitTest{
	{
		src:      `foo, bar, baz`,
		sep:      ",",
		expected: []string{"foo", "bar", "baz"},
	}, {
		src:      `foo, (bar, {baz}), a, b`,
		sep:      ",",
		expected: []string{"foo", "(bar, {baz})", "a", "b"},
	}, {
		src:      `a, "(", {b, "}"}, c`,
		sep:      ",",
		expected: []string{"a", `"("`, `{b, "}"}`, "c"},
	}, {
		src:      `a, "(", {b, "}", c`,
		sep:      ",",
		expected: []string{"a", `"("`, `{b, "}", c`},
	}, {
		src:      `a, b ", c\", d", e`,
		sep:      ",",
		expected: []string{`a`, `b ", c\", d"`, `e`},
	}, {
		src:      `a; c(d;e); f`,
		sep:      `;`,
		expected: []string{`a`, `c(d;e)`, `f`},
	},
}

func TestScopedSplit(t *testing.T) {
	for _, test := range splitTests {
		f := RootLevelSplit(test.src, test.sep, nil)
		if len(f) != len(test.expected) {
			t.Fatalf("length mismatch: expected: %v, got: %v", len(test.expected), len(f))
		}
		for i, s := range f {
			s = strings.TrimSpace(s)
			if s != test.expected[i] {
				t.Fatalf("result substring mismatch: expected: %q, got: %q", test.expected[i], s)
			}
		}
	}
}
