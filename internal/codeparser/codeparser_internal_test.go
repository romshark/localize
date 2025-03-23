package codeparser

import (
	"testing"

	"github.com/romshark/localize/internal/fmtplaceholder"
	"github.com/stretchr/testify/require"
)

func TestPlaceholders(t *testing.T) {
	t.Parallel()
	f := func(t *testing.T, expect []string, input string) {
		t.Helper()
		a := fmtplaceholder.Extract(input)
		require.Equal(t, expect, a)
	}

	f(t, nil, "")
	f(t, nil, "abcdefg")
	// Boolean
	f(t, []string{"%t"}, "foo %t bar")
	// General
	f(t, []string{"%v", "%#v", "%T", "%%"},
		"%v, %#v, %T, %%")
	// Float
	f(t, []string{
		"%f", "%9f", "%.2f", "%9.2f", "%9.f",
		"%F", "%9F", "%.2F", "%9.2F", "%9.F",
		"%g", "%9g", "%.2g", "%9.2g", "%9.g",
		"%G", "%9G", "%.2G", "%9.2G", "%9.G",
		"%x", "%9x", "%.2x", "%9.2x", "%9.x",
		"%X", "%9X", "%.2X", "%9.2X", "%9.X",
		"%e", "%E",
	},
		"%f, %9f, %.2f, %9.2f, %9.f, "+
			"%F, %9F, %.2F, %9.2F, %9.F, "+
			"%g, %9g, %.2g, %9.2g, %9.g, "+
			"%G, %9G, %.2G, %9.2G, %9.G, "+
			"%x, %9x, %.2x, %9.2x, %9.x, "+
			"%X, %9X, %.2X, %9.2X, %9.X, "+
			"%e, %E")
	// Integer
	f(t, []string{"%b", "%c", "%d", "%o", "%O", "%q", "%x", "%X", "%U"},
		"%b, %c, %d, %o, %O, %q, %x, %X, %U",
	)
	// String / Slice / Pointer
	f(t, []string{"%s", "%q", "%x", "%X", "%p"}, "%s, %q, %x, %X, %p")
}
