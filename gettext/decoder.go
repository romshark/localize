package gettext

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"strconv"
	"strings"

	"golang.org/x/text/language"
)

func (p *Decoder) errSyntax(expected string) Error {
	return Error{Pos: p.pos, Expected: expected}
}

type Decoder struct {
	reader *bufio.Reader
	pos    Position
}

func NewDecoder() *Decoder {
	return &Decoder{
		reader: bufio.NewReader(nil),
	}
}

// DecodePO decodes a .po translation file from r.
func (p *Decoder) DecodePO(fileName string, r io.Reader) (FilePO, error) {
	f, err := p.decode(fileName, r, false)
	return FilePO{File: f}, err
}

// DecodePOT decodes a .pot template file from r.
func (p *Decoder) DecodePOT(fileName string, r io.Reader) (FilePOT, error) {
	f, err := p.decode(fileName, r, true)
	return FilePOT{File: f}, err
}

func (p *Decoder) decode(fileName string, r io.Reader, template bool) (*File, error) {
	p.reader.Reset(r)
	p.pos.Filename, p.pos.Index, p.pos.Line, p.pos.Column = fileName, 0, 1, 1

	var f File
	h, err := p.readHeader(template)
	if err != nil {
		return nil, err
	}
	f.Head = h

	for {
		err := p.readWhitespace()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}

		m, err := p.readMessage()
		if err != nil {
			return nil, err
		}
		f.Messages.List = append(f.Messages.List, m)
	}

	return &f, nil
}

func (p *Decoder) advanceByte(n uint32) {
	p.pos.Index += n
	p.pos.Column += n
}

func (p *Decoder) advanceLine() {
	p.pos.Index++
	p.pos.Line++
	p.pos.Column = 1
}

func (p *Decoder) span(start Position) Span {
	return Span{Position: start, Len: p.pos.Index - start.Index}
}

// readWhitespace reads spaces, tabs, carriage-returns and line-breaks.
func (p *Decoder) readWhitespace() error {
	for {
		b, err := p.reader.ReadByte()
		if err != nil {
			return err
		}
		switch b {
		case ' ', '\r', '\t':
			p.advanceByte(1)
			continue
		case '\n':
			p.advanceLine()
			continue
		}
		if err := p.reader.UnreadByte(); err != nil {
			panic(err) // Should never happen.
		}
		break
	}
	return nil
}

func (p *Decoder) readComment() (Comment, error) {
	start := p.pos

	b, err := p.reader.ReadByte()
	if err != nil {
		return Comment{}, err
	}

	if b != '#' {
		if err := p.reader.UnreadByte(); err != nil {
			panic(err) // Should never happen
		}
		return Comment{}, nil // Not a comment
	}

	p.advanceByte(1)

	var c Comment
	b, err = p.reader.ReadByte()
	if err != nil {
		return Comment{}, err
	}
	switch b {
	case '\n':
		// Empty comment
		c.Type = CommentTypeTranslator
		p.advanceLine()
		c.Span = p.span(start)
		return c, nil
	case ' ':
		c.Type = CommentTypeTranslator
	case '.':
		c.Type = CommentTypeExtracted
	case ':':
		c.Type = CommentTypeReference
	case ',':
		c.Type = CommentTypeFlag
	case '|':
		next, err := p.reader.Peek(len(" msgctxt"))
		if err != nil {
			return Comment{}, err
		}
		if string(next) == " msgctxt" {
			c.Type = CommentTypePreviousContext
			break
		}

		next, err = p.reader.Peek(len(" msgid"))
		if err != nil {
			return Comment{}, err
		}
		if string(next) == " msgid" {
			c.Type = CommentTypePreviousUntranslated
			break
		}

		return Comment{}, p.errSyntax("msgid or msgctxt")
	default:
		return Comment{}, p.errSyntax("space")
	}

	p.advanceByte(1)

	line, _, err := p.reader.ReadLine()
	if err != nil {
		return Comment{}, err
	}

	p.advanceByte(uint32(len(line)))
	p.advanceLine()
	c.Span = p.span(start)
	c.Value = string(line)
	return c, nil
}

func (p *Decoder) readComments() (Comments, error) {
	start := p.pos
	var l Comments
	for {
		c, err := p.readComment()
		if err != nil {
			return l, err
		}
		if c.Type == 0 {
			break
		}
		l.Text = append(l.Text, c)
	}
	l.Span = p.span(start)
	return l, nil
}

func (p *Decoder) readHeader(template bool) (FileHead, error) {
	start := p.pos
	if err := p.readWhitespace(); err != nil {
		return FileHead{}, err
	}

	var h FileHead

	stmt, err := p.readStatement()
	if err != nil {
		return FileHead{}, err
	}

	if stmt.statementType != statementTypeMsgid ||
		len(stmt.text.Lines) > 1 ||
		stmt.text.Lines[0].Value != "" {
		return FileHead{}, p.errSyntax("header msgid")
	}
	h.HeadComments = stmt.comments

	stmt, err = p.readStatement()
	if err != nil {
		return FileHead{}, err
	}
	if stmt.statementType != statementTypeMsgstr ||
		len(stmt.text.Lines) < 1 ||
		stmt.text.Lines[0].Value == "" {
		return FileHead{}, p.errSyntax("header msgstr")
	}
	if len(stmt.comments.Text) > 0 {
		return FileHead{}, Error{
			Pos:      p.pos,
			Expected: "header msgstr",
		}
	}

	for _, l := range stmt.text.Lines {
		pos := l.Position
		header, err := parseHeader(pos, l.Value)
		if err != nil {
			return FileHead{}, err
		}
		switch header.Name {
		case "Project-Id-Version":
			if err := setHeader(pos, &h.ProjectIdVersion, header); err != nil {
				return h, err
			}
		case "Report-Msgid-Bugs-To":
			if err := setHeader(pos, &h.ReportMsgidBugsTo, header); err != nil {
				return h, err
			}
		case "POT-Creation-Date":
			if err := setHeader(pos, &h.POTCreationDate, header); err != nil {
				return h, err
			}
		case "PO-Revision-Date":
			if err := setHeader(pos, &h.PORevisionDate, header); err != nil {
				return h, err
			}
		case "Last-Translator":
			if err := setHeader(pos, &h.LastTranslator, header); err != nil {
				return h, err
			}
		case "Language-Team":
			if err := setHeader(pos, &h.LanguageTeam, header); err != nil {
				return h, err
			}
		case "Language":
			if err := setHeader(pos, &h.Language.Header, header); err != nil {
				return h, err
			}
			if template && h.Language.Value != "" {
				return h, Error{
					Pos: pos,
					Err: ErrLanguageInTemplate,
				}
			} else {
				locale, err := language.Parse(h.Language.Value)
				if err != nil {
					return h, Error{
						Pos: pos,
						Err: ErrMalformedHeaderLanguage,
					}
				}
				h.Language.Locale = locale
			}
		case "MIME-Version":
			if err := setHeader(pos, &h.MIMEVersion, header); err != nil {
				return h, err
			}
			if h.MIMEVersion.Value != "1.0" {
				return h, Error{
					Pos: pos,
					Err: ErrUnsupportedMIMEVersion,
				}
			}
		case "Content-Type":
			if err := setHeader(pos, &h.ContentType, header); err != nil {
				return h, err
			}
			if _, _, err := mime.ParseMediaType(h.ContentType.Value); err != nil {
				return h, Error{
					Pos: pos,
					Err: ErrMalformedHeaderContentType,
				}
			}
			if h.ContentType.Value != "text/plain; charset=UTF-8" {
				return h, Error{
					Pos: pos,
					Err: ErrUnsupportedContentType,
				}
			}
		case "Content-Transfer-Encoding":
			if err := setHeader(pos, &h.ContentTransferEncoding, header); err != nil {
				return h, err
			}
			switch h.ContentTransferEncoding.Value {
			case "8bit":
				// OK
			default:
				return h, Error{
					Pos: pos,
					Err: ErrUnsupportedContentTransferEncoding,
				}
			}
		case "Plural-Forms":
			if err := setHeader(l.Position, &h.PluralForms, header); err != nil {
				return h, err
			}
			// TODO: validate
		default:
			if strings.HasPrefix(header.Name, "X-") {
				for _, nsh := range h.NonStandard {
					if nsh.Name == header.Name {
						return h, Error{
							Pos: pos,
							Err: ErrDuplicateHeader,
						}
					}
				}
				h.NonStandard = append(h.NonStandard, header)
				break
			}

			return h, Error{
				Pos: pos,
				Err: ErrUnsupportedHeader,
			}
		}
	}

	h.Span = p.span(start)
	return h, nil
}

var (
	prefixMsgctxt       = []byte("msgctxt ")
	prefixMsgid         = []byte("msgid ")
	prefixMsgidPlural   = []byte("msgid_plural ")
	prefixMsgstr        = []byte("msgstr ")
	prefixMsgstrIndexed = []byte("msgstr[")
	prefixLineBreak     = []byte("\n")
)

func (p *Decoder) readMessage() (m Message, err error) {
	start := p.pos

	var previous statement
LOOP:
	for {
		stmt, err := p.readStatement()
		if err != nil {
			if errors.Is(err, io.EOF) {
				switch previous.statementType {
				case 0:
					return m, Error{
						Pos:      p.pos,
						Expected: "msgctxt or msgid",
						Err:      io.ErrUnexpectedEOF,
					}
				case statementTypeMsgctxt:
					return m, Error{
						Pos:      p.pos,
						Expected: "msgid",
						Err:      io.ErrUnexpectedEOF,
					}
				case statementTypeMsgid:
					return m, Error{
						Pos:      p.pos,
						Expected: "msgid_plural or msgstr",
						Err:      io.ErrUnexpectedEOF,
					}
				case statementTypeMsgidPlural:
					return m, Error{
						Pos:      p.pos,
						Expected: "msgstr",
						Err:      io.ErrUnexpectedEOF,
					}
				case statementTypeMsgstr:
					// TODO: Check whether msgstr[n] was expected
					return m, nil
				case statementTypeMsgstrIndexed:
					// TODO: Check whether an index was still missing
					return m, nil
				}
			}
			return Message{}, err
		}

		// TODO: return correct expectations taking
		// into account the number of plural forms.
		switch stmt.statementType {
		case 0:
			break LOOP
		case statementTypeMsgctxt:
			switch previous.statementType {
			case 0:
				m.Msgctxt.Span = stmt.Span
				m.Msgctxt.Comments = stmt.comments
				m.Msgctxt.Text = stmt.text
			case statementTypeMsgctxt:
				return m, p.errSyntax("msgid")
			case statementTypeMsgid:
				return m, p.errSyntax("msgstr or msgid_plural")
			case statementTypeMsgidPlural:
				return m, p.errSyntax("msgstr[0]")
			case statementTypeMsgstr:
				return m, p.errSyntax("msgctxt or msgid")
			case statementTypeMsgstrIndexed:
				if stmt.pluralFormIndex <= previous.pluralFormIndex {
					return m, p.errSyntax(fmt.Sprintf("msgstr[%d]",
						previous.pluralFormIndex+1))
				}
			}
		case statementTypeMsgid:
			switch previous.statementType {
			case 0, statementTypeMsgctxt:
				m.Msgid.Span = stmt.Span
				m.Msgid.Comments = stmt.comments
				m.Msgid.Text = stmt.text
			case statementTypeMsgidPlural:
				return m, p.errSyntax("msgstr[0]")
			case statementTypeMsgstr:
				return m, p.errSyntax("msgctxt or msgid")
			case statementTypeMsgstrIndexed:
				if stmt.pluralFormIndex <= previous.pluralFormIndex {
					return m, p.errSyntax(fmt.Sprintf("msgstr[%d]",
						previous.pluralFormIndex+1))
				}
			}
		case statementTypeMsgidPlural:
			switch previous.statementType {
			case 0:
				return m, p.errSyntax("msgctxt or msgid")
			case statementTypeMsgctxt:
				return m, p.errSyntax("msgid")
			case statementTypeMsgid:
				m.MsgidPlural.Span = stmt.Span
				m.MsgidPlural.Comments = stmt.comments
				m.MsgidPlural.Text = stmt.text
			case statementTypeMsgidPlural:
				return m, p.errSyntax("msgstr[0]")
			case statementTypeMsgstr:
				return m, p.errSyntax("msgctxt or msgid")
			case statementTypeMsgstrIndexed:
				if stmt.pluralFormIndex <= previous.pluralFormIndex {
					return m, p.errSyntax(fmt.Sprintf("msgstr[%d]",
						previous.pluralFormIndex+1))
				}
			}
		case statementTypeMsgstr:
			switch previous.statementType {
			case 0:
				return m, p.errSyntax("msgctxt or msgid")
			case statementTypeMsgctxt:
				return m, p.errSyntax("msgid")
			case statementTypeMsgid:
				m.Msgstr.Span = stmt.Span
				m.Msgstr.Comments = stmt.comments
				m.Msgstr.Text = stmt.text
			case statementTypeMsgidPlural:
				return m, p.errSyntax("msgstr[0]")
			case statementTypeMsgstr:
				return m, p.errSyntax("msgctxt or msgid")
			case statementTypeMsgstrIndexed:
				return m, p.errSyntax("msgstr[n] or msgctxt or msgid")
			}
		case statementTypeMsgstrIndexed:
			switch previous.statementType {
			case 0:
				return m, p.errSyntax("msgctxt or msgid")
			case statementTypeMsgctxt:
				return m, p.errSyntax("msgid")
			case statementTypeMsgid:
				return m, p.errSyntax("msgid_plural or msgstr")
			case statementTypeMsgstr:
				return m, p.errSyntax("msgctxt or msgid")
			case statementTypeMsgidPlural, statementTypeMsgstrIndexed:
				var msg *Msgstr
				switch stmt.pluralFormIndex {
				case 0:
					msg = &m.Msgstr0
				case 1:
					msg = &m.Msgstr1
				case 2:
					msg = &m.Msgstr2
				case 3:
					msg = &m.Msgstr3
				case 4:
					msg = &m.Msgstr4
				case 5:
					msg = &m.Msgstr5
				default:
					panic(fmt.Errorf("unsupported plural form index: %d",
						stmt.pluralFormIndex)) // Should never happen.
				}
				if previous.statementType == statementTypeMsgstrIndexed &&
					stmt.pluralFormIndex <= previous.pluralFormIndex {
					return m, p.errSyntax(fmt.Sprintf("msgstr[%d]",
						previous.pluralFormIndex+1))
				}
				msg.Span = stmt.Span
				msg.Comments = stmt.comments
				msg.Text = stmt.text
			}
		}
		previous = stmt
	}

	m.Span = p.span(start)
	return m, nil
}

type statementType uint8

const (
	_ statementType = iota

	statementTypeMsgctxt       // msgctxt
	statementTypeMsgid         // msgid
	statementTypeMsgidPlural   // msgid_plural
	statementTypeMsgstr        // msgstr
	statementTypeMsgstrIndexed // msgstr[%d]
)

type statement struct {
	Span
	comments        Comments
	text            StringLiterals
	statementType   statementType
	pluralFormIndex uint8
}

// readStatement parses either msgctxt, msgid, msgid_plural and msgstr[%d]
func (p *Decoder) readStatement() (stmt statement, err error) {
	start := p.pos

	comments, err := p.readComments()
	if err != nil {
		return statement{}, err
	}
	stmt.comments = comments

	longestPossiblePrefixLen := len(prefixMsgidPlural)
	next, err := p.reader.Peek(longestPossiblePrefixLen)
	if err != nil {
		if !errors.Is(err, io.EOF) {
			return statement{}, err
		}
		// Ignore EOF for now.
	}

	if pr := prefixMsgctxt; bytes.HasPrefix(next, pr) {
		stmt.statementType = statementTypeMsgctxt
		p.advanceByte(uint32(len(pr)))
		if _, err := p.reader.Read(pr); err != nil {
			return stmt, err
		}

	} else if pr := prefixMsgidPlural; bytes.HasPrefix(next, pr) {
		stmt.statementType = statementTypeMsgidPlural
		p.advanceByte(uint32(len(pr)))
		if _, err := p.reader.Read(pr); err != nil {
			return stmt, err
		}

	} else if pr := prefixMsgid; bytes.HasPrefix(next, pr) {
		stmt.statementType = statementTypeMsgid
		p.advanceByte(uint32(len(pr)))
		if _, err := p.reader.Read(pr); err != nil {
			return stmt, err
		}

	} else if pr := prefixMsgstrIndexed; bytes.HasPrefix(next, pr) {
		stmt.statementType = statementTypeMsgstrIndexed
		p.advanceByte(uint32(len(pr) - 1))
		if _, err := p.reader.Read(pr[:len(pr)-1]); err != nil {
			return stmt, err
		}

		index, err := p.readPluralIndex()
		if err != nil {
			return stmt, err
		}
		stmt.pluralFormIndex = index

	} else if pr := prefixMsgstr; bytes.HasPrefix(next, pr) {
		stmt.statementType = statementTypeMsgstr
		p.advanceByte(uint32(len(pr)))
		if _, err := p.reader.Read(pr); err != nil {
			return stmt, err
		}

	} else if bytes.HasPrefix(next, prefixLineBreak) {
		return statement{}, nil
	} else {
		return stmt, p.errSyntax("statement")
	}

	strStart := p.pos
	str, err := p.readStringLiteral()
	if err != nil {
		return statement{}, err
	}

	if str.Value != "" {
		stmt.text = StringLiterals{
			Span:  p.span(strStart),
			Lines: []StringLiteral{str},
		}
		stmt.Span = p.span(start)
		return stmt, nil
	}

	// Multi-line
	b, err := p.peekByte()
	if errors.Is(err, io.EOF) || err == nil && b != '"' {
		// Empty string
		stmt.text = StringLiterals{
			Span: p.span(strStart),
			Lines: []StringLiteral{{
				Span:  p.span(strStart),
				Value: "",
			}},
		}
		return stmt, nil
	}
	if err != nil {
		return statement{}, err
	}

	strings, err := p.readStringLiterals()
	if err != nil {
		return stmt, err
	}

	stmt.text = strings
	stmt.Span = p.span(start)
	return stmt, nil
}

func (p *Decoder) readStringLiterals() (strings StringLiterals, err error) {
	start := p.pos
	next, err := p.reader.Peek(1)
	if err != nil {
		return strings, err
	}
	if next[0] != '"' {
		return strings, p.errSyntax("string literal")
	}

	for {
		next, err := p.reader.Peek(1)
		if err != nil {
			return strings, err
		}
		if next[0] != '"' {
			break
		}
		str, err := p.readStringLiteral()
		if err != nil {
			return strings, err
		}
		strings.Lines = append(strings.Lines, str)
	}
	strings.Span = p.span(start)
	return strings, nil
}

func (p *Decoder) readStringLiteral() (StringLiteral, error) {
	start := p.pos
	line, _, err := p.reader.ReadLine()
	if err != nil {
		return StringLiteral{}, err
	}
	var s StringLiteral

	trimmed := strings.TrimSpace(string(line))

	if len(trimmed) < 2 || trimmed[0] != '"' || trimmed[len(trimmed)-1] != '"' {
		return StringLiteral{}, p.errSyntax("string literal")
	}

	unquoted, err := strconv.Unquote(trimmed)
	if err != nil {
		return StringLiteral{}, Error{
			Pos:      p.pos,
			Expected: "string literal",
			Err:      err,
		}
	}

	p.advanceByte(uint32(len(line)))
	p.advanceLine()
	s.Value = unquoted
	s.Span = p.span(start)
	return s, nil
}

func (p *Decoder) readPluralIndex() (index uint8, err error) {
	b, err := p.reader.ReadByte()
	if err != nil {
		return 0, err
	}
	if b != '[' {
		return 0, p.errSyntax("[")
	}
	p.advanceByte(1)

	b, err = p.reader.ReadByte()
	if err != nil {
		return 0, err
	}
	if b < '0' || b > '9' {
		return 0, p.errSyntax("index 0-5")
	}
	p.advanceByte(1)

	index = b - '0'

	b, err = p.reader.ReadByte()
	if err != nil {
		return 0, err
	}
	if b != ']' {
		return 0, p.errSyntax("]")
	}
	p.advanceByte(1)

	b, err = p.reader.ReadByte()
	if err != nil {
		return 0, err
	}
	if b != ' ' {
		return 0, p.errSyntax("space")
	}
	p.advanceByte(1)

	return index, nil
}

func (p *Decoder) peekByte() (byte, error) {
	b, err := p.reader.ReadByte()
	if err != nil {
		return 0, err
	}
	if err := p.reader.UnreadByte(); err != nil {
		panic(err) // Should never happen.
	}
	return b, nil
}

func parseHeader(posLit Position, s string) (h Header, err error) {
	i := strings.IndexByte(s, ':')
	if i == -1 {
		return h, Error{
			Pos:      posLit,
			Expected: "colon",
			Err:      ErrMalformedHeader,
		}
	}
	h.Name = s[:i]
	s = s[i+1:]

	// Skip spaces
	for ; len(s) > 0; s = s[1:] {
		if s[0] == ' ' || s[0] == '\t' {
			continue
		}
		break
	}

	i = strings.IndexByte(s, '\n')
	if i == -1 {
		return h, Error{
			Pos:      posLit,
			Expected: "line break",
			Err:      ErrMalformedHeader,
		}
	}
	h.Value = s[:i]

	return h, nil
}

func setHeader(p Position, dst *Header, h Header) error {
	if dst.Name != "" {
		return Error{
			Pos: p,
			Err: ErrDuplicateHeader,
		}
	}
	*dst = h
	return nil
}
