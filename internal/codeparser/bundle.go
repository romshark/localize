package codeparser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/romshark/localize/gettext"
	"golang.org/x/text/language"
	"golang.org/x/tools/go/packages"
)

func ParseBundle(pkg *packages.Package) (*Bundle, error) {
	bundle := &Bundle{Translations: make(map[language.Tag]gettext.FilePO)}
	gettextDecoder := gettext.NewDecoder()

	err := findPOFiles(pkg.Dir, func(locale language.Tag, file string) error {
		f, err := os.OpenFile(file, os.O_RDONLY, 0o644)
		if err != nil {
			return fmt.Errorf("opening .po file: %w", err)
		}
		po, err := gettextDecoder.DecodePO(file, f)
		if err != nil {
			return fmt.Errorf("decoding .po file (%q): %w", file, err)
		}
		bundle.Translations[locale] = po
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discovering catalog .po files in bundle: %w", err)
	}

	// TODO: validate .po files

	return bundle, nil
}

type Bundle struct {
	Translations map[language.Tag]gettext.FilePO
}

func findPOFiles(dir string, fn func(locale language.Tag, file string) error) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		name := d.Name()
		if len(name) < len("catalog.en.po") ||
			!strings.HasPrefix(name, "catalog.") ||
			!strings.HasSuffix(name, ".po") {
			return nil
		}

		localeStr := name[len("catalog") : len(name)-len(".po")]
		loc, err := language.Parse(localeStr[1:])
		if err != nil {
			return nil
		}
		return fn(loc, path)
	})
}
