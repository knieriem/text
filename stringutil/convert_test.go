package stringutil

import (
	"testing"
)

type methodChainConvTest struct {
	src           string
	expected      string
	expectFailure bool
}

var methodChainConvTests = []*methodChainConvTest{
	{
		src:      `1*foo().bar(2, 3)/baz`,
		expected: `1*bar(foo(), 2, 3)/baz`,
	}, {
		src:      `1*foo(sin(x).round(0.1)).bar(2, 3)`,
		expected: `1*bar(foo(round(sin(x), 0.1)), 2, 3)`,
	}, {
		src:      `1+(2+3.3).round(0.5).clip(1, 4)`,
		expected: `1+clip(round(2+3.3, 0.5), 1, 4)`,
	}, {
		src:           `sin(x)+1+2+3.3).round(0.5)`,
		expectFailure: true,
	},
}

func TestConvertMethodChain(t *testing.T) {
	for _, test := range methodChainConvTests {
		converted, err := ConvertMethodChain(test.src, ", ")
		if err != nil {
			if !test.expectFailure {
				t.Fatalf("test failed, expected success")
			}
			continue
		}
		if test.expectFailure {
			t.Fatalf("test succeeded, expected failure")
		}
		if converted != test.expected {
			t.Fatalf("mismatch: expected: %v, got: %v", test.expected, converted)
		}
	}
}
