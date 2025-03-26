package codeparser

import (
	"fmt"

	"golang.org/x/text/language"
	"golang.org/x/tools/go/packages"
)

func ParseBundle(pkg *packages.Package) (*Bundle, error) {
	fmt.Println("PKG", pkg)
	return nil, nil
}

type Bundle struct {
	GeneratorVersion string
	Locales          []language.Tag
}
