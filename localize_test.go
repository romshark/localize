package localize_test

import (
	"testing"

	"github.com/romshark/localize"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

type MockReaderPlural struct{ Zero, One, Two, Few, Many, Other string }

type MockReader struct {
	tag      language.Tag
	static   map[string]string
	cardinal map[string]MockReaderPlural
	ordinal  func(n int) string
}

var _ localize.Reader = MockReader{}

func (r MockReader) Locale() language.Tag { return r.tag }

func (r MockReader) Base() language.Base {
	b, _ := r.tag.Base()
	return b
}

const (
	MockZero = 0
	MockOne  = 1
	MockTwo  = 2
	MockFew  = 3
	MockMany = 6
)

func (r MockReader) Text(text string) string  { return r.static[text] }
func (r MockReader) Block(text string) string { return r.static[text] }

func (r MockReader) Plural(templates localize.Forms, quantity int) string {
	// TODO
	_ = r.tag
	_ = r.static
	_ = r.cardinal
	_ = r.ordinal
	return ""
	// p := r.cardinal[templates]
	// switch quantity {
	// case MockZero:
	// 	if p.Zero != "" {
	// 		return p.Zero
	// 	}
	// case MockOne:
	// 	if p.One != "" {
	// 		return p.One
	// 	}
	// case MockTwo:
	// 	if p.Two != "" {
	// 		return p.Two
	// 	}
	// case MockFew:
	// 	if p.Few != "" {
	// 		return p.Few
	// 	}
	// case MockMany:
	// 	if p.Many != "" {
	// 		return p.Many
	// 	}
	// }
	// return fmt.Sprintf(p.Other, quantity)
}

func (r MockReader) PluralBlock(templates localize.Forms, quantity int) string {
	// TODO
	return ""
	// p := r.cardinal[template]
	// switch quantity {
	// case MockZero:
	// 	if p.Zero != "" {
	// 		return p.Zero
	// 	}
	// case MockOne:
	// 	if p.One != "" {
	// 		return p.One
	// 	}
	// case MockTwo:
	// 	if p.Two != "" {
	// 		return p.Two
	// 	}
	// case MockFew:
	// 	if p.Few != "" {
	// 		return p.Few
	// 	}
	// case MockMany:
	// 	if p.Many != "" {
	// 		return p.Many
	// 	}
	// }
	// return fmt.Sprintf(p.Other, quantity)
}

func (r MockReader) Ordinal(n int) string { return r.ordinal(n) }

var _ localize.Reader = new(MockReader)

func TestErrEmptyBundle(t *testing.T) {
	l, err := localize.New(language.English /* no bundles */)
	require.ErrorIs(t, err, localize.ErrEmptyBundle)
	require.Nil(t, l)
}

func TestErrReaderConflict(t *testing.T) {
	german1 := &MockReader{tag: language.German}
	german2 := &MockReader{tag: language.German}
	l, err := localize.New(language.English, german1, german2)
	require.ErrorIs(t, err, localize.ErrReaderConflict)
	require.Nil(t, l)
}

// func Test(t *testing.T) {
// 	baseEnglish, _ := language.English.Base()
// 	baseGerman, _ := language.German.Base()
// 	readerEnglish := MockReader{
// 		BaseLang: baseEnglish,
// 		TextByKey: map[string]string{
// 			"key.tree":  "tree",
// 			"key.plant": "plant",
// 		},
// 		PluralByKey: map[string]MockReaderPlural{
// 			"key.trees": {
// 				Zero:  "no trees",
// 				One:   "one tree",
// 				Other: "%d trees",
// 			},
// 		},
// 	}
// 	readerGerman := MockReader{
// 		BaseLang: baseGerman,
// 		TextByKey: map[string]string{
// 			"key.tree":  "Baum",
// 			"key.plant": "Pflanze",
// 		},
// 		PluralByKey: map[string]MockReaderPlural{
// 			"key.trees": {
// 				Zero:  "kein Baum",
// 				One:   "ein Baum",
// 				Other: "%d Bäume",
// 			},
// 		},
// 	}
// 	l := localize.New(readerEnglish, readerGerman)
// }
