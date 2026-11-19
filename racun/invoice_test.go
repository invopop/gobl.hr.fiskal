package racun

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvoiceRequestXML(t *testing.T) {
	t.Run("writes the elements in schema order", func(t *testing.T) {
		out, err := xml.Marshal(testRequest())
		require.NoError(t, err)
		assert.Equal(t, `<RacunZahtjev xmlns="http://www.apis-it.hr/fin/2012/types/f73" Id="RacunZahtjev">`+
			`<Zaglavlje><IdPoruke>24d544c5-859a-47e2-85dc-0e1ce881b819</IdPoruke><DatumVrijeme>06.11.2026T13:58:10</DatumVrijeme></Zaglavlje>`+
			`<Racun><Oib>80432461483</Oib><USustPdv>true</USustPdv><DatVrijeme>06.11.2026T13:58:10</DatVrijeme><OznSlijed>P</OznSlijed>`+
			`<BrRac><BrOznRac>1</BrOznRac><OznPosPr>SHOP1</OznPosPr><OznNapUr>1</OznNapUr></BrRac>`+
			`<Pdv><Porez><Stopa>25.00</Stopa><Osnovica>10.00</Osnovica><Iznos>2.50</Iznos></Porez></Pdv>`+
			`<IznosUkupno>12.50</IznosUkupno><NacinPlac>G</NacinPlac><OibOper>80432461483</OibOper>`+
			`<ZastKod>8b55b3586e66d00b33266e44a774a408</ZastKod><NakDost>false</NakDost></Racun>`+
			`</RacunZahtjev>`, string(out))
	})

	t.Run("writes every optional element in schema order", func(t *testing.T) {
		req := fullRequest()

		out, err := xml.Marshal(req)
		require.NoError(t, err)
		assert.Contains(t, string(out), `<IznosUkupno>12.50</IznosUkupno>`)
		assertInOrder(t, string(out),
			"<Pdv>", "<Pnp>", "<OstaliPor><Porez><Naziv>", "<IznosOslobPdv>", "<IznosMarza>", "<IznosNePodlOpor>",
			"<Naknade><Naknada><NazivN>", "<IznosUkupno>", "<NacinPlac>", "<OibOper>", "<ZastKod>",
			"<NakDost>true", "<ParagonBrRac>", "<SpecNamj>", "<OibPrimateljaRacuna>",
		)

		back := new(InvoiceRequest)
		require.NoError(t, xml.Unmarshal(out, back))
		assert.Equal(t, req, back)
	})
}

// assertInOrder checks that each part appears in s after the one before.
func assertInOrder(t *testing.T, s string, parts ...string) {
	t.Helper()
	at := 0
	for _, p := range parts {
		i := strings.Index(s[at:], p)
		if !assert.GreaterOrEqual(t, i, 0, "%q missing or out of order", p) {
			return
		}
		at += i + len(p)
	}
}
