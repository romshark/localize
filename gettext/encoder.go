package gettext

import (
	"fmt"
	"io"
)

type Encoder struct {
	// NoLocation disables "#:" (reference) comments.
	// NoLocation bool
}

// Encode encodes a `.po` translation file to w.
func (e Encoder) EncodePO(f FilePO, w io.Writer) error { return e.encode(f.File, w) }

// Encode encodes a `.pot` template file to w.
func (e Encoder) EncodePOT(f FilePOT, w io.Writer) error { return e.encode(f.File, w) }

func (e Encoder) encode(f *File, w io.Writer) error {
	if err := e.encodeComments(w, f.Head.HeadComments); err != nil {
		return err
	}

	if _, err := fmt.Fprint(w, "msgid \"\"\nmsgstr \"\"\n"); err != nil {
		return err
	}

	if f.Head.ProjectIdVersion.Value != "" {
		if _, err := fmt.Fprintf(w, "\"Project-Id-Version: %s\\n\"\n",
			f.Head.ProjectIdVersion.Value); err != nil {
			return err
		}
	}
	if f.Head.ReportMsgidBugsTo.Value != "" {
		if _, err := fmt.Fprintf(w, "\"Report-Msgid-Bugs-To: %s\\n\"\n",
			f.Head.ReportMsgidBugsTo.Value); err != nil {
			return err
		}
	}
	if f.Head.POTCreationDate.Value != "" {
		if _, err := fmt.Fprintf(w, "\"POT-Creation-Date: %s\\n\"\n",
			f.Head.POTCreationDate.Value); err != nil {
			return err
		}
	}
	if f.Head.PORevisionDate.Value != "" {
		if _, err := fmt.Fprintf(w, "\"PO-Revision-Date: %s\\n\"\n",
			f.Head.PORevisionDate.Value); err != nil {
			return err
		}
	}
	if f.Head.LastTranslator.Value != "" {
		if _, err := fmt.Fprintf(w, "\"Last-Translator: %s\\n\"\n",
			f.Head.LastTranslator.Value); err != nil {
			return err
		}
	}
	if f.Head.LanguageTeam.Value != "" {
		if _, err := fmt.Fprintf(w, "\"Language-Team: %s\\n\"\n",
			f.Head.LanguageTeam.Value); err != nil {
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
		f.Head.MIMEVersion.Value); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\"Content-Type: %s\\n\"\n",
		f.Head.ContentType.Value); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\"Content-Transfer-Encoding: %s\\n\"\n",
		f.Head.ContentTransferEncoding.Value); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\"Plural-Forms: %s\\n\"\n",
		f.Head.PluralForms.Value); err != nil {
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
		if !m.Msgctxt.IsZero() {
			if err := e.encodeComments(w, m.Msgctxt.Comments); err != nil {
				return err
			}
			if len(m.Msgctxt.Text.Lines) == 1 {
				if _, err := fmt.Fprintf(w, "msgctxt %q\n",
					m.Msgctxt.Text.Lines[0].Value); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintln(w, "msgctxt \"\""); err != nil {
					return err
				}
				for _, l := range m.Msgctxt.Text.Lines {
					if _, err := fmt.Fprintf(w, "%q\n", l.Value); err != nil {
						return err
					}
				}
			}
		}
		if !m.Msgid.IsZero() {
			if err := e.encodeComments(w, m.Msgid.Comments); err != nil {
				return err
			}
			if len(m.Msgid.Text.Lines) == 1 {
				if _, err := fmt.Fprintf(w, "msgid %q\n",
					m.Msgid.Text.Lines[0].Value); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintln(w, "msgid \"\""); err != nil {
					return err
				}
				for _, l := range m.Msgid.Text.Lines {
					if _, err := fmt.Fprintf(w, "%q\n", l.Value); err != nil {
						return err
					}
				}
			}
		}
		if !m.MsgidPlural.IsZero() {
			if err := e.encodeComments(w, m.MsgidPlural.Comments); err != nil {
				return err
			}
			if len(m.MsgidPlural.Text.Lines) == 1 {
				if _, err := fmt.Fprintf(w, "msgid_plural %q\n",
					m.MsgidPlural.Text.Lines[0].Value); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintln(w, "msgid_plural \"\""); err != nil {
					return err
				}
				for _, l := range m.MsgidPlural.Text.Lines {
					if _, err := fmt.Fprintf(w, "%q\n", l.Value); err != nil {
						return err
					}
				}
			}
		}
		if !m.Msgstr.IsZero() {
			if err := e.encodeComments(w, m.Msgstr.Comments); err != nil {
				return err
			}
			if len(m.Msgstr.Text.Lines) == 1 {
				if _, err := fmt.Fprintf(w, "msgstr %q\n",
					m.Msgstr.Text.Lines[0].Value); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintln(w, "msgstr \"\""); err != nil {
					return err
				}
				for _, l := range m.Msgstr.Text.Lines {
					if _, err := fmt.Fprintf(w, "%q\n", l.Value); err != nil {
						return err
					}
				}
			}
		}
		if !m.Msgstr0.IsZero() {
			if err := e.encodeComments(w, m.Msgstr0.Comments); err != nil {
				return err
			}
			if len(m.Msgstr0.Text.Lines) == 1 {
				if _, err := fmt.Fprintf(w, "msgstr[0] %q\n",
					m.Msgstr0.Text.Lines[0].Value); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintln(w, "msgstr[0] \"\""); err != nil {
					return err
				}
				for _, l := range m.Msgstr0.Text.Lines {
					if _, err := fmt.Fprintf(w, "%q\n", l.Value); err != nil {
						return err
					}
				}
			}
		}
		if !m.Msgstr1.IsZero() {
			if err := e.encodeComments(w, m.Msgstr1.Comments); err != nil {
				return err
			}
			if len(m.Msgstr1.Text.Lines) == 1 {
				if _, err := fmt.Fprintf(w, "msgstr[1] %q\n",
					m.Msgstr1.Text.Lines[0].Value); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintln(w, "msgstr[1] \"\""); err != nil {
					return err
				}
				for _, l := range m.Msgstr1.Text.Lines {
					if _, err := fmt.Fprintf(w, "%q\n", l.Value); err != nil {
						return err
					}
				}
			}
		}
		if !m.Msgstr2.IsZero() {
			if err := e.encodeComments(w, m.Msgstr2.Comments); err != nil {
				return err
			}
			if len(m.Msgstr2.Text.Lines) == 1 {
				if _, err := fmt.Fprintf(w, "msgstr[2] %q\n",
					m.Msgstr2.Text.Lines[0].Value); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintln(w, "msgstr[2] \"\""); err != nil {
					return err
				}
				for _, l := range m.Msgstr2.Text.Lines {
					if _, err := fmt.Fprintf(w, "%q\n", l.Value); err != nil {
						return err
					}
				}
			}
		}
		if !m.Msgstr3.IsZero() {
			if err := e.encodeComments(w, m.Msgstr3.Comments); err != nil {
				return err
			}
			if len(m.Msgstr3.Text.Lines) == 1 {
				if _, err := fmt.Fprintf(w, "msgstr[3] %q\n",
					m.Msgstr3.Text.Lines[0].Value); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintln(w, "msgstr[3] \"\""); err != nil {
					return err
				}
				for _, l := range m.Msgstr3.Text.Lines {
					if _, err := fmt.Fprintf(w, "%q\n", l.Value); err != nil {
						return err
					}
				}
			}
		}
		if !m.Msgstr4.IsZero() {
			if err := e.encodeComments(w, m.Msgstr4.Comments); err != nil {
				return err
			}
			if len(m.Msgstr4.Text.Lines) == 1 {
				if _, err := fmt.Fprintf(w, "msgstr[4] %q\n",
					m.Msgstr4.Text.Lines[0].Value); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintln(w, "msgstr[4] \"\""); err != nil {
					return err
				}
				for _, l := range m.Msgstr4.Text.Lines {
					if _, err := fmt.Fprintf(w, "%q\n", l.Value); err != nil {
						return err
					}
				}
			}
		}
		if !m.Msgstr5.IsZero() {
			if err := e.encodeComments(w, m.Msgstr5.Comments); err != nil {
				return err
			}
			if len(m.Msgstr5.Text.Lines) == 1 {
				if _, err := fmt.Fprintf(w, "msgstr[5] %q\n",
					m.Msgstr5.Text.Lines[0].Value); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintln(w, "msgstr[5] \"\""); err != nil {
					return err
				}
				for _, l := range m.Msgstr5.Text.Lines {
					if _, err := fmt.Fprintf(w, "%q\n", l.Value); err != nil {
						return err
					}
				}
			}
		}
		if i+1 < len(f.Messages.List) {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
	}

	return nil
}

func (e *Encoder) encodeComments(w io.Writer, c Comments) error {
	for _, c := range c.Text {
		switch c.Type {
		case CommentTypeTranslator:
			if c.Value == "" {
				if _, err := fmt.Fprintln(w, "#"); err != nil {
					return err
				}
				continue
			}
			if _, err := fmt.Fprint(w, "# "); err != nil {
				return err
			}
		case CommentTypeExtracted:
			if _, err := fmt.Fprint(w, "#."); err != nil {
				return err
			}
		case CommentTypeReference:
			if _, err := fmt.Fprint(w, "#:"); err != nil {
				return err
			}
		case CommentTypeFlag:
			if _, err := fmt.Fprint(w, "#,"); err != nil {
				return err
			}
		case CommentTypePreviousContext:
			if _, err := fmt.Fprint(w, "#| msgctxt"); err != nil {
				return err
			}
		case CommentTypePreviousUntranslated:
			if _, err := fmt.Fprint(w, "#| msgid"); err != nil {
				return err
			}
		default:
			return nil
		}
		if _, err := fmt.Fprintln(w, c.Value); err != nil {
			return err
		}
	}
	return nil
}
