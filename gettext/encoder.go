package gettext

import (
	"fmt"
	"io"
	"strings"
)

type Encoder struct{}

// Encode encodes a `.po` translation file to w.
func (e Encoder) EncodePO(f FilePO, w io.Writer) error {
	return e.encode(f.File, w, false)
}

// Encode encodes a `.pot` template file to w.
func (e Encoder) EncodePOT(f FilePOT, w io.Writer) error {
	return e.encode(f.File, w, true)
}

func (e Encoder) encode(f *File, w io.Writer, template bool) error {
	if err := e.encodeComments(w, f.Head.HeadComments, false); err != nil {
		return err
	}

	if _, err := fmt.Fprint(w, "msgid \"\"\nmsgstr \"\"\n"); err != nil {
		return err
	}

	if f.Head.ProjectIdVersion != "" {
		if _, err := fmt.Fprintf(w, "\"Project-Id-Version: %s\\n\"\n",
			f.Head.ProjectIdVersion); err != nil {
			return err
		}
	}
	if f.Head.ReportMsgidBugsTo != "" {
		if _, err := fmt.Fprintf(w, "\"Report-Msgid-Bugs-To: %s\\n\"\n",
			f.Head.ReportMsgidBugsTo); err != nil {
			return err
		}
	}
	if f.Head.POTCreationDate != "" {
		if _, err := fmt.Fprintf(w, "\"POT-Creation-Date: %s\\n\"\n",
			f.Head.POTCreationDate); err != nil {
			return err
		}
	}
	if f.Head.PORevisionDate != "" {
		if _, err := fmt.Fprintf(w, "\"PO-Revision-Date: %s\\n\"\n",
			f.Head.PORevisionDate); err != nil {
			return err
		}
	}
	if f.Head.LastTranslator != "" {
		if _, err := fmt.Fprintf(w, "\"Last-Translator: %s\\n\"\n",
			f.Head.LastTranslator); err != nil {
			return err
		}
	}
	if f.Head.LanguageTeam != "" {
		if _, err := fmt.Fprintf(w, "\"Language-Team: %s\\n\"\n",
			f.Head.LanguageTeam); err != nil {
			return err
		}
	}
	if f.Head.Language.Value != "" {
		if _, err := fmt.Fprintf(w, "\"Language: %s\\n\"\n",
			f.Head.Language.Value); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "\"MIME-Version: %s\\n\"\n",
		f.Head.MIMEVersion); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\"Content-Type: %s\\n\"\n",
		f.Head.ContentType); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\"Content-Transfer-Encoding: %s\\n\"\n",
		f.Head.ContentTransferEncoding); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\"Plural-Forms: %s\\n\"\n",
		f.Head.PluralForms); err != nil {
		return err
	}
	for _, h := range f.Head.NonStandard {
		if _, err := fmt.Fprintf(w, "\"%s: %s\\n\"\n", h.Name, h.Value); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	for i, m := range f.Messages.List {
		if template && m.Obsolete {
			// Don't encode obsolete messages in .pot files
			continue
		}

		if err := e.printDirective(
			w, "msgctxt", m.Obsolete, m.Msgctxt.Comments, m.Msgctxt.Text,
		); err != nil {
			return err
		}
		if err := e.printDirective(
			w, "msgid", m.Obsolete, m.Msgid.Comments, m.Msgid.Text,
		); err != nil {
			return err
		}
		if err := e.printDirective(
			w, "msgid_plural", m.Obsolete, m.MsgidPlural.Comments, m.MsgidPlural.Text,
		); err != nil {
			return err
		}
		if err := e.printDirective(
			w, "msgstr", m.Obsolete, m.Msgstr.Comments, m.Msgstr.Text,
		); err != nil {
			return err
		}
		if err := e.printDirective(
			w, "msgstr[0]", m.Obsolete, m.Msgstr0.Comments, m.Msgstr0.Text,
		); err != nil {
			return err
		}
		if err := e.printDirective(
			w, "msgstr[1]", m.Obsolete, m.Msgstr1.Comments, m.Msgstr1.Text,
		); err != nil {
			return err
		}
		if err := e.printDirective(
			w, "msgstr[2]", m.Obsolete, m.Msgstr2.Comments, m.Msgstr2.Text,
		); err != nil {
			return err
		}
		if err := e.printDirective(
			w, "msgstr[3]", m.Obsolete, m.Msgstr3.Comments, m.Msgstr3.Text,
		); err != nil {
			return err
		}
		if err := e.printDirective(
			w, "msgstr[4]", m.Obsolete, m.Msgstr4.Comments, m.Msgstr4.Text,
		); err != nil {
			return err
		}
		if err := e.printDirective(
			w, "msgstr[5]", m.Obsolete, m.Msgstr5.Comments, m.Msgstr5.Text,
		); err != nil {
			return err
		}
		if i+1 < len(f.Messages.List) {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
	}

	return nil
}

func (e *Encoder) encodeComments(w io.Writer, c Comments, obsolete bool) error {
	for _, c := range c.Text {
		if obsolete {
			if _, err := fmt.Fprint(w, "#~ "); err != nil {
				return err
			}
		}
		switch c.Type {
		case CommentTypeExtracted:
			if err := printLines(w, "#. ", c.Value); err != nil {
				return err
			}
		case CommentTypeReference:
			if err := printLines(w, "#: ", c.Value); err != nil {
				return err
			}
		case CommentTypeFlag:
			if err := printLines(w, "#, ", c.Value); err != nil {
				return err
			}
		default:
			// Treat everything else as translator comment
			if c.Value == "" {
				if _, err := fmt.Fprintln(w, "#"); err != nil {
					return err
				}
				continue
			}
			if err := printLines(w, "# ", c.Value); err != nil {
				return err
			}
		}
	}
	return nil
}

func printLines(w io.Writer, prefix, s string) error {
	for len(s) > 0 {
		i := strings.IndexByte(s, '\n')
		var line string
		if i == -1 {
			line, s = s, ""
		} else {
			line, s = s[:i], s[i+1:]
		}
		if _, err := fmt.Fprint(w, prefix); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

func (e *Encoder) printDirective(
	w io.Writer, name string, obsolete bool, comments Comments, text StringLiterals,
) error {
	if len(text.Lines) < 1 {
		// Nothing to write
		return nil
	}
	if err := e.encodeComments(w, comments, obsolete); err != nil {
		return err
	}
	if obsolete {
		if _, err := fmt.Fprint(w, "#~ "); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprint(w, name); err != nil {
		return err
	}
	if len(text.Lines) == 1 {
		if _, err := fmt.Fprintf(w, " %q\n",
			text.Lines[0].Value); err != nil {
			return err
		}
		return nil
	}

	// Multi-line
	if _, err := fmt.Fprintln(w, " \"\""); err != nil {
		return err
	}
	for _, l := range text.Lines {
		if obsolete {
			if _, err := fmt.Fprint(w, "#~ "); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "%q\n", l.Value); err != nil {
			return err
		}
	}
	return nil
}
