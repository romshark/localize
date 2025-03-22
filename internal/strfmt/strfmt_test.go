package strfmt_test

import (
	"testing"

	"github.com/romshark/localize/internal/strfmt"
	"github.com/stretchr/testify/require"
)

func TestDedent(t *testing.T) {
	t.Parallel()
	f := func(t *testing.T, expect, input string) {
		t.Helper()
		a := strfmt.Dedent(input)
		require.Equal(t, expect, a)
	}

	f(t, "", ``)
	f(t, "foo", `foo`)
	f(t, "foo", ` foo `)
	f(t, "foo\n\tbar", `foo
	bar`)
	f(t, "foo", `
		foo
	`)
	f(t, "foo", `

		foo

`)
	f(t, "foo\nbar", `
		foo
		bar
	`)
	f(t, "foo\nbar", `
		  foo
		  bar
	`)
	f(t, "foo\nbar", `
            foo
            bar
	`)
	f(t, "foo\n\nbar", `
		foo

		bar
	`)
	f(t, "foo\n bar\nbazz", `
		foo
		 bar
		bazz
	`)
	f(t, "foo\n bar\nbazz", `
		foo
		 bar
		bazz
	`)
	f(t, "foo\n\t\t bar\n\t\tbazz", `foo
		 bar
		bazz
`)
}
