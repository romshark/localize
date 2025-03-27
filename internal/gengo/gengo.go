// Package gengo provides the Go code generator.
package gengo

import (
	_ "embed"
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/romshark/localize/internal/codeparser"
	"golang.org/x/text/language"
)

//go:embed template.gotmpl
var templateGotmpl string

func Write(
	w io.Writer, sourceLocale language.Tag, headComment []string,
	packageName string, catalogs ...*codeparser.Collection,
) error {
	tmpl, err := template.New("gen").Parse(templateGotmpl)
	if err != nil {
		return fmt.Errorf("rendering template: %w", err)
	}
	type catalogInfo struct {
		TypeName           string
		TypeNameUnexported string
		LocaleTag          language.Tag
		// LocalesSubpackage is the subpackage name of the repository
		// "github.com/go-playground/locales
		LocalesSubpackage string
		// Locale is necessary because regular BCP 47 notation can't
		// be used in Go import aliases and type names.
		Locale         string
		MessagesStatic []string
		MessagesPlural []codeparser.Msg
		// Native defines whether this catalog needs no translation and can return
		// all input text straight back to the caller as localized translation.
		Native bool
	}

	catInfo := make([]catalogInfo, len(catalogs))
	for i, c := range catalogs {
		tpName := localizationTypeName(c.Locale)
		catInfo[i] = catalogInfo{
			TypeName:           tpName,
			LocaleTag:          c.Locale,
			LocalesSubpackage:  localesSubpackage(c.Locale),
			Locale:             safeLocaleStr(c.Locale),
			TypeNameUnexported: strings.ToLower(tpName[:1]) + tpName[1:],
			Native:             sourceLocale == c.Locale,
		}

		for m := range c.Ordered() {
			switch m.FuncType {
			case codeparser.FuncTypeText:
			case codeparser.FuncTypeBlock:
				catInfo[i].MessagesStatic = append(catInfo[i].MessagesStatic, m.Other)
			case codeparser.FuncTypePlural, codeparser.FuncTypePluralBlock:
				catInfo[i].MessagesPlural = append(catInfo[i].MessagesPlural, m)
			default:
				panic("normally unreachable")
			}
		}
	}
	return tmpl.Execute(w, struct {
		HeadComment      []string
		GeneratorVersion string
		BundleVersion    string
		Package          string
		Catalogs         []catalogInfo
	}{
		HeadComment:      headComment,
		GeneratorVersion: "1",
		BundleVersion:    "1",
		Package:          packageName,
		Catalogs:         catInfo,
	})
}

func localizationTypeName(locale language.Tag) string {
	s := locale.String() // Like "en-US", "de-CH"
	s = strings.ReplaceAll(s, "-", "_")

	// Capitalize each segment to form CamelCase.
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}

	return "Catalog" + strings.Join(parts, "")
}

func safeLocaleStr(t language.Tag) string {
	s := strings.ReplaceAll(t.String(), "-", "_")
	return strings.ToUpper(s[:1]) + s[1:]
}

func localesSubpackage(t language.Tag) string {
	return strings.ReplaceAll(t.String(), "-", "_")
}
