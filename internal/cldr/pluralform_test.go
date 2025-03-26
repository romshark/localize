package cldr_test

import (
	_ "embed"
	"testing"

	"github.com/romshark/localize/internal/cldr"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestPluralFormsTag(t *testing.T) {
	t.Parallel()

	f := func(t *testing.T, lang language.Tag, expect cldr.PluralForms) {
		t.Helper()
		forms, ok := cldr.ByTag(lang)
		require.True(t, ok)
		require.Equal(t, expect, forms)
	}

	{
		forms := cldr.PluralForms{
			Cardinal: cldr.CLDRForms{One: true, Other: true},
			CardinalForms: []cldr.CLDRPluralForm{
				cldr.CLDRPluralFormOne,
				cldr.CLDRPluralFormOther,
			},
			GettextFormula:     "n != 1",
			GettextPluralForms: "nplurals=2; plural=n != 1",
		}
		f(t, language.Afrikaans, forms)
		f(t, language.Dutch, forms)
		f(t, language.Danish, forms)
		f(t, language.English, forms)
		f(t, language.Estonian, forms)
		f(t, language.Finnish, forms)
		f(t, language.German, forms)
		f(t, language.Greek, forms)
		f(t, language.Norwegian, forms)
		f(t, language.Turkish, forms)
	}

	{
		forms := cldr.PluralForms{
			Cardinal: cldr.CLDRForms{One: true, Few: true, Other: true},
			CardinalForms: []cldr.CLDRPluralForm{
				cldr.CLDRPluralFormOne,
				cldr.CLDRPluralFormFew,
				cldr.CLDRPluralFormOther,
			},
			GettextFormula: "(n % 10 == 1 && n % 100 != 11) ? 0 : " +
				"((n % 10 >= 2 && n % 10 <= 4 && (n % 100 < 12 || n % 100 > 14)) ? 1 : 2)",
			GettextPluralForms: "nplurals=3; plural=" +
				"(n % 10 == 1 && n % 100 != 11) ? 0 : " +
				"((n % 10 >= 2 && n % 10 <= 4 && (n % 100 < 12 || n % 100 > 14)) ? 1 : 2)",
		}
		f(t, language.Ukrainian, forms)
		f(t, language.Russian, forms)
		f(t, language.Serbian, forms)
		f(t, language.Croatian, forms)
	}

	{
		forms := cldr.PluralForms{
			Cardinal: cldr.CLDRForms{One: true, Few: true, Other: true},
			CardinalForms: []cldr.CLDRPluralForm{
				cldr.CLDRPluralFormOne,
				cldr.CLDRPluralFormFew,
				cldr.CLDRPluralFormOther,
			},
			GettextFormula: "(n == 1) ? 0 : ((n % 10 >= 2 && n % 10 <= 4 && " +
				"(n % 100 < 12 || n % 100 > 14)) ? 1 : 2)",
			GettextPluralForms: "nplurals=3; plural=" +
				"(n == 1) ? 0 : ((n % 10 >= 2 && n % 10 <= 4 && " +
				"(n % 100 < 12 || n % 100 > 14)) ? 1 : 2)",
		}
		f(t, language.Polish, forms)
	}
}

func TestPluralFormsTagNotFound(t *testing.T) {
	t.Parallel()

	z, ok := cldr.ByTag(language.AmericanEnglish)
	require.False(t, ok)
	require.Zero(t, z)
}

func TestPluralFormsBase(t *testing.T) {
	t.Parallel()

	f := func(t *testing.T, expect cldr.PluralForms, locale language.Tag) {
		t.Helper()
		base, _ := locale.Base()
		forms, ok := cldr.ByBase(base)
		require.True(t, ok)
		require.Equal(t, expect, forms)
	}

	{
		forms := cldr.PluralForms{
			Cardinal: cldr.CLDRForms{One: true, Other: true},
			CardinalForms: []cldr.CLDRPluralForm{
				cldr.CLDRPluralFormOne,
				cldr.CLDRPluralFormOther,
			},
			GettextFormula:     "n != 1",
			GettextPluralForms: "nplurals=2; plural=n != 1",
		}
		f(t, forms, language.AmericanEnglish)
		f(t, forms, language.BritishEnglish)
	}
}

func TestCLDRPluralFormString(t *testing.T) {
	t.Parallel()
	require.Equal(t, "", cldr.CLDRPluralForm(0).String())
	require.Equal(t, "Zero", cldr.CLDRPluralFormZero.String())
	require.Equal(t, "One", cldr.CLDRPluralFormOne.String())
	require.Equal(t, "Two", cldr.CLDRPluralFormTwo.String())
	require.Equal(t, "Few", cldr.CLDRPluralFormFew.String())
	require.Equal(t, "Many", cldr.CLDRPluralFormMany.String())
	require.Equal(t, "Other", cldr.CLDRPluralFormOther.String())
}
