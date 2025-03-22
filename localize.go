package localize

import (
	"errors"
	"fmt"

	"golang.org/x/text/language"
)

// Forms defines the different forms
//
// For more information, see CLDR documentation:
// https://cldr.unicode.org/index/cldr-spec/plural-rules
type Forms struct {
	// Zero defines the plural form used when the quantity is zero,
	// as required by some languages.
	Zero string

	// One defines the plural form used when the quantity is exactly one,
	// which is the singular form in most languages.
	One string

	// Two defines the dual form used in some languages.
	Two string

	// Few defines the paucal form used in some languages.
	Few string

	// Many is used for fractions in some languages
	// if they have a separate class.
	Many string

	// Other is the general plural form used for all languages and
	// must always be defined.
	Other string
}

// Reader reads localized data.
type Reader interface {
	// Locale provides the locale this reader localizes for.
	Locale() language.Tag

	// Base provides the base language this reader localizes for.
	Base() language.Base

	// Text provides static 1-to-1 translations.
	Text(text string) (localized string)

	// Block provides static 1-to-1 translations for a multi-line string block.
	// Common leading indentation is automatically removed. For example:
	//
	//   `
	//       First line.
	//        Second line.
	//
	//       Third line.
	//   `
	//
	// becomes:
	//
	//   "First line.\n Second line.\n\nThird line."
	Block(text string) (localized string)

	// Plural provides plural translations in cardinal form like:
	//
	//   template="You have %d unread emails":
	//    localized="You have 5 unread emails" (quantity=5)
	//    localized="You have 1 unread email" (quantity=1)
	//
	// For more information see unicode plural rules specification:
	// https://www.unicode.org/cldr/charts/47/supplemental/language_plural_rules.html
	Plural(template Forms, quantity int) (localized string)

	// PluralBlock behaves like Plural and formats like Block.
	PluralBlock(templates Forms, quantity int) (localized string)

	// Ordinal provides localized representation of numbers in ordinal form like:
	//
	//   "1st" (en; n=1)
	//   "2nd" (en; n=2)
	//   "23rd" (en; n=23)
	//   "104th" (en; n=104)
	//
	// For more information see unicode plural rules specification:
	// https://www.unicode.org/cldr/charts/47/supplemental/language_plural_rules.html
	Ordinal(n int) (localized string)
}

// Bundle is a group of localized readers.
type Bundle struct {
	locales          []language.Tag
	defaultLocaleStr string
	readers          []Reader
	readerByLocale   map[string]Reader
}

var (
	ErrEmptyBundle    = errors.New("empty bundle")
	ErrReaderConflict = errors.New("conflicting readers")
)

// New creates a new localization bundle.
func New(defaultLocale language.Tag, bundle ...Reader) (*Bundle, error) {
	if len(bundle) < 1 {
		return nil, ErrEmptyBundle
	}
	def := defaultLocale.String()
	readers := make([]Reader, len(bundle))
	readerByLocale := make(map[string]Reader, len(bundle))
	locales := make([]language.Tag, len(bundle))
	for i, r := range bundle {
		locale := r.Locale()
		locales[i] = locale
		localeStr := locale.String()
		if _, ok := readerByLocale[localeStr]; ok {
			return nil, fmt.Errorf("%w for %q", ErrReaderConflict, locale)
		}
		readerByLocale[localeStr] = r
		readers[i] = r
	}
	return &Bundle{
		locales:          locales,
		readers:          readers,
		defaultLocaleStr: def,
		readerByLocale:   readerByLocale,
	}, nil
}

// Match returns the best matching reader for locale.
func (l *Bundle) Match(
	locale language.Tag, tags ...language.Tag,
) (Reader, language.Confidence) {
	matcher := language.NewMatcher(l.locales)
	matchedTag, _, c := matcher.Match(tags...)
	return l.readerByLocale[matchedTag.String()], c
}

// ForBase returns either the localization for language, or the default localization
// if no localization for language is found.
func (l *Bundle) ForBase(language language.Base) Reader {
	r := l.readerByLocale[language.String()]
	if r == nil {
		r = l.readerByLocale[l.defaultLocaleStr]
	}
	return r
}

// Default returns the reader for the default locale.
func (l *Bundle) Default() Reader { return l.readerByLocale[l.defaultLocaleStr] }

// Locales returns all locales of the bundle.
func (l *Bundle) Locales() []language.Tag { return l.locales }

// Readers returns all available readers.
func (l *Bundle) Readers() []Reader { return l.readers }
