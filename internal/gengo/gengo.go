// Package gengo provides the Go code generator.
package gengo

import (
	_ "embed"
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/romshark/localize"
	"github.com/romshark/localize/gettext"
	"github.com/romshark/localize/internal/cldr"
	"github.com/romshark/localize/internal/codeparser"
	"golang.org/x/text/language"
)

//go:embed template.gotmpl
var templateGotmpl string

func Write(
	w io.Writer, sourceLocale language.Tag, headComment []string,
	packageName string, collection *codeparser.Collection, bundle *codeparser.Bundle,
) error {
	tmpl, err := template.New("gen").Parse(templateGotmpl)
	if err != nil {
		return fmt.Errorf("rendering template: %w", err)
	}
	type localeInfo struct {
		Tag language.Tag
		// GoPlaygroundPkg is the subpackage name of the repository
		// "github.com/go-playground/locales
		GoPlaygroundPkg string
		// Str is necessary because regular BCP 47 notation can't
		// be used in Go import aliases and type names.
		Str string
	}
	type typeName struct {
		Exported   string
		Unexported string
	}
	type catalogInfo struct {
		TypeName       typeName
		Locale         localeInfo
		POFile         gettext.FilePO
		PluralMessages []localize.Forms
	}
	type tmplInfo struct {
		Package              string
		BundleVersion        string
		HeadComment          []string
		GeneratorVersion     string
		SourceTypeName       typeName
		SourceLocale         localeInfo
		SourceMessagesStatic []string
		SourceMessagesPlural []codeparser.Msg
		Catalogs             []catalogInfo
	}

	tpNameSource := localizationTypeName(collection.Locale)
	tpNameSourceUnexp := strings.ToLower(tpNameSource[:1]) + tpNameSource[1:]
	info := tmplInfo{
		HeadComment:      headComment,
		GeneratorVersion: "1",
		BundleVersion:    "1",
		Package:          packageName,
		SourceTypeName: typeName{
			Exported:   tpNameSource,
			Unexported: tpNameSourceUnexp,
		},
		SourceLocale: localeInfo{
			Tag:             collection.Locale,
			GoPlaygroundPkg: goPlaygroundLocalesPkg(collection.Locale),
			Str:             safeLocaleStr(collection.Locale),
		},
		Catalogs: make([]catalogInfo, 0, len(bundle.Catalogs)),
	}
	{
		for loc, bundle := range bundle.Catalogs {
			cldrData, ok := cldr.ByTagOrBase(loc)
			if !ok {
				return fmt.Errorf("resolving plural forms by locale: %s", loc.String())
			}
			tpName := localizationTypeName(loc)
			tpNameUnexp := strings.ToLower(tpName[:1]) + tpName[1:]

			pluralMessages := []localize.Forms{}
			for _, msg := range bundle.FilePO.Messages.List {
				if msg.Obsolete || len(msg.MsgidPlural.Text.Lines) == 0 {
					continue
				}
				f := pluralFromGettextMsg(cldrData.CardinalForms, &msg)
				pluralMessages = append(pluralMessages, f)
			}

			info.Catalogs = append(info.Catalogs, catalogInfo{
				TypeName: typeName{
					Exported:   tpName,
					Unexported: tpNameUnexp,
				},
				Locale: localeInfo{
					Tag:             loc,
					Str:             safeLocaleStr(loc),
					GoPlaygroundPkg: goPlaygroundLocalesPkg(loc),
				},
				POFile:         bundle.FilePO,
				PluralMessages: pluralMessages,
			})
		}
	}

	for m := range collection.Ordered() {
		switch m.FuncType {
		case codeparser.FuncTypeText, codeparser.FuncTypeBlock:
			info.SourceMessagesStatic = append(info.SourceMessagesStatic, m.Other)
		case codeparser.FuncTypePlural, codeparser.FuncTypePluralBlock:
			info.SourceMessagesPlural = append(info.SourceMessagesPlural, m)
		default:
			panic("normally unreachable")
		}
	}
	return tmpl.Execute(w, info)
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

func goPlaygroundLocalesPkg(t language.Tag) string {
	tag := strings.ReplaceAll(t.String(), "-", "_")
	return "github.com/go-playground/locales/" + tag
}

// pluralFromGettextMsg translates GNU gettext indexed messages to CLDR forms.
func pluralFromGettextMsg(
	formsCLDR []cldr.CLDRPluralForm,
	m *gettext.Message,
) (f localize.Forms) {
	putInto := func(cf cldr.CLDRPluralForm, val string) {
		switch cf {
		case cldr.CLDRPluralFormZero:
			f.Zero = val
		case cldr.CLDRPluralFormOne:
			f.One = val
		case cldr.CLDRPluralFormTwo:
			f.Two = val
		case cldr.CLDRPluralFormFew:
			f.Few = val
		case cldr.CLDRPluralFormMany:
			f.Many = val
		case cldr.CLDRPluralFormOther:
			f.Other = val
		default:
			panic(fmt.Errorf("unexpected plural form: %d", cf))
		}
	}
	for index, cf := range formsCLDR {
		switch index {
		case 0:
			putInto(cf, m.Msgstr0.Text.String())
		case 1:
			putInto(cf, m.Msgstr1.Text.String())
		case 2:
			putInto(cf, m.Msgstr2.Text.String())
		case 3:
			putInto(cf, m.Msgstr3.Text.String())
		case 4:
			putInto(cf, m.Msgstr4.Text.String())
		case 5:
			putInto(cf, m.Msgstr5.Text.String())
		default:
			panic(fmt.Errorf("unexpected index: %d", index))
		}
	}
	return f
}
