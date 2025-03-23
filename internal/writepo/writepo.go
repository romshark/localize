// Package writepo provides functions to marshal catalogs into
// gettext compatible .po files.
package writepo

import (
	"fmt"
	"io"
	"iter"
	"strings"
	"time"

	"github.com/romshark/localize/internal/codeparser"
	"github.com/romshark/localize/internal/pluralform"
	"golang.org/x/text/language"
)

// WriteCatalog writes a .po file.
func WriteCatalog(
	w io.Writer, locale language.Tag, catalog *codeparser.Catalog,
) {
	// Header
	if catalog.CopyrightNotice != "" {
		fmt.Fprintf(w, "# %s\n", catalog.CopyrightNotice)
	}

	// Metadata block
	fmt.Fprintln(w, `msgid ""`)
	fmt.Fprintln(w, `msgstr ""`)
	if !catalog.LastRevision.DateTime.IsZero() {
		formatted := catalog.LastRevision.DateTime.Format(time.RFC3339)
		fmt.Fprintf(w, "\"PO-Revision-Date: %s\"\n", formatted)
	}
	if catalog.LastRevision.Translator != "" {
		fmt.Fprintf(w, "\"Last-Translator: %s\"\n", catalog.LastRevision.Translator)
	}
	if catalog.BugsReportEmail != "" {
		fmt.Fprintf(w, "\"Report-Msgid-Bugs-To: <%s>\"\n", catalog.BugsReportEmail)
	}
	fmt.Fprintf(w, "\"Language: %s\\n\"\n", locale)
	fmt.Fprintln(w, "\"MIME-Version: 1.0\\n\"")
	fmt.Fprintln(w, "\"Content-Type: text/plain; charset=UTF-8\\n\"")
	fmt.Fprintln(w, "\"Content-Transfer-Encoding: 8bit\\n\"")
	fmt.Fprintf(w, "\"Plural-Forms: %s\\n\"\n", pluralform.ByTag(locale).GettextFormula)
	fmt.Fprint(w, "\"X-Generator: "+
		"https://github.com/romshark/localize/cmd/localize\\n\"\n\n")

	for msg, meta := range catalog.Ordered() {
		for _, p := range meta.Pos {
			fmt.Fprintf(w, "#: %s:%d:%d\n", p.Filename, p.Line, p.Column)
		}

		if msg.Description != "" {
			for l := range iterateLines(msg.Description) {
				fmt.Fprintf(w, "#. %s\n", l)
			}
		}

		fmt.Fprintf(w, "msgid %q\n", msg.Hash)

		switch msg.FuncType {
		case codeparser.FuncTypePlural, codeparser.FuncTypePluralBlock:
			// Other
			fmt.Fprintf(w, "msgid_plural %q\n\n", msg.Other)
			forms := pluralform.ByTag(locale)
			if forms.Zero {
				fmt.Fprintf(w, "#. zero\nmsgstr[0] %q\n", msg.Zero)
			}
			if forms.One {
				fmt.Fprintf(w, "#. one\nmsgstr[1] %q\n", msg.One)
			}
			if forms.Two {
				fmt.Fprintf(w, "#. two\nmsgstr[2] %q\n", msg.Two)
			}
			if forms.Few {
				fmt.Fprintf(w, "#. few\nmsgstr[3] %q\n", msg.Few)
			}
			if forms.Many {
				fmt.Fprintf(w, "#. many\nmsgstr[4] %q\n", msg.Many)
			}
		default:
			// Other
			fmt.Fprintf(w, "msgstr %q\n\n", msg.Other)
		}
	}
}

// WriteTemplate writes a .pot file
func WriteTemplate(
	w io.Writer, locale, origLocale language.Tag, catalog *codeparser.Catalog,
) {
	fmt.Fprint(w, "# Generated by romshark/localize/cmd/extract. DO NOT EDIT.\n")
	fmt.Fprintf(w, "# language: %q\n\n", locale)

	for msg, meta := range catalog.Messages {
		_ = msg
		for _, p := range meta.Pos {
			fmt.Fprintf(w, "# %s:%d:%d\n", p.Filename, p.Line, p.Column)
		}

		fmt.Fprintf(w, "# %s:", origLocale)
		// if cldrPluralFormsCardinal.Zero && m.Zero != "" {
		// 	fmt.Fprint(w, "zero = \"\"\n")
		// }
		// if cldrPluralFormsCardinal.One {
		// 	fmt.Fprint(w, "one = \"\"\n")
		// }
		// if cldrPluralFormsCardinal.Two {
		// 	fmt.Fprint(w, "two = \"\"\n")
		// }
		// if cldrPluralFormsCardinal.Few {
		// 	fmt.Fprint(w, "few = \"\"\n")
		// }
		// if cldrPluralFormsCardinal.Many {
		// 	fmt.Fprint(w, "many = \"\"\n")
		// }

		// fmt.Fprintf(w, "[%s]\n", messageHash(msg.Text, msg.Description))
		// if msg.Description != "" {
		// 	for l := range iterateLines(msg.Description) {
		// 		fmt.Fprintf(w, "description = %q\n", msg.Description)
		// 	}
		// }

		// if msg.FuncType == codeparser.FuncTypeCardinal {
		// 	if cldrPluralFormsCardinal.Zero {
		// 		fmt.Fprint(w, "zero = \"\"\n")
		// 	}
		// 	if cldrPluralFormsCardinal.One {
		// 		fmt.Fprint(w, "one = \"\"\n")
		// 	}
		// 	if cldrPluralFormsCardinal.Two {
		// 		fmt.Fprint(w, "two = \"\"\n")
		// 	}
		// 	if cldrPluralFormsCardinal.Few {
		// 		fmt.Fprint(w, "few = \"\"\n")
		// 	}
		// 	if cldrPluralFormsCardinal.Many {
		// 		fmt.Fprint(w, "many = \"\"\n")
		// 	}
		// }

		// Other
		fmt.Fprint(w, "other = \"\"\n\n")
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
