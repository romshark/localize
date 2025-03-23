package fmtplaceholder_test

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

	// None
	f(t, nil, "")
	f(t, nil, "abc de fg")
	// Boolean
	f(t, []string{"%t"}, "foo жщя %t bar")
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

func TestNumeric(t *testing.T) {
	t.Parallel()
	f := func(t *testing.T, expect bool, input string) {
		t.Helper()
		a := fmtplaceholder.Numeric(input)
		require.Equal(t, expect, a)
	}

	// Non-numeric
	f(t, false, "%t")
	f(t, false, "%#v")
	f(t, false, "%T")
	f(t, false, "%%")
	f(t, false, "%s")
	f(t, false, "%p")

	// Generic v
	f(t, true, "%v")

	// Integer
	f(t, true, "%b")
	f(t, true, "%c")
	f(t, true, "%d")
	f(t, true, "%o")
	f(t, true, "%O")
	f(t, true, "%q")
	f(t, true, "%x")
	f(t, true, "%X")
	f(t, true, "%U")
	// Float
	f(t, true, "%f")
	f(t, true, "%9f")
	f(t, true, "%.2f")
	f(t, true, "%9.2f")
	f(t, true, "%9.f")
	f(t, true, "%F")
	f(t, true, "%9F")
	f(t, true, "%.2F")
	f(t, true, "%9.2F")
	f(t, true, "%9.F")
	f(t, true, "%g")
	f(t, true, "%9g")
	f(t, true, "%.2g")
	f(t, true, "%9.2g")
	f(t, true, "%9.g")
	f(t, true, "%G")
	f(t, true, "%9G")
	f(t, true, "%.2G")
	f(t, true, "%9.2G")
	f(t, true, "%9.G")
	f(t, true, "%x")
	f(t, true, "%9x")
	f(t, true, "%.2x")
	f(t, true, "%9.2x")
	f(t, true, "%9.x")
	f(t, true, "%X")
	f(t, true, "%9X")
	f(t, true, "%.2X")
	f(t, true, "%9.2X")
	f(t, true, "%9.X")
	f(t, true, "%e")
	f(t, true, "%E")
}
