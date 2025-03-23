package fmtplaceholder

import (
	"regexp"
	"strings"
)

var regexpGoFmtPlaceholders = regexp.MustCompile(
	`%[#0\-+\s]*\d*(?:\.\d*)?[bcdeEfFgGopqstTvxXUO%]`,
)

// Extract returns all Go fmt placeholder like %s, %d, %v, %q, etc. from s.
func Extract(s string) []string {
	return regexpGoFmtPlaceholders.FindAllString(s, -1)
}

var numericPlaceholders = "vfgxeFGXEbcdoOqU"

// Numeric returns true if placeholder can format numeric values (floats, ints, etc.).
// Warning: s is not validated! Expect false positives for invalid placeholders.
func Numeric(s string) bool {
	if s == "" || s == "%#v" {
		return false
	}
	return strings.IndexByte(numericPlaceholders, s[len(s)-1]) != -1
}
