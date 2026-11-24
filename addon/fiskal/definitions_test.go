package fiskal_test

import (
	"testing"

	"github.com/invopop/gobl.hr.fiskal/addon/fiskal"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/head"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStampDefinitions checks that the addon registers its stamps, so the
// GOBL docs list them with their names.
func TestStampDefinitions(t *testing.T) {
	for _, key := range []cbc.Key{fiskal.StampZKI, fiskal.StampJIR, fiskal.StampQR} {
		t.Run(key.String(), func(t *testing.T) {
			def := head.StampDefFor(key)
			require.NotNil(t, def)
			assert.Contains(t, def.Usage.Keys, fiskal.V1)
		})
	}
	assert.Len(t, head.FilterStampDefs([]cbc.Key{fiskal.V1}, nil), 3)
}

// TestExtensionUsage checks that each extension says where it goes, so the
// GOBL docs can show it.
func TestExtensionUsage(t *testing.T) {
	want := map[cbc.Key]string{
		fiskal.ExtKeyPremises:      bill.ShortSchemaTax,
		fiskal.ExtKeyDevice:        bill.ShortSchemaTax,
		fiskal.ExtKeySequenceScope: bill.ShortSchemaTax,
		fiskal.ExtKeyPaperNumber:   bill.ShortSchemaTax,
		fiskal.ExtKeyPP:            tax.ShortSchemaCombo,
		fiskal.ExtKeyVATStatus:     org.ShortSchemaParty,
	}
	for key, schema := range want {
		t.Run(key.String(), func(t *testing.T) {
			def := tax.ExtensionForKey(key)
			require.NotNil(t, def)
			require.NotNil(t, def.Usage)
			assert.Equal(t, []cbc.Key{fiskal.V1}, def.Usage.Keys)
			assert.Equal(t, []string{schema}, def.Usage.Schemas)
		})
	}
}
