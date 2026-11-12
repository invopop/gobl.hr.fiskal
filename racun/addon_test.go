package racun

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/invopop/gobl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Invopop numbers an invoice before it is fiscalized, so an invoice the
// addon accepts must always convert: a numbered invoice that is never
// issued leaves a gap in the sequence (Art. 9(2) of the Fiscalization Act).
// The conversion does not check the invoice again; the addon rules refuse,
// before the invoice is numbered, what it can't convert.

// TestAddonExamplesConvert converts every example of the addon in the
// module's examples, as calculated and validated envelopes.
func TestAddonExamplesConvert(t *testing.T) {
	names, err := filepath.Glob("../examples/out/invoice-hr-hr-fiskal-*.json")
	require.NoError(t, err)
	require.NotEmpty(t, names)
	for _, name := range names {
		t.Run(filepath.Base(name), func(t *testing.T) {
			data, err := os.ReadFile(name)
			require.NoError(t, err)
			env := new(gobl.Envelope)
			require.NoError(t, json.Unmarshal(data, env))
			require.NoError(t, env.Validate())
			_, err = NewInvoice(env, testOptions(), testKey(t))
			assert.NoError(t, err)
		})
	}
}
