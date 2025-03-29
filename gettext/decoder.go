package gettext

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/text/language"
)

func (d *Decoder) err(expected string) Error {
	return Error{Pos: d.pos, Expected: expected}
}

type Decoder struct {
	reader *bufio.Reader
	pos    Position

	// pending is the a pending directive that was successfuly read from
	// reader but wasn't actually consumed by a message reader yet.
	// If pending.directiveType != 0 it must be used in readMessage
	// before a new directive is read from reader because it was actually
	// determined as the start of a new message while reading a message.
	pending directive
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
	// Reset the decoder.
	d.reader.Reset(r)
	d.pos.Filename, d.pos.Index, d.pos.Line, d.pos.Column = fileName, 0, 1, 1
	d.pending.directiveType = 0

	// Start by reading the head message.
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

	// If a message is still pending then we encountered an unexpected EOF.
	switch d.pending.directiveType {
	case 0:
		// OK, no pending message.
	case directiveTypeMsgctxt:
		return nil, d.err("msgid")
	case directiveTypeMsgid:
		return nil, d.err("msgid_plural or msgstr")
	case directiveTypeMsgidPlural:
		return nil, d.err("msgstr[0]")
	case directiveTypeMsgstr:
		return nil, d.err("msgid or mstctxt")
	case directiveTypeMsgstrIndexed:
		if d.pending.pluralFormIndex < 5 {
			return nil, d.err(fmt.Sprintf("msgstr[%d]",
				d.pending.pluralFormIndex+1))
		}
		return nil, d.err("msgid or mstctxt")
	}

	return &f, nil
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

	var c Comment
	b, err = d.reader.ReadByte()
	if err != nil {
		return Comment{}, err
	}
	switch b {
	case '\n':
		// Empty comment
		c.Type = CommentTypeTranslator
		d.advanceByte(1)
		d.advanceLine()
		c.Span = d.span(start)
		return c, nil
	case ' ':
		c.Type = CommentTypeTranslator
		d.advanceByte(2)
	case '.':
		c.Type = CommentTypeExtracted
		d.advanceByte(2)
		b, err = d.reader.ReadByte()
		if err != nil {
			return Comment{}, err
		}
		if b != ' ' {
			return Comment{}, d.err("space")
		}
		d.advanceByte(1)
	case ':':
		c.Type = CommentTypeReference
		d.advanceByte(2)
		b, err = d.reader.ReadByte()
		if err != nil {
			return Comment{}, err
		}
		if b != ' ' {
			return Comment{}, d.err("space")
		}
		d.advanceByte(1)
	case ',':
		c.Type = CommentTypeFlag
		d.advanceByte(2)
		b, err = d.reader.ReadByte()
		if err != nil {
			return Comment{}, err
		}
		if b != ' ' {
			return Comment{}, d.err("space")
		}
		d.advanceByte(1)
	case '|':
		// Previous is unsupported yet.
		d.advanceByte(1)
		line, _, err := d.reader.ReadLine()
		if err != nil {
			return Comment{}, err
		}
		d.advanceByte(uint32(len(line)))
		d.advanceLine()
	default:
		if err := d.reader.UnreadByte(); err != nil {
			panic(err) // Should never happen
		}
		return Comment{}, nil // Not a comment
	}

	line, _, err := d.reader.ReadLine()
	if err != nil {
		return Comment{}, err
	}

	d.advanceByte(uint32(len(line)))
	d.advanceLine()
	c.Span = d.span(start)
	c.Value = string(line)
	return c, nil
}

func (d *Decoder) readComments(obsolete bool) (Comments, error) {
	start := d.pos
	var l Comments
	for {
		next, err := d.reader.Peek(4)
		if err != nil {
			return Comments{}, err
		}
		if obsolete {
			if string(next) != "#~ #" {
				// Not a comment on an obsolete message.
				return l, nil
			}
			if err := d.readPrefixObsolete(); err != nil {
				return Comments{}, err
			}
		} else if len(next) > 1 && string(next[:2]) == "#~" {
			return Comments{}, errEndOfMessage
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
		return d.err("#")
	}
	d.advanceByte(1)

	b, err = d.reader.ReadByte()
	if err != nil {
		return err
	}
	if b != '~' {
		return d.err("~")
	}
	d.advanceByte(1)

	b, err = d.reader.ReadByte()
	if err != nil {
		return err
	}
	if b != ' ' {
		return d.err("space")
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
				return h, Error{Pos: pos, Err: ErrUnsupportedContentTransferEncoding}
			}
		case "Plural-Forms":
			n, expr, err := parsePluralFormsHeader(value)
			if err != nil {
				return h, Error{Pos: pos, Err: err}
			}
			h.PluralForms = HeaderPluralForms{N: n, Expression: expr}
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
)

func (d *Decoder) readMessage() (m Message, err error) {
	var previousPluralFormIndex uint8
	var previous directiveType

	defer func() {
		if errors.Is(err, errEndOfMessage) {
			switch previous {
			case 0:
				err = d.err("msgctxt or msgid")
			case directiveTypeMsgctxt:
				err = d.err("msgid")
			case directiveTypeMsgid:
				err = d.err("msgid_plural or msgstr")
			case directiveTypeMsgidPlural:
				err = d.err("msgstr")
			case directiveTypeMsgstr:
				err = nil
			case directiveTypeMsgstrIndexed:
				// TODO: Check whether an index was still missing
				err = nil
			default:
				err = nil
			}
		} else if errors.Is(err, io.EOF) {
			switch previous {
			case 0:
				err = Error{
					Pos: d.pos, Expected: "msgctxt or msgid",
					Err: io.ErrUnexpectedEOF,
				}
			case directiveTypeMsgctxt:
				err = Error{
					Pos: d.pos, Expected: "msgid",
					Err: io.ErrUnexpectedEOF,
				}
			case directiveTypeMsgid:
				err = Error{
					Pos: d.pos, Expected: "msgid_plural or msgstr",
					Err: io.ErrUnexpectedEOF,
				}
			case directiveTypeMsgidPlural:
				err = Error{Pos: d.pos, Expected: "msgstr", Err: io.ErrUnexpectedEOF}
			case directiveTypeMsgstr:
				err = nil
			case directiveTypeMsgstrIndexed:
				// TODO: Check whether an index was still missing
				err = nil
			default:
				err = nil
			}
		}
	}()

	start := d.pos
	next, err := d.reader.Peek(2)
	m.Obsolete = err == nil && string(next) == "#~"

LOOP:
	for {
		if m.Obsolete {
			next, err := d.reader.Peek(2)
			if err != nil {
				return m, err
			}
			if string(next) != "#~" {
				// End of obsolete message
				return m, errEndOfMessage
			}
		}
		if err := d.readOptionalWhitespace(); err != nil {
			if !errors.Is(err, io.EOF) {
				return m, err
			}
			// Ignore EOF.
		}

		var dir directive
		if d.pending.directiveType == 0 {
			dir, err = d.readDirective(m.Obsolete)
			if err != nil {
				return m, err
			}
		} else {
			// Consume pending.
			dir = d.pending
			d.pending.directiveType = 0
		}

		switch dir.directiveType {
		case 0:
			break LOOP
		case directiveTypeMsgctxt:
			switch previous {
			case 0:
				// msgctxt is a the start of a message.
				m.Msgctxt.Span = dir.Span
				m.Msgctxt.Comments = dir.comments
				m.Msgctxt.Text = dir.text
			case directiveTypeMsgctxt:
				return m, d.err("msgid")
			case directiveTypeMsgid:
				return m, d.err("msgstr or msgid_plural")
			case directiveTypeMsgidPlural:
				return m, d.err("msgstr[0]")
			case directiveTypeMsgstr, directiveTypeMsgstrIndexed:
				// End of message is detected when
				// msgctxt follows msgstr or msgstr[index].
				d.pending = dir
				return m, nil
			}
		case directiveTypeMsgid:
			switch previous {
			case 0, directiveTypeMsgctxt:
				// msgid is either at the start of a message or follows msgctxt.
				m.Msgid.Span = dir.Span
				m.Msgid.Comments = dir.comments
				m.Msgid.Text = dir.text
			case directiveTypeMsgidPlural:
				return m, d.err("msgstr[0]")
			case directiveTypeMsgstr, directiveTypeMsgstrIndexed:
				// End of message is detected when
				// msgid follows msgstr or msgstr[index].
				d.pending = dir
				return m, nil
			}
		case directiveTypeMsgidPlural:
			switch previous {
			case directiveTypeMsgid:
				// msgid_plural always follows msgid.
				m.MsgidPlural.Span = dir.Span
				m.MsgidPlural.Comments = dir.comments
				m.MsgidPlural.Text = dir.text
			default:
				return m, d.err("msgid")
			}
		case directiveTypeMsgstr:
			switch previous {
			case directiveTypeMsgid:
				// msgstr always follows msgid.
				m.Msgstr.Span = dir.Span
				m.Msgstr.Comments = dir.comments
				m.Msgstr.Text = dir.text
			default:
				return m, d.err("msgid")
			}
		case directiveTypeMsgstrIndexed:
			switch previous {
			case 0,
				directiveTypeMsgctxt,
				directiveTypeMsgid,
				directiveTypeMsgstr:
				return m, d.err("msgid_plural")
			case directiveTypeMsgidPlural:
				// msgstr[index] follows msgid_plural.
				if dir.pluralFormIndex != 0 {
					return m, d.err("msgstr[0]")
				}
				m.Msgstr0.Span = dir.Span
				m.Msgstr0.Comments = dir.comments
				m.Msgstr0.Text = dir.text
			case directiveTypeMsgstrIndexed:
				// msgstr[index] follows msgstr[index]
				var msg *Msgstr
				switch dir.pluralFormIndex {
				case 0:
					return m, d.err("msgid_plural")
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
						dir.pluralFormIndex)) // Should never happen.
				}
				if err = d.checkMsgstrIndexedAgainstPrevious(
					dir.pluralFormIndex, previousPluralFormIndex); err != nil {
					return m, err
				}
				msg.Span = dir.Span
				msg.Comments = dir.comments
				msg.Text = dir.text
			}
		}

		previousPluralFormIndex = dir.pluralFormIndex
		previous = dir.directiveType
	}

	m.Span = d.span(start)
	return m, nil
}

type directiveType uint8

const (
	_ directiveType = iota

	directiveTypeMsgctxt       // msgctxt
	directiveTypeMsgid         // msgid
	directiveTypeMsgidPlural   // msgid_plural
	directiveTypeMsgstr        // msgstr
	directiveTypeMsgstrIndexed // msgstr[%d]
)

type directive struct {
	Span
	comments        Comments
	text            StringLiterals
	directiveType   directiveType
	pluralFormIndex uint8
}

var errEndOfMessage = errors.New("end of message")

// readDirective parses either msgctxt, msgid, msgid_plural and msgstr[%d]
func (d *Decoder) readDirective(obsolete bool) (dir directive, err error) {
	start := d.pos

	comments, err := d.readComments(obsolete)
	if err != nil {
		return directive{}, err
	}
	dir.comments = comments

	if obsolete {
		b, err := d.peekByte()
		if err != nil {
			return directive{}, err
		}
		if b != '#' {
			return directive{}, errEndOfMessage
		}
		if err := d.readPrefixObsolete(); err != nil {
			return directive{}, err
		}
	}

	longestPossiblePrefixLen := len(prefixMsgidPlural)
	next, err := d.reader.Peek(longestPossiblePrefixLen)
	if err != nil {
		if !errors.Is(err, io.EOF) {
			return directive{}, err
		}
		// Ignore EOF.
	}

	if pr := prefixMsgctxt; bytes.HasPrefix(next, pr) {
		dir.directiveType = directiveTypeMsgctxt
		d.advanceByte(uint32(len(pr)))
		if _, err := d.reader.Read(pr); err != nil {
			return dir, err
		}

	} else if pr := prefixMsgidPlural; bytes.HasPrefix(next, pr) {
		dir.directiveType = directiveTypeMsgidPlural
		d.advanceByte(uint32(len(pr)))
		if _, err := d.reader.Read(pr); err != nil {
			return dir, err
		}

	} else if pr := prefixMsgid; bytes.HasPrefix(next, pr) {
		dir.directiveType = directiveTypeMsgid
		d.advanceByte(uint32(len(pr)))
		if _, err := d.reader.Read(pr); err != nil {
			return dir, err
		}

	} else if pr := prefixMsgstrIndexed; bytes.HasPrefix(next, pr) {
		dir.directiveType = directiveTypeMsgstrIndexed
		d.advanceByte(uint32(len(pr) - 1))
		if _, err := d.reader.Read(pr[:len(pr)-1]); err != nil {
			return dir, err
		}

		index, err := d.readPluralIndex()
		if err != nil {
			return dir, err
		}
		dir.pluralFormIndex = index

	} else if pr := prefixMsgstr; bytes.HasPrefix(next, pr) {
		dir.directiveType = directiveTypeMsgstr
		d.advanceByte(uint32(len(pr)))
		if _, err := d.reader.Read(pr); err != nil {
			return dir, err
		}

	} else {
		return directive{}, errEndOfMessage
	}

	strStart := d.pos
	str, err := d.readStringLiteral()
	if err != nil {
		return directive{}, err
	}

	if str.Value != "" {
		dir.text = StringLiterals{
			Span:  d.span(strStart),
			Lines: []StringLiteral{str},
		}
		dir.Span = d.span(start)
		return dir, nil
	}

	// Multi-line
	b, err := d.peekByte()
	if errors.Is(err, io.EOF) || err == nil && b != '"' {
		// Empty string
		dir.text = StringLiterals{
			Span: d.span(strStart),
			Lines: []StringLiteral{{
				Span:  d.span(strStart),
				Value: "",
			}},
		}
		return dir, nil
	}
	if err != nil {
		return directive{}, err
	}

	strings, err := d.readStringLiterals(obsolete)
	if err != nil {
		return dir, err
	}

	dir.text = strings
	dir.Span = d.span(start)
	return dir, nil
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
		return strings, d.err("string literal")
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
		return StringLiteral{}, d.err("string literal")
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
		return 0, d.err("[")
	}
	d.advanceByte(1)

	b, err = d.reader.ReadByte()
	if err != nil {
		return 0, err
	}
	if b < '0' || b > '9' {
		return 0, d.err("index 0-5")
	}
	d.advanceByte(1)

	index = b - '0'

	b, err = d.reader.ReadByte()
	if err != nil {
		return 0, err
	}
	if b != ']' {
		return 0, d.err("]")
	}
	d.advanceByte(1)

	b, err = d.reader.ReadByte()
	if err != nil {
		return 0, err
	}
	if b != ' ' {
		return 0, d.err("space")
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

func (d *Decoder) checkMsgstrIndexedAgainstPrevious(
	currentIndex, previousIndex uint8,
) error {
	if currentIndex != previousIndex+1 {
		switch previousIndex {
		case 0:
			return d.err("msgstr[1]")
		case 1:
			return d.err("msgstr[2]")
		case 2:
			return d.err("msgstr[3]")
		case 3:
			return d.err("msgstr[4]")
		case 4:
			return d.err("msgstr[5]")
		case 5:
			return d.err("msgctxt or msgid")
		}
	}
	return nil
}

var regexpPluralFormsHeaderVal = regexp.MustCompile(
	`nplurals\s*=\s*(\d+)\s*;\s*plural\s*=\s*([^;]+)\s*;`,
)

func parsePluralFormsHeader(s string) (n uint8, expression string, err error) {
	matches := regexpPluralFormsHeaderVal.FindStringSubmatch(s)
	if len(matches) != 3 {
		return 0, "", ErrMalformedHeaderPluralForms
	}
	np, err := strconv.ParseUint(matches[1], 10, 8)
	if err != nil {
		return 0, "", fmt.Errorf("%w: %w", ErrMalformedHeaderPluralForms, err)
	}
	return uint8(np), matches[2], nil
}
