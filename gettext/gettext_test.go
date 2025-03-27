package gettext_test

import (
	"bytes"
	_ "embed"
	"os"
	"testing"

	"github.com/romshark/localize/gettext"

	"github.com/stretchr/testify/require"
)

func TestDecodeEncode(t *testing.T) {
	for _, files := range [...]struct {
		PO  string
		POT string
	}{
		{
			PO:  "testdata/minimal.en.po",
			POT: "testdata/minimal.pot",
		},
		{
			PO:  "testdata/small.en.po",
			POT: "testdata/small.pot",
		},
		{
			PO:  "testdata/valid.en.po",
			POT: "testdata/valid.pot",
		},
		{
			PO:  "testdata/utf8.uk.po",
			POT: "testdata/utf8.pot",
		},
		{
			PO:  "testdata/deprecated.po",
			POT: "testdata/deprecated.pot",
		},
	} {
		t.Run(files.PO, func(t *testing.T) {
			// Decode `.po` from original.
			fdPO, err := os.OpenFile(files.PO, os.O_RDONLY, 0o644)
			require.NoError(t, err)
			defer func() { _ = fdPO.Close() }()
			dec := gettext.NewDecoder()
			po, err := dec.DecodePO(files.PO, fdPO)
			require.NoError(t, err)

			// Encode `.po` model to file.
			var bufPO bytes.Buffer
			enc := gettext.Encoder{}
			err = enc.EncodePO(po, &bufPO)
			require.NoError(t, err)

			// Compare encoded `.po` to original.
			originalPO, err := os.ReadFile(files.PO)
			require.NoError(t, err)
			require.Equal(t, string(originalPO), bufPO.String())

			// Generate `.pot` file from the `.po` file.
			var bufPOT bytes.Buffer
			pot := po.MakePOT() // Turn `.po` into `.pot`.
			err = enc.EncodePOT(pot, &bufPOT)
			require.NoError(t, err)

			// Compare encoded `.pot` to original.
			originalPOT, err := os.ReadFile(files.POT)
			require.NoError(t, err)
			require.Equal(t, string(originalPOT), bufPOT.String())

			// Decode `.pot` and compare to original.
			fdPOT, err := os.OpenFile(files.POT, os.O_RDONLY, 0o644)
			require.NoError(t, err)
			defer func() { _ = fdPOT.Close() }()
			decodedPOT, err := dec.DecodePOT(files.POT, fdPOT)
			require.NoError(t, err)

			// Re-encode `.pot` and compare again to original.
			var bufReencodedPOT bytes.Buffer
			err = enc.EncodePOT(decodedPOT, &bufReencodedPOT)
			require.NoError(t, err)
			require.Equal(t, string(originalPOT), bufPOT.String())
		})
	}
}
