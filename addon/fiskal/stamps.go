package fiskal

import (
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
)

// Stamps added to a fiscalized invoice. The ZKI and the QR code go on every
// printed invoice, even one CIS has not answered yet; the JIR is added when
// CIS accepts the invoice, and the QR code then refers to it.
//
// The keys name the system, as tbai-qr and verifactu-qr do, and as the
// eRačun stamps of gobl.hr.eracun (eracun-cis-*) do.
const (
	// StampZKI is the ZKI (zaštitni kod izdavatelja), the issuer's
	// protection code: 32 hexadecimal characters computed from the invoice
	// and signed with the issuer's certificate.
	StampZKI cbc.Key = "fiskal-zki"
	// StampJIR is the JIR (jedinstveni identifikator računa), the unique
	// invoice identifier CIS gives an accepted invoice: a UUID.
	StampJIR cbc.Key = "fiskal-jir"
	// StampQR is the URL the QR code on the printed invoice holds, for a
	// buyer to check the invoice with the Tax Administration. It has the JIR,
	// or the ZKI while the invoice has no JIR.
	StampQR cbc.Key = "fiskal-qr"
)

var stampUsage = &cbc.Usage{
	Keys: []cbc.Key{V1},
}

var stamps = []*cbc.Definition{
	{
		Key: StampZKI,
		Name: i18n.String{
			i18n.EN: "Croatia Fiskal: ZKI",
			i18n.HR: "Hrvatska fiskalizacija: ZKI",
		},
		Desc: i18n.String{
			i18n.EN: "Issuer's protection code (zaštitni kod izdavatelja): 32 hexadecimal characters computed from the invoice and signed with the issuer's certificate. It is printed on every invoice.",
		},
		Usage: stampUsage,
	},
	{
		Key: StampJIR,
		Name: i18n.String{
			i18n.EN: "Croatia Fiskal: JIR",
			i18n.HR: "Hrvatska fiskalizacija: JIR",
		},
		Desc: i18n.String{
			i18n.EN: "Unique invoice identifier (jedinstveni identifikator računa) that CIS gives an accepted invoice: a UUID.",
		},
		Usage: stampUsage,
	},
	{
		Key: StampQR,
		Name: i18n.String{
			i18n.EN: "Croatia Fiskal: QR Code",
			i18n.HR: "Hrvatska fiskalizacija: QR kod",
		},
		Desc: i18n.String{
			i18n.EN: "URL in the QR code printed on the invoice, for the buyer to check it with the Tax Administration. It has the JIR, or the ZKI while the invoice has no JIR.",
		},
		Usage: stampUsage,
	},
}
