package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/romshark/localize/internal/codeparser"
	"github.com/romshark/localize/internal/gengo"
	"github.com/romshark/localize/internal/writepo"
	"golang.org/x/text/language"
	"mvdan.cc/gofumpt/format"
)

var (
	OutputFormatPO         = "po"
	OutputFormatPOTemplate = "pot"
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
	conf, err := parseCLIArgsGenerate(osArgs)
	if err != nil {
		return fmt.Errorf("parsing arguments: %w", err)
	}

	start := time.Now()

	catalog, stats, srcErrs, err := codeparser.Parse(
		conf.SrcPathPattern, conf.Locale,
		conf.TrimPath, conf.QuietMode, conf.VerboseMode,
	)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAnalyzingSource, err)
	}

	if len(srcErrs) > 0 {
		// Print source errors to console.
		fmt.Fprintf(os.Stderr, "SOURCE ERRORS (%d):\n", len(srcErrs))
		for _, e := range srcErrs {
			fmt.Fprintf(os.Stderr, " %s:%d:%d: %s\n", e.Filename, e.Line, e.Column, e.Err.Error())
		}
		return ErrSourceErrors
	}

	if err := os.MkdirAll(conf.BundlePkgPath, 0o755); err != nil {
		return fmt.Errorf("creating bundle package directory: %w", err)
	}

	{ // Write the native source catalog file.
		fileName := catalogFileName(conf.OutDirCatalog, conf.Locale, conf)
		f, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			return fmt.Errorf("opening output file: %v", err)
		}
		writepo.WriteCatalog(f, conf.Locale, catalog, false)
	}

	// Write translation template file.
	{
		f, err := os.OpenFile(
			conf.OutPathCatalogTemplate, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644,
		)
		if err != nil {
			return fmt.Errorf("opening file: %v", err)
		}
		writepo.WriteCatalog(f, conf.Locale, catalog, true)
	}

	{ // Generate Go code
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
		err = gengo.Write(
			&buf, conf.Locale, catalog.CopyrightNotice, pkgName, catalog,
		)
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

func catalogFileName(pkgDir string, locale language.Tag, conf *ConfigGenerate) string {
	fileName := "catalog." + locale.String() + "." + conf.OutputFormat
	return filepath.Join(pkgDir, fileName)
}

func catalogTemplateFileName(outPath, outFormat string) string {
	extension := outFormat
	if extension == OutputFormatPO {
		extension = OutputFormatPOTemplate
	}
	return filepath.Join(outPath, "catalog."+extension)
}

type ConfigGenerate struct {
	Locale                 language.Tag
	LocalesForTranslation  []language.Tag
	SrcPathPattern         string
	OutDirCatalog          string
	OutPathCatalogTemplate string
	OutputFormat           string
	TrimPath               bool
	QuietMode              bool
	VerboseMode            bool
	BundlePkgPath          string
}

// parseCLIArgsGenerate parses CLI arguments for command "generate"
func parseCLIArgsGenerate(osArgs []string) (*ConfigGenerate, error) {
	c := &ConfigGenerate{}

	var templatesForLangs flagArray
	var locale string

	cli := flag.NewFlagSet(osArgs[0], flag.ExitOnError)
	cli.StringVar(&locale, "l", "",
		"default locale of the original source code texts in BCP 47")
	cli.Var(&templatesForLangs, "t", "locale for translation (multiple allowed)")
	cli.StringVar(&c.SrcPathPattern, "p", ".", "path to Go module")
	cli.StringVar(&c.OutDirCatalog, "catdir", "",
		"catalog output directory. Set to bundle package by default.")
	cli.StringVar(&c.OutPathCatalogTemplate, "tmpl", "",
		"catalog template output file path. Set to bundle package by default.")
	cli.StringVar(&c.OutputFormat, "f", OutputFormatPO, "catalog output format")
	cli.BoolVar(&c.TrimPath, "trimpath", true, "enable source code path trimming")
	cli.BoolVar(&c.QuietMode, "q", false, "disable all console logging")
	cli.BoolVar(&c.VerboseMode, "v", false, "enables verbose console logging")
	cli.StringVar(&c.BundlePkgPath, "b", "localizebundle",
		"path to generated Go bundle package relative to module path (-p)")

	if err := cli.Parse(osArgs[2:]); err != nil {
		return nil, fmt.Errorf("parsing: %w", err)
	}

	if c.OutDirCatalog == "" {
		c.OutDirCatalog = c.BundlePkgPath
	}
	if c.OutPathCatalogTemplate == "" {
		c.OutPathCatalogTemplate = catalogTemplateFileName(
			c.BundlePkgPath, c.OutputFormat,
		)
	}

	switch c.OutputFormat {
	case OutputFormatPO:
	default:
		return nil, fmt.Errorf(
			"unsupported output format (available options: ["+
				OutputFormatPO+
				"]): %q",
			c.OutputFormat,
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

	// Sort and deduplicate.
	slices.Sort(templatesForLangs)
	templatesForLangs = slices.Compact(templatesForLangs)

	c.LocalesForTranslation = make([]language.Tag, len(templatesForLangs))
	for i, s := range templatesForLangs {
		locale, err := language.Parse(s)
		if err != nil {
			return nil, fmt.Errorf(
				"argument 't' at index %d (%q) must be a valid BCP 47 locale: %w",
				i, s, err,
			)
		}
		if locale == c.Locale {
			return nil, fmt.Errorf(
				"locale %q picked both for translation and as original source locale",
				locale,
			)
		}
		c.LocalesForTranslation[i] = locale
	}

	return c, nil
}

type flagArray []string

var _ flag.Value = &flagArray{}

func (i *flagArray) String() string {
	return fmt.Sprintf("%v", *i)
}

func (i *flagArray) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func goBundleFileName(pkgPath string) string {
	return filepath.Join(pkgPath, filepath.Base(pkgPath)+"_gen.go")
}
