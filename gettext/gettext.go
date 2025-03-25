// Package gettext provides GNU gettext `.pot` and `.po` file decoder and encoder.
//
// WARNING: This encoder and decoder implementation is optimized to handle the needs
// of github.com/romshark/localize only and may not be fully spec compliant!
package gettext

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/text/language"
)

type Position struct {
	Filename     string
	Index        uint32
	Line, Column uint32
}

type Span struct {
	Position
	Len uint32
}

func (s Span) IsZero() bool { return s.Len == 0 }

type Message struct {
	Span
	Msgctxt     Msgctxt
	Msgid       Msgid
	MsgidPlural MsgidPlural
	Msgstr      MsgidStr
	Msgstr0     MsgidStr
	Msgstr1     MsgidStr
	Msgstr2     MsgidStr
	Msgstr3     MsgidStr
	Msgstr4     MsgidStr
	Msgstr5     MsgidStr
}

// Clone returns a deep copy of m.
func (m Message) Clone() Message {
	cp := m
	cp.Msgctxt.Comments = m.Msgctxt.Comments.Clone()
	cp.Msgctxt.Text = m.Msgctxt.Text.Clone()
	cp.Msgid.Comments = m.Msgid.Comments.Clone()
	cp.Msgid.Text = m.Msgid.Text.Clone()
	cp.MsgidPlural.Comments = m.MsgidPlural.Comments.Clone()
	cp.MsgidPlural.Text = m.MsgidPlural.Text.Clone()
	cp.Msgstr.Comments = m.Msgstr.Comments.Clone()
	cp.Msgstr.Text = m.Msgstr.Text.Clone()
	cp.Msgstr0.Comments = m.Msgstr0.Comments.Clone()
	cp.Msgstr0.Text = m.Msgstr0.Text.Clone()
	cp.Msgstr1.Comments = m.Msgstr1.Comments.Clone()
	cp.Msgstr1.Text = m.Msgstr1.Text.Clone()
	cp.Msgstr2.Comments = m.Msgstr2.Comments.Clone()
	cp.Msgstr2.Text = m.Msgstr2.Text.Clone()
	cp.Msgstr3.Comments = m.Msgstr3.Comments.Clone()
	cp.Msgstr3.Text = m.Msgstr3.Text.Clone()
	cp.Msgstr4.Comments = m.Msgstr4.Comments.Clone()
	cp.Msgstr4.Text = m.Msgstr4.Text.Clone()
	cp.Msgstr5.Comments = m.Msgstr5.Comments.Clone()
	cp.Msgstr5.Text = m.Msgstr5.Text.Clone()
	return cp
}

type Msgctxt struct {
	Span
	Comments Comments
	Text     StringLiterals
}

type Msgid struct {
	Span
	Comments Comments
	Text     StringLiterals
}

type MsgidPlural struct {
	Span
	Comments Comments
	Text     StringLiterals
}

type MsgidStr struct {
	Span
	Comments Comments
	Text     StringLiterals
}

type Comment struct {
	Span
	Type  CommentType
	Value string
}

type StringLiterals struct {
	Span
	Lines []StringLiteral
}

func (l StringLiterals) String() string {
	switch len(l.Lines) {
	case 0:
		return ""
	case 1:
		return l.Lines[0].Value
	}
	// Concatenate all lines.
	var b strings.Builder
	for _, l := range l.Lines {
		b.WriteString(l.Value)
	}
	return b.String()
}

// Clone returns a deep copy of s.
func (s StringLiterals) Clone() StringLiterals {
	lines := make([]StringLiteral, len(s.Lines))
	copy(lines, s.Lines)
	return StringLiterals{Lines: lines}
}

type StringLiteral struct {
	Span
	Value string
}

// FilePO is a `.po` translation file.
type FilePO struct{ *File }

// MakePOT returns a new `.pot` template file from f.
func (f FilePO) MakePOT() FilePOT {
	cp := f.Clone()
	cp.Head.Language = HeaderLanguage{}
	cp.Head.LastTranslator = Header{}
	cp.Head.PORevisionDate = Header{}
	cp.Head.LanguageTeam = Header{}
	for i, m := range f.Messages.List {
		m.Msgstr.Text = StringLiterals{}
		m.Msgstr0.Text = StringLiterals{}
		m.Msgstr1.Text = StringLiterals{}
		m.Msgstr2.Text = StringLiterals{}
		m.Msgstr3.Text = StringLiterals{}
		m.Msgstr4.Text = StringLiterals{}
		m.Msgstr5.Text = StringLiterals{}
		cp.Messages.List[i] = m
	}
	return FilePOT{File: cp}
}

// FilePOT is a `.pot` template file.
type FilePOT struct{ *File }

type File struct {
	Head     FileHead
	Messages Messages
}

type Messages struct {
	List []Message
}

// Clone returns a deep copy of m.
func (m Messages) Clone() Messages {
	cp := make([]Message, len(m.List))
	for i, m := range m.List {
		cp[i] = m.Clone()
	}
	return Messages{List: cp}
}

// Clone returns a deep copy of f.
func (f *File) Clone() *File {
	cp := *f
	cp.Head = f.Head.Clone()
	cp.Messages = f.Messages.Clone()
	return &cp
}

type FileHead struct {
	Span
	HeadComments            Comments
	ProjectIdVersion        Header
	ReportMsgidBugsTo       Header
	POTCreationDate         Header
	PORevisionDate          Header
	LastTranslator          Header
	LanguageTeam            Header
	Language                HeaderLanguage
	MIMEVersion             Header
	ContentType             Header
	ContentTransferEncoding Header
	PluralForms             Header
	NonStandard             []Header
}

// Clone returns a deep copy of h.
func (f FileHead) Clone() FileHead {
	cp := f
	cp.HeadComments = f.HeadComments.Clone()
	cp.NonStandard = make([]Header, len(f.NonStandard))
	copy(cp.NonStandard, f.NonStandard)
	return cp
}

type Header struct{ Name, Value string }

type HeaderLanguage struct {
	Header
	Locale language.Tag
}

type Comments struct {
	Span
	Text []Comment
}

// Clone returns a deep copy of c.
func (c Comments) Clone() Comments {
	text := make([]Comment, len(c.Text))
	copy(text, c.Text)
	return Comments{Text: text}
}

type CommentType uint8

const (
	_ CommentType = iota

	CommentTypeTranslator           // #  translator-comments
	CommentTypeExtracted            // #. extracted-comments
	CommentTypeReference            // #: reference...
	CommentTypeFlag                 // #, flag...
	CommentTypePreviousContext      // #| msgctxt previous-context
	CommentTypePreviousUntranslated // #| msgid previous-untranslated-string
)

type Error struct {
	Pos      Position
	Expected string
	Err      error
}

func (e Error) Error() string {
	err := e.Err
	if err == nil {
		err = ErrUnexpectedToken
	}
	if e.Expected == "" {
		return fmt.Sprintf("%s:%d:%d: %s",
			e.Pos.Filename, e.Pos.Line, e.Pos.Column, err.Error())
	}
	return fmt.Sprintf("%s:%d:%d: expected %s; %s",
		e.Pos.Filename, e.Pos.Line, e.Pos.Column, e.Expected, err.Error())
}

var (
	ErrUnexpectedToken         = errors.New("found unexpected token")
	ErrMalformedHeader         = errors.New("malformed header")
	ErrDuplicateHeader         = errors.New("duplicate header")
	ErrMalformedHeaderLanguage = errors.New(
		"malformed Language header, must be BCP 47")
	ErrLanguageInTemplate = errors.New(
		"header Language must be kept empty in .pot file")
	ErrMalformedHeaderContentType         = errors.New("malformed Content-Type header")
	ErrUnsupportedHeader                  = errors.New("unsupported header")
	ErrUnsupportedContentTransferEncoding = errors.New(
		"unsupported Content-Transfer-Encoding")
	ErrUnsupportedContentType = errors.New(
		"unsupported Content-Type, use \"text/plain; charset=UTF-8\"")
	ErrUnsupportedMIMEVersion = errors.New(
		"unsupported MIME-Version, use \"1.0\"")
)
