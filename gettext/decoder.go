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

func (d *Decoder) errSyntax(expected string) Error {
	return Error{Pos: d.pos, Expected: expected}
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
func (d *Decoder) DecodePO(fileName string, r io.Reader) (FilePO, error) {
	f, err := d.decode(fileName, r, false)
	return FilePO{File: f}, err
}

// DecodePOT decodes a .pot template file from r.
func (d *Decoder) DecodePOT(fileName string, r io.Reader) (FilePOT, error) {
	f, err := d.decode(fileName, r, true)
	return FilePOT{File: f}, err
}

func (d *Decoder) decode(fileName string, r io.Reader, template bool) (*File, error) {
	d.reader.Reset(r)
	d.pos.Filename, d.pos.Index, d.pos.Line, d.pos.Column = fileName, 0, 1, 1

	var f File
	mHead, err := d.readMessage()
	if err != nil {
		return nil, err
	}
	f.Head, err = d.parseHead(mHead, template)
	if err != nil {
		return nil, err
	}

	for {
		err := d.readOptionalWhitespace()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}

		m, err := d.readMessage()
		if err != nil {
			return nil, err
		}
		f.Messages.List = append(f.Messages.List, m)
	}

	return &f, nil
}

func (d *Decoder) advance(str []byte) {
	for _, c := range str {
		switch c {
		case '\n':
			d.advanceLine()
		default:
			d.advanceByte(1)
		}
	}
}

func (d *Decoder) advanceByte(n uint32) {
	d.pos.Index += n
	d.pos.Column += n
}

func (d *Decoder) advanceLine() {
	d.pos.Index++
	d.pos.Line++
	d.pos.Column = 1
}

func (d *Decoder) span(start Position) Span {
	return Span{Position: start, Len: d.pos.Index - start.Index}
}

// readOptionalWhitespace reads spaces, tabs, carriage-returns and line-breaks.
func (d *Decoder) readOptionalWhitespace() error {
	for {
		b, err := d.reader.ReadByte()
		if err != nil {
			return err
		}
		switch b {
		case ' ', '\r', '\t':
			d.advanceByte(1)
			continue
		case '\n':
			d.advanceLine()
			continue
		}
		if err := d.reader.UnreadByte(); err != nil {
			panic(err) // Should never happen.
		}
		break
	}
	return nil
}

func (d *Decoder) readComment() (Comment, error) {
	start := d.pos

	b, err := d.reader.ReadByte()
	if err != nil {
		return Comment{}, err
	}

	if b != '#' {
		if err := d.reader.UnreadByte(); err != nil {
			panic(err) // Should never happen
		}
		return Comment{}, nil // Not a comment
	}

	d.advanceByte(1)

	var c Comment
	b, err = d.reader.ReadByte()
	if err != nil {
		return Comment{}, err
	}
	switch b {
	case '\n':
		// Empty comment
		c.Type = CommentTypeTranslator
		d.advanceLine()
		c.Span = d.span(start)
		return c, nil
	case ' ':
		c.Type = CommentTypeTranslator
		d.advanceByte(1)
	case '.':
		c.Type = CommentTypeExtracted
		d.advanceByte(1)
		b, err = d.reader.ReadByte()
		if err != nil {
			return Comment{}, err
		}
		if b != ' ' {
			return Comment{}, d.errSyntax("space")
		}
		d.advanceByte(1)
	case ':':
		c.Type = CommentTypeReference
		d.advanceByte(1)
		b, err = d.reader.ReadByte()
		if err != nil {
			return Comment{}, err
		}
		if b != ' ' {
			return Comment{}, d.errSyntax("space")
		}
		d.advanceByte(1)
	case ',':
		c.Type = CommentTypeFlag
		d.advanceByte(1)
		b, err = d.reader.ReadByte()
		if err != nil {
			return Comment{}, err
		}
		if b != ' ' {
			return Comment{}, d.errSyntax("space")
		}
		d.advanceByte(1)
	case '|':
		// Previous is unsupported yet.
		line, _, err := d.reader.ReadLine()
		if err != nil {
			return Comment{}, err
		}
		d.advance(line)
	default:
		return Comment{}, d.errSyntax("space")
	}

	line, _, err := d.reader.ReadLine()
	if err != nil {
		return Comment{}, err
	}

	d.advance(line)
	c.Span = d.span(start)
	c.Value = string(line)
	return c, nil
}

func (d *Decoder) readComments(obsolete bool) (Comments, error) {
	start := d.pos
	var l Comments
	for {
		if obsolete {
			next, err := d.reader.Peek(4)
			if err != nil {
				return Comments{}, err
			}
			if string(next) != "#~ #" {
				// Not a comment on an obsolete message.
				return l, nil
			}

			if err := d.readPrefixObsolete(); err != nil {
				return Comments{}, err
			}
		}

		c, err := d.readComment()
		if err != nil {
			return l, err
		}
		if c.Type == 0 {
			break
		}
		l.Text = append(l.Text, c)
	}
	l.Span = d.span(start)
	return l, nil
}

func (d *Decoder) readPrefixObsolete() error {
	b, err := d.reader.ReadByte()
	if err != nil {
		return err
	}
	if b != '#' {
		return d.errSyntax("#")
	}
	d.advanceByte(1)

	b, err = d.reader.ReadByte()
	if err != nil {
		return err
	}
	if b != '~' {
		return d.errSyntax("~")
	}
	d.advanceByte(1)

	b, err = d.reader.ReadByte()
	if err != nil {
		return err
	}
	if b != ' ' {
		return d.errSyntax("space")
	}
	d.advanceByte(1)

	return nil
}

func (d *Decoder) parseHead(m Message, template bool) (h FileHead, err error) {
	if !m.Msgctxt.IsZero() {
		return FileHead{}, Error{Pos: m.Msgctxt.Position}
	}
	if !m.MsgidPlural.IsZero() {
		return FileHead{}, Error{Pos: m.MsgidPlural.Position}
	}
	if !m.Msgstr0.IsZero() {
		return FileHead{}, Error{Pos: m.Msgstr0.Position}
	}
	if !m.Msgstr1.IsZero() {
		return FileHead{}, Error{Pos: m.Msgstr1.Position}
	}
	if !m.Msgstr2.IsZero() {
		return FileHead{}, Error{Pos: m.Msgstr2.Position}
	}
	if !m.Msgstr3.IsZero() {
		return FileHead{}, Error{Pos: m.Msgstr3.Position}
	}
	if !m.Msgstr4.IsZero() {
		return FileHead{}, Error{Pos: m.Msgstr4.Position}
	}
	if !m.Msgstr5.IsZero() {
		return FileHead{}, Error{Pos: m.Msgstr5.Position}
	}
	if m.Msgid.Text.String() != "" {
		return FileHead{}, Error{Pos: m.Msgid.Position, Expected: "empty msgid"}
	}
	if len(m.Msgstr.Text.Lines) < 1 {
		return FileHead{}, Error{Pos: m.Msgid.Position, Expected: "msgstr with headers"}
	}

	// Join the msgstr lines, then split by "\n"
	// to get individual header key-value pairs.
	headers := strings.Split(m.Msgstr.Text.String(), "\n")

	pos := m.Msgstr.Position
	byName := make(map[string]struct{}, len(headers))
	for _, header := range headers {
		if header == "" {
			continue
		}
		name, value := splitHeader(header)
		if err := checkHeaderDuplicate(pos, byName, name); err != nil {
			return h, err
		}
		switch name {
		case "Project-Id-Version":
			h.ProjectIdVersion = value
		case "Report-Msgid-Bugs-To":
			h.ReportMsgidBugsTo = value
		case "POT-Creation-Date":
			h.POTCreationDate = value
		case "PO-Revision-Date":
			h.PORevisionDate = value
		case "Last-Translator":
			h.LastTranslator = value
		case "Language-Team":
			h.LanguageTeam = value
		case "Language":
			h.Language.Value = value
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
			h.MIMEVersion = value
			if h.MIMEVersion != "1.0" {
				return h, Error{
					Pos: pos,
					Err: ErrUnsupportedMIMEVersion,
				}
			}
		case "Content-Type":
			h.ContentType = value
			if _, _, err := mime.ParseMediaType(h.ContentType); err != nil {
				return h, Error{
					Pos: pos,
					Err: ErrMalformedHeaderContentType,
				}
			}
			if h.ContentType != "text/plain; charset=UTF-8" {
				return h, Error{
					Pos: pos,
					Err: ErrUnsupportedContentType,
				}
			}
		case "Content-Transfer-Encoding":
			h.ContentTransferEncoding = value
			switch h.ContentTransferEncoding {
			case "8bit":
				// OK
			default:
				return h, Error{
					Pos: pos,
					Err: ErrUnsupportedContentTransferEncoding,
				}
			}
		case "Plural-Forms":
			h.PluralForms = value
			// TODO: validate
		default:
			if strings.HasPrefix(name, "X-") {
				for _, nsh := range h.NonStandard {
					if nsh.Name == name {
						return h, Error{
							Pos: pos,
							Err: ErrDuplicateHeader,
						}
					}
				}
				h.NonStandard = append(h.NonStandard, XHeader{
					Name:  name,
					Value: value,
				})
				break
			}
			return h, Error{
				Pos: pos,
				Err: ErrUnsupportedHeader,
			}
		}
	}

	h.HeadComments = m.Msgid.Comments

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

func (d *Decoder) readMessage() (m Message, err error) {
	start := d.pos

	var previous statement
	next, err := d.reader.Peek(2)
	m.Obsolete = err == nil && string(next) == "#~"

LOOP:
	for {
		stmt, err := d.readStatement(m.Obsolete)
		if err != nil {
			if errors.Is(err, io.EOF) {
				switch previous.statementType {
				case 0:
					return m, Error{
						Pos:      d.pos,
						Expected: "msgctxt or msgid",
						Err:      io.ErrUnexpectedEOF,
					}
				case statementTypeMsgctxt:
					return m, Error{
						Pos:      d.pos,
						Expected: "msgid",
						Err:      io.ErrUnexpectedEOF,
					}
				case statementTypeMsgid:
					return m, Error{
						Pos:      d.pos,
						Expected: "msgid_plural or msgstr",
						Err:      io.ErrUnexpectedEOF,
					}
				case statementTypeMsgidPlural:
					return m, Error{
						Pos:      d.pos,
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
			return m, err
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
				return m, d.errSyntax("msgid")
			case statementTypeMsgid:
				return m, d.errSyntax("msgstr or msgid_plural")
			case statementTypeMsgidPlural:
				return m, d.errSyntax("msgstr[0]")
			case statementTypeMsgstr:
				return m, d.errSyntax("msgctxt or msgid")
			case statementTypeMsgstrIndexed:
				if stmt.pluralFormIndex <= previous.pluralFormIndex {
					return m, d.errSyntax(fmt.Sprintf("msgstr[%d]",
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
				return m, d.errSyntax("msgstr[0]")
			case statementTypeMsgstr:
				return m, d.errSyntax("msgctxt or msgid")
			case statementTypeMsgstrIndexed:
				if stmt.pluralFormIndex <= previous.pluralFormIndex {
					return m, d.errSyntax(fmt.Sprintf("msgstr[%d]",
						previous.pluralFormIndex+1))
				}
			}
		case statementTypeMsgidPlural:
			switch previous.statementType {
			case 0:
				return m, d.errSyntax("msgctxt or msgid")
			case statementTypeMsgctxt:
				return m, d.errSyntax("msgid")
			case statementTypeMsgid:
				m.MsgidPlural.Span = stmt.Span
				m.MsgidPlural.Comments = stmt.comments
				m.MsgidPlural.Text = stmt.text
			case statementTypeMsgidPlural:
				return m, d.errSyntax("msgstr[0]")
			case statementTypeMsgstr:
				return m, d.errSyntax("msgctxt or msgid")
			case statementTypeMsgstrIndexed:
				if stmt.pluralFormIndex <= previous.pluralFormIndex {
					return m, d.errSyntax(fmt.Sprintf("msgstr[%d]",
						previous.pluralFormIndex+1))
				}
			}
		case statementTypeMsgstr:
			switch previous.statementType {
			case 0:
				return m, d.errSyntax("msgctxt or msgid")
			case statementTypeMsgctxt:
				return m, d.errSyntax("msgid")
			case statementTypeMsgid:
				m.Msgstr.Span = stmt.Span
				m.Msgstr.Comments = stmt.comments
				m.Msgstr.Text = stmt.text
			case statementTypeMsgidPlural:
				return m, d.errSyntax("msgstr[0]")
			case statementTypeMsgstr:
				return m, d.errSyntax("msgctxt or msgid")
			case statementTypeMsgstrIndexed:
				return m, d.errSyntax("msgstr[n] or msgctxt or msgid")
			}
		case statementTypeMsgstrIndexed:
			switch previous.statementType {
			case 0:
				return m, d.errSyntax("msgctxt or msgid")
			case statementTypeMsgctxt:
				return m, d.errSyntax("msgid")
			case statementTypeMsgid:
				return m, d.errSyntax("msgid_plural or msgstr")
			case statementTypeMsgstr:
				return m, d.errSyntax("msgctxt or msgid")
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
					return m, d.errSyntax(fmt.Sprintf("msgstr[%d]",
						previous.pluralFormIndex+1))
				}
				msg.Span = stmt.Span
				msg.Comments = stmt.comments
				msg.Text = stmt.text
			}
		}
		previous = stmt
	}

	m.Span = d.span(start)
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
func (d *Decoder) readStatement(obsolete bool) (stmt statement, err error) {
	start := d.pos

	comments, err := d.readComments(obsolete)
	if err != nil {
		return statement{}, err
	}
	stmt.comments = comments

	if obsolete {
		b, err := d.peekByte()
		if err != nil {
			return statement{}, err
		}
		if b == '\n' {
			return statement{}, nil
		}
		if err := d.readPrefixObsolete(); err != nil {
			return statement{}, err
		}
	}

	longestPossiblePrefixLen := len(prefixMsgidPlural)
	next, err := d.reader.Peek(longestPossiblePrefixLen)
	if err != nil {
		if !errors.Is(err, io.EOF) {
			return statement{}, err
		}
		// Ignore EOF.
	}

	if pr := prefixMsgctxt; bytes.HasPrefix(next, pr) {
		stmt.statementType = statementTypeMsgctxt
		d.advanceByte(uint32(len(pr)))
		if _, err := d.reader.Read(pr); err != nil {
			return stmt, err
		}

	} else if pr := prefixMsgidPlural; bytes.HasPrefix(next, pr) {
		stmt.statementType = statementTypeMsgidPlural
		d.advanceByte(uint32(len(pr)))
		if _, err := d.reader.Read(pr); err != nil {
			return stmt, err
		}

	} else if pr := prefixMsgid; bytes.HasPrefix(next, pr) {
		stmt.statementType = statementTypeMsgid
		d.advanceByte(uint32(len(pr)))
		if _, err := d.reader.Read(pr); err != nil {
			return stmt, err
		}

	} else if pr := prefixMsgstrIndexed; bytes.HasPrefix(next, pr) {
		stmt.statementType = statementTypeMsgstrIndexed
		d.advanceByte(uint32(len(pr) - 1))
		if _, err := d.reader.Read(pr[:len(pr)-1]); err != nil {
			return stmt, err
		}

		index, err := d.readPluralIndex()
		if err != nil {
			return stmt, err
		}
		stmt.pluralFormIndex = index

	} else if pr := prefixMsgstr; bytes.HasPrefix(next, pr) {
		stmt.statementType = statementTypeMsgstr
		d.advanceByte(uint32(len(pr)))
		if _, err := d.reader.Read(pr); err != nil {
			return stmt, err
		}

	} else if bytes.HasPrefix(next, prefixLineBreak) {
		return statement{}, nil
	} else {
		return stmt, d.errSyntax("statement")
	}

	strStart := d.pos
	str, err := d.readStringLiteral()
	if err != nil {
		return statement{}, err
	}

	if str.Value != "" {
		stmt.text = StringLiterals{
			Span:  d.span(strStart),
			Lines: []StringLiteral{str},
		}
		stmt.Span = d.span(start)
		return stmt, nil
	}

	// Multi-line
	b, err := d.peekByte()
	if errors.Is(err, io.EOF) || err == nil && b != '"' {
		// Empty string
		stmt.text = StringLiterals{
			Span: d.span(strStart),
			Lines: []StringLiteral{{
				Span:  d.span(strStart),
				Value: "",
			}},
		}
		return stmt, nil
	}
	if err != nil {
		return statement{}, err
	}

	strings, err := d.readStringLiterals(obsolete)
	if err != nil {
		return stmt, err
	}

	stmt.text = strings
	stmt.Span = d.span(start)
	return stmt, nil
}

func (d *Decoder) readStringLiterals(obsolete bool) (strings StringLiterals, err error) {
	start := d.pos

	if obsolete {
		if err := d.readPrefixObsolete(); err != nil {
			return strings, err
		}
	}

	next, err := d.reader.Peek(1)
	if err != nil {
		return strings, err
	}
	if next[0] != '"' {
		return strings, d.errSyntax("string literal")
	}

	for {
		next, err := d.reader.Peek(1)
		if err != nil {
			return strings, err
		}
		if next[0] != '"' {
			break
		}
		str, err := d.readStringLiteral()
		if err != nil {
			return strings, err
		}
		strings.Lines = append(strings.Lines, str)

		if obsolete {
			next, err := d.reader.Peek(2)
			if err != nil {
				return strings, err
			}
			if string(next) != "#~" {
				// End of obsolete string literals.
				return strings, nil
			}

			if err := d.readPrefixObsolete(); err != nil {
				return strings, err
			}
		}
	}
	strings.Span = d.span(start)
	return strings, nil
}

func (d *Decoder) readStringLiteral() (StringLiteral, error) {
	start := d.pos
	line, _, err := d.reader.ReadLine()
	if err != nil {
		return StringLiteral{}, err
	}
	var s StringLiteral

	trimmed := strings.TrimSpace(string(line))

	if len(trimmed) < 2 || trimmed[0] != '"' || trimmed[len(trimmed)-1] != '"' {
		return StringLiteral{}, d.errSyntax("string literal")
	}

	unquoted, err := strconv.Unquote(trimmed)
	if err != nil {
		return StringLiteral{}, Error{
			Pos:      d.pos,
			Expected: "string literal",
			Err:      err,
		}
	}

	d.advanceByte(uint32(len(line)))
	d.advanceLine()
	s.Value = unquoted
	s.Span = d.span(start)
	return s, nil
}

func (d *Decoder) readPluralIndex() (index uint8, err error) {
	b, err := d.reader.ReadByte()
	if err != nil {
		return 0, err
	}
	if b != '[' {
		return 0, d.errSyntax("[")
	}
	d.advanceByte(1)

	b, err = d.reader.ReadByte()
	if err != nil {
		return 0, err
	}
	if b < '0' || b > '9' {
		return 0, d.errSyntax("index 0-5")
	}
	d.advanceByte(1)

	index = b - '0'

	b, err = d.reader.ReadByte()
	if err != nil {
		return 0, err
	}
	if b != ']' {
		return 0, d.errSyntax("]")
	}
	d.advanceByte(1)

	b, err = d.reader.ReadByte()
	if err != nil {
		return 0, err
	}
	if b != ' ' {
		return 0, d.errSyntax("space")
	}
	d.advanceByte(1)

	return index, nil
}

func (d *Decoder) peekByte() (byte, error) {
	b, err := d.reader.ReadByte()
	if err != nil {
		return 0, err
	}
	if err := d.reader.UnreadByte(); err != nil {
		panic(err) // Should never happen.
	}
	return b, nil
}

func checkHeaderDuplicate(p Position, byName map[string]struct{}, name string) error {
	if _, ok := byName[name]; ok {
		return Error{Pos: p, Err: ErrDuplicateHeader}
	}
	byName[name] = struct{}{}
	return nil
}

func splitHeader(s string) (name, value string) {
	i := strings.IndexByte(s, ':')
	if i == -1 {
		return "", ""
	}
	name = s[:i]
	value = strings.TrimSpace(s[i+1:])
	return name, value
}
