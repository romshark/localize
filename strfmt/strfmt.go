// Package strfmt provides string formatting functions.
package strfmt

import "strings"

// Dedent removes leading/trailing blank lines and
// the common leading indentation from all non-empty lines.
func Dedent(s string) string {
	lines := strings.Split(s, "\n")
	for len(lines) > 0 && isLineBlank(lines[0]) {
		lines = lines[1:]
	}
	for len(lines) > 0 && isLineBlank(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}
	minInd := -1
	for _, line := range lines {
		if isLineBlank(line) {
			continue
		}
		if indent := leadingWhitespace(line); minInd == -1 || indent < minInd {
			minInd = indent
		}
	}
	// Dedent.
	for i, line := range lines {
		if isLineBlank(line) {
			continue
		}
		if len(line) >= minInd {
			lines[i] = line[minInd:]
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func isLineBlank(s string) bool { return strings.TrimSpace(s) == "" }

func leadingWhitespace(s string) (count int) {
	for _, r := range s {
		if r == ' ' || r == '\t' {
			count++
			continue
		}
		break
	}
	return count
}
