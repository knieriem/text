package rc

import (
	"testing"
)

type quoteTestSpec struct {
	src       string
	quoted    string
	quotedCmd string
}

var quoteTests = []quoteTestSpec{
	{
		src:       "foo",
		quoted:    "foo",
		quotedCmd: "foo",
	},
	{
		src:       "$foo",
		quoted:    `'$foo'`,
		quotedCmd: `'$foo'`,
	},
	{
		src:       "a=b",
		quoted:    `'a=b'`,
		quotedCmd: `a=b`,
	},
	{
		src:       "$foo=$bar",
		quoted:    `'$foo=$bar'`,
		quotedCmd: `'$foo'='$bar'`,
	},
	{
		src:       "===",
		quoted:    `'==='`,
		quotedCmd: "===",
	},
	{
		src:       "a===b",
		quoted:    `'a===b'`,
		quotedCmd: "a===b",
	},
	{
		src:       "a===$b",
		quoted:    `'a===$b'`,
		quotedCmd: `a==='$b'`,
	},
	{
		src:       "a=b'c=d",
		quoted:    `'a=b''c=d'`,
		quotedCmd: `a='b''c'=d`,
	},
	{
		src:       "'='",
		quoted:    `'''='''`,
		quotedCmd: `''''=''''`,
	},
}

func TestQuote(t *testing.T) {
	for i := range quoteTests {
		test := &quoteTests[i]
		q := Quote(test.src)
		if test.quoted != q {
			t.Errorf("[%d] mismatch: %q != %q", i, q, test.quoted)
			return
		}
	}
}

func TestQuoteCmd(t *testing.T) {
	for i := range quoteTests {
		test := &quoteTests[i]
		q := QuoteCmd(test.src)
		if test.quotedCmd != q {
			t.Errorf("[%d] mismatch: %q != %q", i, q, test.quotedCmd)
			return
		}
	}
}
