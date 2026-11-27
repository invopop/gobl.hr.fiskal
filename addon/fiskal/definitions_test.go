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

// TestIdentityDefinitions checks that the addon declares the supplier
// identities it reads, with the pattern its rules check, and that their keys
// are no longer extensions.
func TestIdentityDefinitions(t *testing.T) {
	ad := tax.AddonForKey(fiskal.V1)
	require.NotNil(t, ad)
	patterns := map[cbc.Key]string{}
	for _, def := range ad.Identities {
		patterns[def.Key] = def.Pattern
	}
	assert.Equal(t, map[cbc.Key]string{
		fiskal.IdentityKeyPremises: `^[0-9a-zA-Z]{1,20}$`,
		fiskal.IdentityKeyDevice:   `^[1-9][0-9]{0,19}$`,
	}, patterns)
	for key := range patterns {
		assert.Nil(t, tax.ExtensionForKey(key), "%s is an extension too", key)
	}
}
