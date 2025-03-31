package main

import (
	"bytes"
	"cmp"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/romshark/localize/gettext"
	"github.com/romshark/localize/internal/cldr"
	"github.com/romshark/localize/internal/codeparser"
	"github.com/romshark/localize/internal/gengo"
	"golang.org/x/text/language"
	"mvdan.cc/gofumpt/format"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Println("ERR:", err)
		os.Exit(1)
	}
}

var (
	ErrSourceErrors    = errors.New("source code contains errors")
	ErrNoCommand       = errors.New("no command")
	ErrUnknownCommand  = errors.New("unknown command")
	ErrAnalyzingSource = errors.New("analyzing sources")
)

func run(osArgs []string) error {
	if len(osArgs) < 2 {
		return fmt.Errorf("%w, use either of: [generate,lint]", ErrNoCommand)
	}
	switch osArgs[1] {
	case "lint":
		// TODO: implement lint command
		panic("not yet implemented")
	case "generate":
		return runGenerate(osArgs)
	}
	return fmt.Errorf("%w %q, use either of: [generate,lint]",
		ErrUnknownCommand, osArgs[1])
}

func runGenerate(osArgs []string) error {
	start := time.Now()
	conf, err := parseCLIArgsGenerate(osArgs)
	if err != nil {
		return fmt.Errorf("parsing arguments: %w", err)
	}

	poEncoder := gettext.Encoder{}

	collection, bundle, stats, srcErrs, err := codeparser.Parse(
		conf.SrcPathPattern, conf.BundlePkgPath, conf.Locale,
		conf.TrimPath, conf.QuietMode, conf.VerboseMode,
	)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAnalyzingSource, err)
	}

	_ = bundle // TODO

	if len(srcErrs) > 0 {
		// Print source errors to console.
		fmt.Fprintf(os.Stderr, "SOURCE ERRORS (%d):\n", len(srcErrs))
		for _, e := range srcErrs {
			fmt.Fprintf(os.Stderr, " %s:%d:%d: %s\n",
				e.Filename, e.Line, e.Column, e.Err.Error())
		}
		return ErrSourceErrors
	}

	if err := os.MkdirAll(conf.BundlePkgPath, 0o755); err != nil {
		return fmt.Errorf("creating bundle package directory: %w", err)
	}

	headTxt, err := readOrCreateHeadTxt(conf)
	if err != nil {
		return err
	}

	po := collection.MakePO(headTxt)

	if err := writeSourceCatalog(conf, poEncoder, po); err != nil {
		return fmt.Errorf("writing native catalog: %w", err)
	}

	if err := writeTranslationTemplate(conf, poEncoder, po); err != nil {
		return fmt.Errorf("writing catalog.pot: %w", err)
	}

	if err := generateGoBundle(conf, headTxt, collection, bundle); err != nil {
		return fmt.Errorf("writing bundle_gen.go: %w", err)
	}

	if err := updateTranslationCatalogs(
		conf, bundle, collection, poEncoder,
	); err != nil {
		return fmt.Errorf("updating translation catalogs: %w", err)
	}

	timeTotal := time.Since(start)
	if !conf.QuietMode {
		w := os.Stderr
		_, _ = fmt.Fprintf(w, "Text/Block: %d/%d\n",
			stats.TextTotal.Load(), stats.BlockTotal.Load())
		_, _ = fmt.Fprintf(w, "Plural/PluralBlock: %d/%d\n",
			stats.PluralTotal.Load(), stats.PluralBlockTotal.Load())
		_, _ = fmt.Fprintf(w, "Calls merged: %d\n", stats.Merges.Load())
		_, _ = fmt.Fprintf(w, "files scanned: %d\n", stats.FilesTraversed.Load())
		_, _ = fmt.Fprintf(w, "time total: %s\n", timeTotal.String())
	}

	return nil
}

func catalogTemplateFileName(outPath string) string {
	return filepath.Join(outPath, "catalog.pot")
}

type ConfigGenerate struct {
	Locale                 language.Tag
	SrcPathPattern         string
	OutPathCatalogTemplate string
	TrimPath               bool
	QuietMode              bool
	VerboseMode            bool
	BundlePkgPath          string
}

// parseCLIArgsGenerate parses CLI arguments for command "generate"
func parseCLIArgsGenerate(osArgs []string) (*ConfigGenerate, error) {
	c := &ConfigGenerate{}

	var locale string

	cli := flag.NewFlagSet(osArgs[0], flag.ExitOnError)
	cli.StringVar(&locale, "l", "",
		"default locale of the original source code texts in BCP 47")
	cli.StringVar(&c.SrcPathPattern, "p", ".", "path to Go module")
	cli.StringVar(&c.OutPathCatalogTemplate, "tmpl", "",
		"catalog template output file path. Set to bundle package by default.")
	cli.BoolVar(&c.TrimPath, "trimpath", true, "enable source code path trimming")
	cli.BoolVar(&c.QuietMode, "q", false, "disable all console logging")
	cli.BoolVar(&c.VerboseMode, "v", false, "enables verbose console logging")
	cli.StringVar(&c.BundlePkgPath, "b", "localizebundle",
		"path to generated Go bundle package relative to module path (-p)")

	if err := cli.Parse(osArgs[2:]); err != nil {
		return nil, fmt.Errorf("parsing: %w", err)
	}

	if c.OutPathCatalogTemplate == "" {
		c.OutPathCatalogTemplate = catalogTemplateFileName(
			c.BundlePkgPath,
		)
	}

	if locale == "" {
		return nil, fmt.Errorf(
			"please provide a valid BCP 47 locale for " +
				"the default language of your original code base " +
				"using the 'l' parameter",
		)
	}
	var err error
	c.Locale, err = language.Parse(locale)
	if err != nil {
		return nil, fmt.Errorf(
			"argument 'l' (%q) must be a valid BCP 47 locale: %w", locale, err,
		)
	}

	return c, nil
}

func goBundleFileName(pkgPath string) string {
	return filepath.Join(pkgPath, filepath.Base(pkgPath)+"_gen.go")
}

func generateGoBundle(
	conf *ConfigGenerate, headTxt []string,
	collection *codeparser.Collection, bundle *codeparser.Bundle,
) error {
	f, err := os.OpenFile(
		goBundleFileName(conf.BundlePkgPath),
		os.O_CREATE|os.O_TRUNC|os.O_WRONLY,
		0o644,
	)
	if err != nil {
		return fmt.Errorf("opening Go bundle output file: %w", err)
	}
	var buf bytes.Buffer

	pkgName := filepath.Base(conf.BundlePkgPath)
	err = gengo.Write(&buf, conf.Locale, headTxt, pkgName, collection, bundle)
	if err != nil {
		return fmt.Errorf("generating Go bundle: %w", err)
	}

	// Format and write to file.
	formatted, err := format.Source(buf.Bytes(), format.Options{})
	if err != nil {
		return fmt.Errorf("formatting generated Go bundle code: %w", err)
	}

	if _, err := f.Write(formatted); err != nil {
		return fmt.Errorf("writing formatted Go bundle code to file: %w", err)
	}
	return nil
}

// readOrCreateHeadTxt reads the head.txt file if it exists, otherwise creates it.
func readOrCreateHeadTxt(conf *ConfigGenerate) ([]string, error) {
	headFilePath := filepath.Join(conf.BundlePkgPath, "head.txt")
	if fc, err := os.ReadFile(headFilePath); errors.Is(err, os.ErrNotExist) {
		if !conf.QuietMode {
			fmt.Fprintln(os.Stderr, "head.txt not found, creating a new one")
		}
		f, err := os.Create(headFilePath)
		if err != nil {
			return nil, fmt.Errorf("creating head.txt file: %w", err)
		}
		if err := f.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "closing head.txt file: %v", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("reading head.txt: %w", err)
	} else {
		return strings.Split(string(fc), "\n"), nil
	}
	return nil, nil
}

func writeSourceCatalog(
	conf *ConfigGenerate, poEncoder gettext.Encoder, po gettext.FilePO,
) error {
	{ // Write the source catalog `.po` file.
		fileName := filepath.Join(
			conf.BundlePkgPath,
			"source."+conf.Locale.String()+".po",
		)
		f, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			return fmt.Errorf("opening output file: %v", err)
		}
		// Add do not edit head comment.
		po.Head.HeadComments.Text = append(po.Head.HeadComments.Text,
			gettext.Comment{Value: "generated by " +
				"github.com/romshark/localize/cmd/localize. DO NOT EDIT."},
			gettext.Comment{Value: ""},
			gettext.Comment{Value: "Any changes made to this file will be overwritten"},
			gettext.Comment{Value: "as soon as localize is executed again."})
		if err := poEncoder.EncodePO(po, f); err != nil {
			return fmt.Errorf("encoding PO file: %w", err)
		}
	}
	return nil
}

func writeTranslationTemplate(
	conf *ConfigGenerate, poEncoder gettext.Encoder, po gettext.FilePO,
) error {
	f, err := os.OpenFile(
		conf.OutPathCatalogTemplate, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644,
	)
	if err != nil {
		return fmt.Errorf("opening file: %v", err)
	}
	pot := po.MakePOT()
	// Add do not edit head comment.
	pot.Head.HeadComments.Text = append(pot.Head.HeadComments.Text,
		gettext.Comment{Value: "generated by " +
			"github.com/romshark/localize/cmd/localize. DO NOT EDIT."},
		gettext.Comment{Value: ""},
		gettext.Comment{Value: "Any changes made to this file will be overwritten"},
		gettext.Comment{Value: "as soon as localize is executed again."})
	if err := poEncoder.EncodePOT(pot, f); err != nil {
		return fmt.Errorf("encoding POT file: %w", err)
	}
	return nil
}

func updateTranslationCatalogs(
	conf *ConfigGenerate,
	bundle *codeparser.Bundle, collection *codeparser.Collection,
	poEncoder gettext.Encoder,
) error {
	collMsgsByHash := make(map[string]codeparser.Msg, len(collection.Messages))
	for msg := range collection.Messages {
		collMsgsByHash[msg.Hash] = msg
	}

	for l, b := range bundle.Catalogs {
		locale := l.String()

		pluralForms, ok := cldr.ByTagOrBase(l)
		if !ok {
			return fmt.Errorf("couldn't find plural forms for locale: %s", locale)
		}

		inCatalog := map[string]*gettext.Message{}

		for i, m := range b.Messages.List {
			msgctxt := m.Msgctxt.Text.String()
			if _, ok := collMsgsByHash[msgctxt]; !ok {
				// Message not found in source code any more, make it obsolete.
				if b.Messages.List[i].Obsolete {
					// Already marked as obsolete.
					continue
				}

				if !conf.QuietMode && conf.VerboseMode {
					fmt.Fprintf(os.Stderr, "obsolete message %s in locale %s\n",
						msgctxt, locale)
				}

				m.Obsolete = true
				b.Messages.List[i] = m
			}
			inCatalog[msgctxt] = &b.Messages.List[i]
		}

		for m, meta := range collection.Messages {
			if catalogMsg, ok := inCatalog[m.Hash]; !ok {
				// New message to be added to the catalog.

				if !conf.QuietMode && conf.VerboseMode {
					fmt.Fprintf(os.Stderr, "add missing message %s in locale %s\n",
						m.Hash, locale)
				}

				nm := codeparser.MsgFromGettextMessage(pluralForms, m, meta)
				if len(nm.Msgstr.Text.Lines) > 0 {
					nm.Msgstr.Text = gettext.StringLiterals{
						Lines: []gettext.StringLiteral{{}},
					}
				}
				if len(nm.Msgstr0.Text.Lines) > 0 {
					nm.Msgstr0.Text = gettext.StringLiterals{
						Lines: []gettext.StringLiteral{{}},
					}
				}
				if len(nm.Msgstr1.Text.Lines) > 0 {
					nm.Msgstr1.Text = gettext.StringLiterals{
						Lines: []gettext.StringLiteral{{}},
					}
				}
				if len(nm.Msgstr2.Text.Lines) > 0 {
					nm.Msgstr2.Text = gettext.StringLiterals{
						Lines: []gettext.StringLiteral{{}},
					}
				}
				if len(nm.Msgstr3.Text.Lines) > 0 {
					nm.Msgstr3.Text = gettext.StringLiterals{
						Lines: []gettext.StringLiteral{{}},
					}
				}
				if len(nm.Msgstr4.Text.Lines) > 0 {
					nm.Msgstr4.Text = gettext.StringLiterals{
						Lines: []gettext.StringLiteral{{}},
					}
				}
				if len(nm.Msgstr5.Text.Lines) > 0 {
					nm.Msgstr5.Text = gettext.StringLiterals{
						Lines: []gettext.StringLiteral{{}},
					}
				}
				b.Messages.List = append(b.Messages.List, nm)
			} else {
				updateComments(catalogMsg, meta)
			}
		}

		if !conf.QuietMode {
			fmt.Fprintf(os.Stderr, "updating catalog %s\n", b.Path)
		}

		f, err := os.OpenFile(b.Path, os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return fmt.Errorf("opening catalog file: %w", err)
		}

		if err := poEncoder.EncodePO(b.FilePO, f); err != nil {
			return fmt.Errorf("encoding catalog file: %w", err)
		}
	}
	return nil
}

// updateComments syncs the code reference comments in dst with the position from m
// and returns true if any changes were made, otherwise returns false.
func updateComments(dst *gettext.Message, m codeparser.MsgMeta) {
	indexOfComment := func(formatted string) int {
		for i, com := range dst.Msgctxt.Comments.Text {
			if com.Type != gettext.CommentTypeReference {
				continue
			}
			if com.Value == formatted {
				return i
			}
		}
		return -1
	}
	indexOfPos := func(comment string) int {
		for i, pos := range m.Pos {
			formatted := gettext.FmtCodeRef(pos.Filename, pos.Line)
			if formatted == comment {
				return i
			}
		}
		return -1
	}

	for ci, com := range dst.Msgctxt.Comments.Text {
		if com.Type != gettext.CommentTypeReference {
			continue
		}
		i := indexOfPos(com.Value)
		if i == -1 {
			// Reference comment is obsolete, remove it.
			dst.Msgctxt.Comments.Text = slices.Delete(dst.Msgctxt.Comments.Text, ci, ci+1)
		}
	}
	for _, pos := range m.Pos {
		formatted := gettext.FmtCodeRef(pos.Filename, pos.Line)
		i := indexOfComment(formatted)
		if i == -1 {
			// New position, add new reference comment.
			dst.Msgctxt.Comments.Text = append(dst.Msgctxt.Comments.Text,
				gettext.Comment{
					Type:  gettext.CommentTypeReference,
					Value: formatted,
				})
		}
	}

	// Sort comments to enforce strict comment order by type.
	sortCommentsByType(dst)
}

func sortCommentsByType(m *gettext.Message) {
	cmp := func(a, b gettext.Comment) int { return cmp.Compare(a.Type, b.Type) }
	slices.SortFunc(m.Msgctxt.Comments.Text, cmp)
	slices.SortFunc(m.Msgid.Comments.Text, cmp)
	slices.SortFunc(m.MsgidPlural.Comments.Text, cmp)
	slices.SortFunc(m.Msgstr.Comments.Text, cmp)
	slices.SortFunc(m.Msgstr0.Comments.Text, cmp)
	slices.SortFunc(m.Msgstr1.Comments.Text, cmp)
	slices.SortFunc(m.Msgstr2.Comments.Text, cmp)
	slices.SortFunc(m.Msgstr3.Comments.Text, cmp)
	slices.SortFunc(m.Msgstr4.Comments.Text, cmp)
	slices.SortFunc(m.Msgstr5.Comments.Text, cmp)
}
