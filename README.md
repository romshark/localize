# localize

⚠️ This experiment has been suspended in favor of https://github.com/romshark/toki ⚠️
This repository now serves as a public archive only.

![Localize Banner](./localize_banner.svg)

localize helps you localize your Go programs by generating and synchronizing
[GNU gettext](https://www.gnu.org/software/gettext/) `.po`, `.pot` and generated
`.go` files.

- 🌐 Supports cardinal plural forms specified by
  [CLDR 47](https://cldr.unicode.org/downloads/cldr-47).
- 🛠️ Has [github.com/go-playground/locales](https://github.com/go-playground/locales)
  built-in.
- ⚡ Automatically generates highly efficient Go code from your translation bundles
  to translate your texts at runtime and improve application startup time since no
  costly runtime loading is necessary.
- 🔍 Automatically lints your `.po` and `.go` files to ensure correctness.

## General Workflow

1. Wrap plain texts you'd like to translate like `print("Hello")` in your code
   into one of the provided Reader methods such as `Text`:
   `print(localizedReader.Text("Hello")`.
2. Generate the localize bundle package using `localize generate`
   ([see example](#example-workflow)) containing GNU gettext `.po` translation files,
   the `.pot` template file and the `bundle_gen.go` Go bundle file.
3. Add your generated bundles to all `localize.New` constructor calls.
4. Translate the `.po` files.
5. Use the same `localize generate` command to update your `bundle_gen.go` and `.po`/
   `.pot` files linting them ✅ and keeping them in sync 🔄 when you add or remove texts.

## Example Workflow

1. Define the default texts in your code:

```go
package main

import (
	"github.com/romshark/localize"
	"golang.org/x/text/language"
	// Once you generated localizebundle you'll import it here
)

// ℹ️ This will automatically bring your bundle in shape when you run `go generate`.
//go:generate go run github.com/romshark/localize@latest generate -l en -b localizebundle

func main() {
	// Set English as your default source code's locale.
	localization, err := localize.New(language.English,
		/* once you generated the localized readers you'll add them here */)
	if err != nil {
		panic(err)
	}

	// Determine the user's preferred locale.
	userLocalePreference := language.English

	// Get the best matching localized reader for English.
	l := localization.Match(userLocalePreference)

	// ℹ️ All comments above localize method calls are included in the translation
	// and template files. This will give the translator and/or automated translation
	// software more context to provide higher quality translations.

	// Politely asking for the user's mood.
	fmt.Println(l.Text("How are you today?"))

	messagesUnread, messagesProcessing := 4, 10

	// ℹ️ when reading your code, localize will make sure you provided all plural forms
	// your source code's locale requires and report errors if you left something out.

	// Number of unread messages the user currently has.
	l.Plural(localize.Forms{
		One: "You have %d unread message",
		Other: "You have %d unread messages",
	}, messagesUnread)

	// ℹ️ Block and TextBlock methods allow you to format your texts in a more
	// readable way. They behave very similarly to GraphQL's block strings.

	// Number of messages that are currently being processed.
	l.PluralBlock(localize.Forms{
		One: `
			%d message is currently being processed.
			Please keep calm and continue internationalizing.
		`,
		Other: `
			%d messages are currently being processed.
			Please keep calm and continue internationalizing.
		`,
	}, messagesProcessing)
}
```

2. Generate a localize bundle to generate the `.po` file and as `.pot` files for translation:

```sh
go run github.com/romshark/localize/cmd/localize generate -l en -t de -t fr
```

This will generate:

- `locale.en.po` file containing the original English translation from the source code.
- `locale.de.pot` template file for German translations.
- `locale.fr.pot` template file for French translations.

3. Translate your

## Bundle File Structure

The generated bundle always contains the following files:

- `bundle_gen.go` is the generated Go code containing `localize.Reader` implementations
  for all languages defined by `.po` files in the bundle.
  - **Not editable** 🤖 Any manual change is always overwritten.
- `catalog.pot` is a gettext template file used to create `.po` translation files.
  - **Not editable** 🤖 Any manual change is always overwritten.
- `source.[locale].po` is a gettext translation file containing original source texts.
  - **Not editable** 🤖 Any manual change is always overwritten.
- `catalog.[locale].po` are gettext translation files
  for the locale specified in `[locale]`.
  - **Editable 📝**
    - Changed translations are preserved.
    - If a new text isn't found in the translation file it's automatically added.
    - If a text is no longer used in the source
      it's marked obsolete in the translation file.
    - Obsolete messages must be cleaned up manually.
    - Texts are reordered if necessary to preserve the right sorting order.
- `head.txt` is a text file defining the head comment to use in generated files.
  If this file isn't found a blank new one is generated.
  - **Editable 📝** You're supposed to edit this file.

All other files in the bundle package are ignored.
