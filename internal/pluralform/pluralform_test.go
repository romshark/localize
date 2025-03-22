package pluralform_test

import (
	_ "embed"
	"testing"

	"github.com/romshark/localize/internal/pluralform"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestPluralForms(t *testing.T) {
	const expect = `nplurals=2; plural=n != 1`
	require.Equal(t, expect, pluralform.PluralForms(language.German))
}
