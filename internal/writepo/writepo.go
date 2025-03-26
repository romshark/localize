// Package writepo provides functions to marshal catalogs into
// gettext compatible .po files.
package writepo

import (
	"fmt"
	"io"
	"iter"
	"strings"
	"time"

	"github.com/romshark/localize/internal/cldr"
	"github.com/romshark/localize/internal/codeparser"
	"golang.org/x/text/language"
)

// WriteCatalog writes a .po file.
func WriteCatalog(
	w io.Writer, locale language.Tag, catalog *codeparser.Catalog, template bool,
) {
	pluralForms, ok := cldr.ByTag(locale)
	if !ok {
		pluralForms, ok = cldr.ByTag(locale)
		if !ok {
			panic(fmt.Errorf("unsupported locale: %v", locale))
		}
	}

	// Header
	if catalog.CopyrightNotice != "" {
		_, _ = fmt.Fprintf(w, "# %s\n", catalog.CopyrightNotice)
	}

	// Metadata block
	_, _ = fmt.Fprintln(w, `msgid ""`)
	_, _ = fmt.Fprintln(w, `msgstr ""`)
	if !catalog.LastRevision.DateTime.IsZero() {
		formatted := catalog.LastRevision.DateTime.Format(time.RFC3339)
		_, _ = fmt.Fprintf(w, "\"PO-Revision-Date: %s\"\n", formatted)
	}
	if template {
		if catalog.LastRevision.Translator != "" {
			_, _ = fmt.Fprint(w, "\"Last-Translator: \"\n")
		}
	} else {
		if catalog.LastRevision.Translator != "" {
			_, _ = fmt.Fprintf(w, "\"Last-Translator: %s\"\n", catalog.LastRevision.Translator)
		}
	}
	if catalog.BugsReportEmail != "" {
		_, _ = fmt.Fprintf(w, "\"Report-Msgid-Bugs-To: <%s>\"\n", catalog.BugsReportEmail)
	}
	if template {
		_, _ = fmt.Fprint(w, "\"Language: \\n\"\n")
	} else {
		_, _ = fmt.Fprintf(w, "\"Language: %s\\n\"\n", locale)
	}
	_, _ = fmt.Fprintln(w, "\"MIME-Version: 1.0\\n\"")
	_, _ = fmt.Fprintln(w, "\"Content-Type: text/plain; charset=UTF-8\\n\"")
	_, _ = fmt.Fprintln(w, "\"Content-Transfer-Encoding: 8bit\\n\"")
	_, _ = fmt.Fprintf(w, "\"Plural-Forms: %s\\n\"\n", pluralForms.GettextFormula)
	_, _ = fmt.Fprint(w, "\"X-Generator: "+
		"https://github.com/romshark/localize/cmd/localize\\n\"\n\n")

	for msg, meta := range catalog.Ordered() {
		for _, p := range meta.Pos {
			_, _ = fmt.Fprintf(w, "#: %s:%d:%d\n", p.Filename, p.Line, p.Column)
		}

		if msg.Description != "" {
			for l := range iterateLines(msg.Description) {
				_, _ = fmt.Fprintf(w, "#. %s\n", l)
			}
		}

		_, _ = fmt.Fprintf(w, "msgctxt %q\n", msg.Hash)
		_, _ = fmt.Fprintf(w, "msgid %q\n", msg.Other)

		switch msg.FuncType {
		case codeparser.FuncTypePlural, codeparser.FuncTypePluralBlock:
			_, _ = fmt.Fprintf(w, "msgid_plural %q\n", msg.Other)
			for i, f := range pluralForms.CardinalForms {
				var txt string
				switch f {
				case cldr.CLDRPluralFormZero:
					txt = msg.Zero
				case cldr.CLDRPluralFormOne:
					txt = msg.One
				case cldr.CLDRPluralFormTwo:
					txt = msg.Two
				case cldr.CLDRPluralFormFew:
					txt = msg.Few
				case cldr.CLDRPluralFormMany:
					txt = msg.Many
				case cldr.CLDRPluralFormOther:
					txt = msg.Other
				default:
					panic("unknown case: %q")
				}
				if template {
					_, _ = fmt.Fprintf(w, "msgstr[%d] \"\"\n", i)
				} else {
					_, _ = fmt.Fprintf(w, "msgstr[%d] %q\n", i, txt)
				}
			}
		default:
			// Other
			if template {
				_, _ = fmt.Fprintf(w, "msgstr \"\"\n")
			} else {
				_, _ = fmt.Fprintf(w, "msgstr %q\n", msg.Other)
			}
		}
		_, _ = fmt.Fprintln(w)
	}
}

func iterateLines(s string) iter.Seq[string] {
	return func(yield func(string) bool) {
		for {
			i := strings.IndexByte(s, '\n')
			if i == -1 {
				if !yield(s) {
					break
				}
				return
			}
			if !yield(s[:i]) {
				break
			}
			s = s[i+1:]
		}
	}
}
