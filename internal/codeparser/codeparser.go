package codeparser

import (
	"errors"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"hash"
	"iter"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/cespare/xxhash"
	"github.com/romshark/localize"
	"github.com/romshark/localize/internal/pluralform"
	"github.com/romshark/localize/internal/strfmt"
	"golang.org/x/text/language"
	"golang.org/x/tools/go/packages"
)

const (
	targetPackage = "github.com/romshark/localize"
	targetType    = targetPackage + ".Reader"

	FuncTypeText        = "Text"
	FuncTypeBlock       = "Block"
	FuncTypePlural      = "Plural"
	FuncTypePluralBlock = "PluralBlock"
	FuncTypeOrdinal     = "Ordinal"
)

type Statistics struct {
	TextTotal        atomic.Int64
	BlockTotal       atomic.Int64
	PluralTotal      atomic.Int64
	PluralBlockTotal atomic.Int64
	OrdinalTotal     atomic.Int64
	Merges           atomic.Int64
	FilesTraversed   atomic.Int64
}

type CatalogRevision struct {
	DateTime   time.Time // Optional
	Translator string    // Optional, format: "John Doe <john.doe@example.com>"
}

// Catalog is a collection of messages that can be marshaled into a .po gettext file.
type Catalog struct {
	CopyrightNotice string          // Optional
	BugsReportEmail string          // Optional
	LastRevision    CatalogRevision // Optional
	Locale          language.Tag
	Messages        map[Msg]MsgMeta
}

// Ordered returns an iterator over all messages ordered by hash.
func (c *Catalog) Ordered() iter.Seq2[Msg, MsgMeta] {
	ordered := make([]Msg, 0, len(c.Messages))
	for k := range maps.Keys(c.Messages) {
		ordered = append(ordered, k)
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Hash < ordered[j].Hash
	})
	return func(yield func(Msg, MsgMeta) bool) {
		for _, m := range ordered {
			if !yield(m, c.Messages[m]) {
				break
			}
		}
	}
}

type Msg struct {
	Hash        string
	Description string
	Zero        string
	One         string
	Two         string
	Few         string
	Many        string
	Other       string
	FuncType    string
}

type MsgMeta struct {
	Pos []token.Position
}

var (
	ErrSource          = errors.New("source code contains errors")
	ErrSourceTextEmpty = errors.New("text empty")
	ErrSourceArgType   = errors.New(
		"non-literal argument (only string literals and constants are supported)",
	)
	ErrMissingPluralForm     = errors.New("missing required plural form")
	ErrUnsupportedPluralForm = errors.New(
		"plural form not supported by source language",
	)
	ErrMissingQuantityPlaceholder = errors.New(
		"missing quantity placeholder \"%d\" in template",
	)
	ErrTooManyQuantityPlaceholders = errors.New(
		"plural template strings are expected to " +
			`have only one quantity placeholder "%d"`,
	)
	ErrWrongQuantityArgType = errors.New(
		"passing wrong type to quantity argument",
	)
)

type ErrorSrc struct {
	token.Position
	Err error
}

func Parse(pathPattern string, locale language.Tag, trimpath, quiet, verbose bool) (
	catalog *Catalog, stats *Statistics,
	srcErrs []ErrorSrc, err error,
) {
	fileset := token.NewFileSet()
	stats = new(Statistics)

	pluralForms := pluralform.ByTag(locale)

	cfg := &packages.Config{
		Mode: packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedDeps,
		Fset: fileset,
	}
	pkgs, err := packages.Load(cfg, pathPattern)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("loading packages: %w", err)
	}

	catalog = &Catalog{
		Messages: make(map[Msg]MsgMeta),
		Locale:   locale,
	}

	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			stats.FilesTraversed.Add(1)
			for _, decl := range file.Decls {
				ast.Inspect(decl, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}

					selector, ok := call.Fun.(*ast.SelectorExpr)
					if !ok { // Not a function selector (method call).
						return true
					}
					if len(call.Args) != 1 && len(call.Args) != 2 {
						return true
					}

					obj := pkg.TypesInfo.Uses[selector.Sel]
					if obj == nil { // Not the right package and type.
						return true
					}

					methodType, ok := obj.Type().(*types.Signature)
					if !ok {
						return true
					}

					recv := methodType.Recv()
					if recv == nil || recv.Type().String() != targetType {
						return true // Not the right receiver type.
					}

					if obj.Pkg() == nil || obj.Pkg().Path() != targetPackage {
						return true // Not from the target package.
					}

					funcType := selector.Sel.Name
					switch funcType {
					case FuncTypeText:
						stats.TextTotal.Add(1)
					case FuncTypeBlock:
						stats.BlockTotal.Add(1)
					case FuncTypePlural:
						stats.PluralTotal.Add(1)
					case FuncTypePluralBlock:
						stats.PluralBlockTotal.Add(1)
					case FuncTypeOrdinal:
						stats.OrdinalTotal.Add(1)
						return true // Doesn't require further processing.
					default:
						return true // Not the right methods.
					}

					pos := fileset.Position(call.Pos())
					if trimpath {
						pos.Filename = mustTrimPath(pathPattern, pos.Filename)
					}
					argType := pkg.TypesInfo.Types[call.Args[0]]

					msg := Msg{
						FuncType: funcType,
					}

					switch funcType {
					case FuncTypePlural, FuncTypePluralBlock:
						cl, ok := call.Args[0].(*ast.CompositeLit)
						if !ok {
							// Unsupported argument value type.
							appendSrcErr(&srcErrs, pos, fmt.Errorf(
								"%w: %s", ErrSourceArgType, typeKind(call.Args[0]),
							))
							return false
						}
						f := parseForms(fileset, cl, pkg.TypesInfo, &srcErrs)
						msg.Zero = mustFmtTemplate(funcType, f.Zero)
						msg.One = mustFmtTemplate(funcType, f.One)
						msg.Two = mustFmtTemplate(funcType, f.Two)
						msg.Few = mustFmtTemplate(funcType, f.Few)
						msg.Many = mustFmtTemplate(funcType, f.Many)
						msg.Other = mustFmtTemplate(funcType, f.Other)

						validateForms(&srcErrs, locale, pos, pluralForms, msg)

						validateQuantityArgument(
							&srcErrs, pos, call.Args[1], pkg.TypesInfo,
						)

					default:
						var textValue string
						switch k := call.Args[0].(type) {
						case *ast.Ident:
							v := argType.Value

							if v != nil && v.Kind() == constant.String {
								// Constants are supported.
								textValue = constant.StringVal(v)
							} else {
								// Unsupported argument value type.
								appendSrcErr(&srcErrs, pos, fmt.Errorf(
									"%w: %s", ErrSourceArgType, typeKind(call.Args[0]),
								))
								return true
							}
						case *ast.BasicLit:
							textValue = k.Value
						default:
							appendSrcErr(&srcErrs, pos, fmt.Errorf(
								"%w: %s", ErrSourceArgType, typeKind(call.Args[0]),
							))
							return true
						}
						msg.Other = mustFmtTemplate(funcType, textValue)
					}

					if verbose && !quiet {
						fmt.Fprintf(
							os.Stderr, "%s:%d:%d\n",
							pos.Filename, pos.Line, pos.Column,
						)
					}

					if msg.Other == "" {
						appendSrcErr(&srcErrs, pos, ErrSourceTextEmpty)
					}

					for _, group := range file.Comments {
						if group.Pos() < call.Pos() && group.End() < call.Pos() {
							commentLines := extractComments(group)
							msg.Description = strings.Join(commentLines, "\n")
						}
					}

					msg.Hash = messageHash(msg.Other, msg.Description)

					if m, ok := catalog.Messages[msg]; ok {
						// Identical message was already found in another place.
						// Merge messages into one.
						m.Pos = append(m.Pos, pos)
						catalog.Messages[msg] = m
						stats.Merges.Add(1)
					} else {
						// New message found.
						m.Pos = []token.Position{pos}
						catalog.Messages[msg] = m
					}

					return true
				})
			}
		}
	}

	return catalog, stats, srcErrs, nil
}

func extractComments(group *ast.CommentGroup) (lines []string) {
	for _, com := range group.List {
		s := strings.TrimSpace(com.Text)
		s = strings.TrimPrefix(s, "//")
		s = strings.TrimSpace(s)
		lines = append(lines, s)
	}
	return lines
}

func mustTrimPath(basePattern, s string) string {
	basePattern = strings.TrimSuffix(basePattern, "/...")
	abs, err := filepath.Abs(basePattern)
	if err != nil {
		panic(fmt.Errorf("getting absolute path: %w", err))
	}
	return strings.TrimPrefix(s, abs)
}

var ErrSyntax = errors.New("syntax error")

var hasherPool = sync.Pool{
	New: func() any { return xxhash.New() },
}

// messageHash computes a unique 64-bit XXHash for a message.
func messageHash(text, description string) string {
	h := hasherPool.Get().(hash.Hash64)
	defer hasherPool.Put(h)

	h.Reset()
	_, _ = h.Write(unsafeS2B(text))
	_, _ = h.Write(unsafeS2B(description))
	s := h.Sum64()
	return strconv.FormatUint(s, 16)
}

// unsafeS2B unsafely converts s to []byte.
//
// WARNING: The returned byte slice shares the underlying memory with s
// and therefore breaks Go's string immutability guarantee.
// Use for temporary conversions with utmost caution!
func unsafeS2B(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

func typeKind(e ast.Expr) string { return fmt.Sprintf("%T", e) }

func parseForms(
	fset *token.FileSet, cl *ast.CompositeLit, info *types.Info, srcErrs *[]ErrorSrc,
) (forms localize.Forms) {
	// TODO: report errors to srcErrs
	_ = srcErrs

	typ := info.Types[cl].Type
	named, ok := typ.(*types.Named)
	if !ok {
		return // Not a named type, return empty.
	}

	structType, ok := named.Underlying().(*types.Struct)
	if !ok {
		return // Not a struct, return empty.
	}

	// Field order fallback for positional fields
	fieldOrder := make([]string, structType.NumFields())
	for i := range structType.NumFields() {
		fieldOrder[i] = structType.Field(i).Name()
	}

	for i, elt := range cl.Elts {
		var fieldName string
		var valExpr ast.Expr

		switch v := elt.(type) {
		case *ast.KeyValueExpr:
			ident, ok := v.Key.(*ast.Ident)
			if !ok {
				continue
			}
			fieldName = ident.Name
			valExpr = v.Value
		default:
			// Positional field.
			if i >= len(fieldOrder) {
				continue
			}
			fieldName = fieldOrder[i]
			valExpr = v
		}

		val := info.Types[valExpr].Value
		if val == nil || val.Kind() != constant.String {
			continue // Not a constant string.
		}

		str := constant.StringVal(val)

		switch fieldName {
		case "Zero":
			forms.Zero = str
		case "One":
			forms.One = str
		case "Two":
			forms.Two = str
		case "Few":
			forms.Few = str
		case "Many":
			forms.Many = str
		case "Other":
			forms.Other = str
		}
	}

	if forms.Other == "" {
		appendSrcErr(srcErrs, fset.Position(cl.Pos()), ErrSourceTextEmpty)
	}

	return forms
}

func appendSrcErr(s *[]ErrorSrc, pos token.Position, err error) {
	*s = append(*s, ErrorSrc{Position: pos, Err: err})
}

func mustFmtTemplate(funcType string, templateText string) string {
	if templateText == "" {
		return ""
	}
	if templateText[0] == '`' || templateText[0] == '"' {
		var err error
		templateText, err = strconv.Unquote(templateText)
		if err != nil {
			panic(err)
		}
	}
	switch funcType {
	case FuncTypeBlock, FuncTypePluralBlock:
		return strfmt.Dedent(templateText)
	}
	return templateText
}

func validateForms(
	errs *[]ErrorSrc, locale language.Tag, pos token.Position,
	pluralForms pluralform.PluralForms, msg Msg,
) {
	// TODO returns the correct line:column for the particular line the error was
	// detected at since currently it's the pos of the call.
	if msg.Other == "" {
		appendSrcErr(errs, pos, fmt.Errorf(
			"%w: all languages require form Other",
			ErrMissingPluralForm,
		))
	}
	validatePluralTemplate(errs, pos, msg.Other)

	if pluralForms.Zero && msg.Zero == "" {
		appendSrcErr(errs, pos, fmt.Errorf(
			"%w: locale %q requires plural form Zero",
			ErrMissingPluralForm, locale.String(),
		))
	}
	if !pluralForms.Zero && msg.Zero != "" {
		appendSrcErr(errs, pos, fmt.Errorf(
			"%w: locale %q doesn't support plural form Zero",
			ErrUnsupportedPluralForm, locale.String(),
		))
	}
	if msg.Zero != "" {
		validatePluralTemplate(errs, pos, msg.Zero)
	}

	if pluralForms.One && msg.One == "" {
		appendSrcErr(errs, pos, fmt.Errorf(
			"%w: locale %q requires plural form One",
			ErrMissingPluralForm, locale.String(),
		))
	}
	if !pluralForms.One && msg.One != "" {
		appendSrcErr(errs, pos, fmt.Errorf(
			"%w: locale %q doesn't support plural form One",
			ErrUnsupportedPluralForm, locale.String(),
		))
	}
	if msg.One != "" {
		validatePluralTemplate(errs, pos, msg.One)
	}

	if pluralForms.Two && msg.Two == "" {
		appendSrcErr(errs, pos, fmt.Errorf(
			"%w: locale %q requires plural form Two",
			ErrMissingPluralForm, locale.String(),
		))
	}
	if !pluralForms.Two && msg.Two != "" {
		appendSrcErr(errs, pos, fmt.Errorf(
			"%w: locale %q doesn't support plural form Two",
			ErrUnsupportedPluralForm, locale.String(),
		))
	}
	if msg.Two != "" {
		validatePluralTemplate(errs, pos, msg.Two)
	}

	if pluralForms.Few && msg.Few == "" {
		appendSrcErr(errs, pos, fmt.Errorf(
			"%w: locale %q requires plural form Few",
			ErrMissingPluralForm, locale.String(),
		))
	}
	if !pluralForms.Few && msg.Few != "" {
		appendSrcErr(errs, pos, fmt.Errorf(
			"%w: locale %q doesn't support plural form Few",
			ErrUnsupportedPluralForm, locale.String(),
		))
	}
	if msg.Few != "" {
		validatePluralTemplate(errs, pos, msg.Few)
	}

	if pluralForms.Many && msg.Many == "" {
		appendSrcErr(errs, pos, fmt.Errorf(
			"%w: locale %q requires plural form Many",
			ErrMissingPluralForm, locale.String(),
		))
	}
	if !pluralForms.Many && msg.Many != "" {
		appendSrcErr(errs, pos, fmt.Errorf(
			"%w: locale %q doesn't support plural form Many",
			ErrUnsupportedPluralForm, locale.String(),
		))
	}
	if msg.Many != "" {
		validatePluralTemplate(errs, pos, msg.Many)
	}
}

func validatePluralTemplate(errs *[]ErrorSrc, pos token.Position, s string) {
	n := strings.Count(s, "%d")
	if n < 1 {
		appendSrcErr(errs, pos, fmt.Errorf(
			"%w", ErrMissingQuantityPlaceholder,
		))
	} else if n > 1 {
		appendSrcErr(errs, pos, fmt.Errorf(
			"%w: found %d", ErrTooManyQuantityPlaceholders, n,
		))
	}
}

func validateQuantityArgument(
	errs *[]ErrorSrc, pos token.Position, expr ast.Expr, info *types.Info,
) {
	tv := info.Types[expr]
	basic, ok := tv.Type.Underlying().(*types.Basic)
	if ok {
		infoBits := basic.Info()
		if infoBits&(types.IsInteger|types.IsUnsigned|types.IsFloat) != 0 {
			return
		}
	}
	appendSrcErr(errs, pos, fmt.Errorf(
		"%w: %T", ErrWrongQuantityArgType, tv.Type.String(),
	))
}
