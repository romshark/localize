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
	w io.Writer, copyrightNotice string,
	packageName string, catalogs ...*codeparser.Catalog,
) error {
	tmpl, err := template.New("gen").Parse(templateGotmpl)
	if err != nil {
		return fmt.Errorf("rendering template: %w", err)
	}
	type catalogInfo struct {
		TypeName           string
		TypeNameUnexported string
		Locale             language.Tag
	}
	catInfo := make([]catalogInfo, len(catalogs))
	for i, c := range catalogs {
		tpName := localizationTypeName(c.Locale)
		catInfo[i] = catalogInfo{
			TypeName:           tpName,
			Locale:             c.Locale,
			TypeNameUnexported: strings.ToLower(tpName[:1]) + tpName[1:],
		}
	}
	return tmpl.Execute(w, struct {
		CopyrightNotice  string
		GeneratorVersion string
		Package          string
		Catalogs         []catalogInfo
	}{
		CopyrightNotice:  copyrightNotice,
		GeneratorVersion: "1",
		Package:          packageName,
		Catalogs:         catInfo,
	})

	// fmt.Fprint(w, "// Code generated by github.com/romshark/localize/cmd/localize."+
	// 	" DO NOT EDIT.\n\n")

	// fmt.Fprintf(w,
	// 	"// Package %s provides generated localization readers for:\n", pkgName)
	// for _, c := range catalogs {
	// 	fmt.Fprintf(w, "// - %s\n", c.Locale)
	// }
	// fmt.Fprintf(w, "package %s\n\n", pkgName)

	// fmt.Fprintln(w, `import "golang.org/x/text/language"`)
	// fmt.Fprintln(w, "")

	// for _, c := range catalogs {
	// 	typeName := localizationTypeName(c.Locale)
	// 	fmt.Fprintf(w, "// %s is a localized reader implementation for %s\n",
	// 		typeName, c.Locale.String())
	// 	fmt.Fprintf(w, "type %s struct {\n", typeName)
	// 	fmt.Fprint(w, "}\n")

	// 	// Method: Locale
	// 	fmt.Fprintf(w, "func (r %s) Locale() language.Tag { return %q }\n",
	// 		typeName, c.Locale.String())

	// 	// // Locale provides the locale this reader localizes for.
	// 	// Locale() language.Tag

	// 	// // Base provides the base language this reader localizes for.
	// 	// Base() language.Base

	// 	// // Text provides static 1-to-1 translations.
	// 	// Text(text string) string

	// 	// // Block provides static 1-to-1 translations for a multi-line string block.
	// 	// // Common leading indentation is automatically removed. For example:
	// 	// //
	// 	// //   `
	// 	// //       First line.
	// 	// //        Second line.
	// 	// //
	// 	// //       Third line.
	// 	// //   `
	// 	// //
	// 	// // becomes:
	// 	// //
	// 	// //   "First line.\n Second line.\n\nThird line."
	// 	// Block(text string) string

	// 	// // Plural provides plural translations in cardinal form like:
	// 	// //
	// 	// //   template="You have %d unread emails":
	// 	// //    localized="You have 5 unread emails" (quantity=5)
	// 	// //    localized="You have 1 unread email" (quantity=1)
	// 	// //
	// 	// // For more information see unicode plural rules specification:
	// 	// // https://www.unicode.org/cldr/charts/47/supplemental/language_plural_rules.html
	// 	// Plural(template Forms, quantity int) (localized string)

	// 	// // PluralBlock behaves like Plural and formats like Block.
	// 	// PluralBlock(templates Forms, quantity int) (localized string)

	// 	// // Ordinal provides localized representation of numbers in ordinal form like:
	// 	// //
	// 	// //   "1st" (en; n=1)
	// 	// //   "2nd" (en; n=2)
	// 	// //   "23rd" (en; n=23)
	// 	// //   "104th" (en; n=104)
	// 	// //
	// 	// // For more information see unicode plural rules specification:
	// 	// // https://www.unicode.org/cldr/charts/47/supplemental/language_plural_rules.html
	// 	// Ordinal(n int) (localized string)
	// }

	// return nil
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
