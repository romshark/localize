package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtract(t *testing.T) {
	s := testSetup(t)
	_ = s

	outDir := t.TempDir()
	bundleDir := filepath.Join(outDir, "localizebundle")

	err := run([]string{"extract", "generate", "-b", bundleDir, "-l", "en"})
	require.NoError(t, err)
}

func testSetup(t *testing.T) string {
	return CreateSetup(t, map[string]string{
		// go.mod
		`go.mod`: `module example

go 1.24.1

require (
	github.com/romshark/localize v0.0.0-00010101000000-000000000000
	golang.org/x/text v0.23.0
)
`,
		// go.sum
		`go.sum`: `github.com/davecgh/go-spew v1.1.1 h1:vj9j/u1bqnvCEfJOwUhtlOARqs3+rkHYY13jYWTU97c=
github.com/davecgh/go-spew v1.1.1/go.mod h1:J7Y8YcW2NihsgmVo/mv3lAwl/skON4iLHjSsI+c5H38=
github.com/pmezard/go-difflib v1.0.0 h1:4DBwDE0NGyQoBHbLQYPwSUPoCMWR5BEzIk/f1lZbAQM=
github.com/pmezard/go-difflib v1.0.0/go.mod h1:iKH77koFhYxTK1pcRnkKkqfTogsbg7gZNVY4sRDYZ/4=
github.com/stretchr/testify v1.10.0 h1:Xv5erBjTwe/5IxqUQTdXv5kgmIvbHo3QQyRwhJsOfJA=
github.com/stretchr/testify v1.10.0/go.mod h1:r2ic/lqez/lEtzL7wO/rwa5dbSLXVDPFyf8C91i36aY=
golang.org/x/text v0.23.0 h1:D71I7dUrlY+VX0gQShAThNGHFxZ13dGLBHQLVl1mJlY=
golang.org/x/text v0.23.0/go.mod h1:/BLNzu4aZCJ1+kcD0DNRotWKage4q2rGVAg4o22unh4=
gopkg.in/yaml.v3 v3.0.1 h1:fxVm/GzAzEWqLHuvctI91KS9hhNmmWOoWu0XTYJS7CA=
gopkg.in/yaml.v3 v3.0.1/go.mod h1:K4uyk7z7BCEPqu6E+C64Yfv1cQ7kz7rIZviUmN+EgEM=
`,
		// main.go
		`main.go`: `package main

import (
	"fmt"

	"example/subpack"

	"github.com/romshark/localize"

	"golang.org/x/text/language"
)

func main() {
	langEnglish := language.English
	baseEnglish, _ := langEnglish.Base()
	localization, err := localize.New(baseEnglish)
	if err != nil {
		panic(err)
	}

	l := localization.ForBase(baseEnglish)
	l.Text("Main message 1")

	// description: This message is reused in multiple places.
	l.Text("Repeating message")

	fmt.Println(
		// description: This is the second static text.
		l.Text("Main message 2"),
	)

	// description: This is a plural translation in cardinal form.
	l.Cardinal("You achieved %d rank", 0)

	// description: This is a plural translation in ordinal form.
	// one: "You have %d unread email"
	l.Ordinal("You have %d unread emails", 1)
	subpack.Foo(localization)
}
`,
		// subpack/subpack.go
		`subpack/subpack.go`: `package subpack

import (
	"fmt"

	"github.com/romshark/localize"
	"golang.org/x/text/language"
)

func Foo(localization *localize.Bundle) {
	baseGerman, _ := language.German.Base()
	l := localization.ForBase(baseGerman)

	l.Text("Message from subpack package") // description: First subpack localization
	fmt.Println(l.Cardinal("Pluralized message from subpack package", 0))

	// description: This message is reused in multiple places.
	l.Text("Repeating message")
}
`,
	})
}

func CreateSetup(t *testing.T, fileMap map[string]string) string {
	t.Helper()
	root := t.TempDir()

	for path, content := range fileMap {
		fullPath := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("failed to create directories for %s: %v", fullPath, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write file %s: %v", fullPath, err)
		}
	}

	return root
}
