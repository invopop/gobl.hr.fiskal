# GOBL ➡️ Croatia Fiskal

Croatian Fiskalizacija 1.0 (B2C invoices) addon and CIS client for [GOBL](https://github.com/invopop/gobl).

Released under the Apache 2.0 [LICENSE](https://github.com/invopop/gobl.hr.fiskal/blob/main/LICENSE), Copyright 2026 [Invopop S.L.](https://invopop.com).

[![Lint](https://github.com/invopop/gobl.hr.fiskal/actions/workflows/lint.yaml/badge.svg)](https://github.com/invopop/gobl.hr.fiskal/actions/workflows/lint.yaml)
[![Test Go](https://github.com/invopop/gobl.hr.fiskal/actions/workflows/test.yaml/badge.svg)](https://github.com/invopop/gobl.hr.fiskal/actions/workflows/test.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/invopop/gobl.hr.fiskal)](https://goreportcard.com/report/github.com/invopop/gobl.hr.fiskal)
[![codecov](https://codecov.io/gh/invopop/gobl.hr.fiskal/graph/badge.svg)](https://codecov.io/gh/invopop/gobl.hr.fiskal)
[![GoDoc](https://godoc.org/github.com/invopop/gobl.hr.fiskal?status.svg)](https://godoc.org/github.com/invopop/gobl.hr.fiskal)
![Latest Tag](https://img.shields.io/github/v/tag/invopop/gobl.hr.fiskal)

This module fiscalizes Croatian invoices at the time of the sale, such as
those a shop issues at the till (Fiscalization 1.0, "fiskalizacija računa u krajnjoj
potrošnji", Zakon o fiskalizaciji, NN 89/25), with the Tax Administration's
invoice fiscalization service, CIS. Croatian law calls these documents
računi (invoices), the same word as in eRačun; it has no separate term for a
receipt. The module provides the GOBL addon `hr-fiskal-b2c-v1` in
`addon/fiskal/`, and the conversion and client in `racun/`.

The Croatian tax regime itself (`regimes/hr`) lives in GOBL core. The
Croatian eRačun, the e-invoice businesses exchange under Fiscalization 2.0,
lives in [gobl.hr.eracun](https://github.com/invopop/gobl.hr.eracun).

## Layout

- `addon/fiskal/` — the GOBL addon `hr-fiskal-b2c-v1` for invoices
  fiscalized at the time of the sale: the business premises and device
  labels, the numbering scope, the VAT system and the paper invoice number as
  extensions of the invoice tax, the operator as a supplier person (as in
  `hr-eracun-v1`), and the rules CIS needs met. It also defines the stamps of
  a fiscalized invoice: `fiskal-zki`, `fiskal-jir` and `fiskal-qr`.
- `racun/` — invoice fiscalization: builds the invoice request
  (`RacunZahtjev`) from a GOBL invoice with the `hr-fiskal-b2c-v1` addon,
  computes its ZKI (the issuer's protection code), signs it with XML-DSig,
  and sends it to CIS, checking CIS's signed answer for the JIR. `AddStamps`
  adds the ZKI, the JIR and the QR code URL to the envelope.
- `ca/` — the FINA CA certificates that CIS clients trust, embedded: the demo
  chain for CIS test and the production chain for production CIS. FINA's CAs
  are not in the system certificate store, and the CIS servers send their
  TLS certificate without its intermediate. `racun` trusts the chain of its
  environment and nothing else.
- the module root is reserved for tooling built on top of them.

## Usage

Add a blank import of the **addon** so it registers itself, then use GOBL as
normal:

```go
import (
	"github.com/invopop/gobl"
	_ "github.com/invopop/gobl.hr.fiskal/addon/fiskal"
)
```

Declare `hr-fiskal-b2c-v1` on an invoice and `Calculate` + `Validate` will
run the addon's normalization and rules. To fiscalize it, build the
invoice with `racun.NewInvoice`, wrap it with `racun.NewRequest`, send it
with `Client.Fiscalize`, and add the stamps with `racun.AddStamps`.

> **Note**: `hr-fiskal-b2c-v1` is not yet in GOBL core's approved
> external-addon registry. It is added there once this module is released.

## Extensions

The `hr-fiskal-b2c-v1` addon sets these on the invoice tax:

| Key | Description |
| --- | --- |
| `hr-fiskal-premises` | Business premises label registered in ePorezna (`OznPosPr`): up to 20 letters and digits. Required. |
| `hr-fiskal-device` | Label of the device that issued the invoice (`OznNapUr`): up to 20 digits with no leading zero. Required. |
| `hr-fiskal-sequence-scope` | What the invoice numbers count within (`OznSlijed`): `P` per business premises (the default) or `N` per device. |
| `hr-fiskal-paper-number` | Number of an invoice from the bound paper invoice book, entered after the till broke down (`ParagonBrRac`): up to 100 characters. Such an invoice is sent late. |

The supplier party carries one more:

| Key | Description |
| --- | --- |
| `hr-fiskal-vat-system` | Whether the supplier is in the VAT system (`USustPdv`): `Y` (the default) or `N`. A small taxpayer, exempt from VAT under Art. 90(1) of the VAT Act, is outside it: its lines use the VAT key `exempt`, and the addon adds a note citing the exemption, as Art. 79(3) requires. `racun` sends CIS only the total of such an invoice, with no VAT breakdown: CIS checks the breakdown only for an issuer in the VAT system. |

The invoice code is the bare sequential number (`BrOznRac`): digits with no
leading zero. A printed invoice shows the number as
`BrOznRac/OznPosPr/OznNapUr`, such as `1001/POS1/1`.

The number can come from two places:

- **A till that numbers its own invoices** sends the code with the labels.
- **Invopop numbers the invoice.** The invoice arrives without a code, and the
  sequence step of the workflow gives it one. That step keeps one counter for
  each supplier and series, so the addon sets the series from the labels: the
  premises label for scope `P` (`POS1`), or `<premises>/<device>` for scope `N`
  (`POS1/1`). CIS refuses leading zeros, so the addon removes them from a
  code made only of digits: the step's default padding of six digits gives
  `000042`, which becomes `42`. The step must not add a prefix or a suffix,
  and must count per series (not use one fixed sequence for every premises).

The rules refuse a series that does not match the labels, so that a counter
never numbers two premises or two devices.

## Issue date and time

The invoice must have its `issue_date` and `issue_time`, in Croatian time.
CIS receives both (`DatVrijeme`), and the issue time goes into the ZKI. The
addon does not fill in a missing time with the current time. A normalizer
runs on every calculation, so it would use the time when a draft was first
calculated, not the time when the invoice was issued.

## Operator

The operator is the person who issued the invoice: the person at the till,
or the supplier for a self-service device. CIS receives their OIB
(`OibOper`). Give the operator the `operator` key and an `OIB` identity, the
same convention as the `hr-eracun-v1` addon:

```yaml
supplier:
  people:
    - key: operator
      name:
        given: "Operater1"
      identities:
        - type: OIB
          code: "12345678903"
```

An invoice with no person with the key, or more than one, is rejected; no
other person is taken in their place.

## Invoice fiscalization request

`racun` sends CIS an invoice request, `RacunZahtjev`, with the schema's
Croatian element names. This is the request built from
`racun/testdata/examples/cash-prices-include-vat.json`, before it is signed,
with the meaning of each element. `racun/invoice.go` gives the Go name of
each field.

```xml
<!-- RacunZahtjev: invoice request. Sign adds an XML-DSig <Signature> that
     points at Id. -->
<RacunZahtjev xmlns="http://www.apis-it.hr/fin/2012/types/f73" Id="RacunZahtjev">
	<!-- Zaglavlje: message header -->
	<Zaglavlje>
		<!-- IdPoruke: message ID, a new UUID for each send -->
		<IdPoruke>00000000-0000-4000-8000-000000000000</IdPoruke>
		<!-- DatumVrijeme: when the message was sent -->
		<DatumVrijeme>06.11.2026T13:58:10</DatumVrijeme>
	</Zaglavlje>
	<!-- Racun: the invoice (račun) -->
	<Racun>
		<!-- Oib: the issuer's OIB, from supplier.tax_id -->
		<Oib>80432461483</Oib>
		<!-- USustPdv (u sustavu PDV-a): the issuer is registered for VAT -->
		<USustPdv>true</USustPdv>
		<!-- DatVrijeme: issue date and time, from issue_date and issue_time -->
		<DatVrijeme>06.11.2026T13:58:10</DatVrijeme>
		<!-- OznSlijed: what the number counts within, P (premises) or N (device) -->
		<OznSlijed>P</OznSlijed>
		<!-- BrRac: invoice number, printed as 1001/POS1/1 -->
		<BrRac>
			<!-- BrOznRac: sequential number, from the invoice code -->
			<BrOznRac>1001</BrOznRac>
			<!-- OznPosPr: business premises label, registered with the Tax
			     Administration -->
			<OznPosPr>POS1</OznPosPr>
			<!-- OznNapUr: device (till) label -->
			<OznNapUr>1</OznNapUr>
		</BrRac>
		<!-- Pdv: VAT, one Porez for each rate -->
		<Pdv>
			<Porez>
				<!-- Stopa: rate, in percent ("general" in GOBL) -->
				<Stopa>25.00</Stopa>
				<!-- Osnovica: taxable base. The prices include VAT, so
				     2 × 2.50 = 5.00 is 4.00 base and 1.00 VAT. -->
				<Osnovica>4.00</Osnovica>
				<!-- Iznos: VAT amount -->
				<Iznos>1.00</Iznos>
			</Porez>
			<Porez>
				<!-- "reduced" in GOBL: 5.65 is 5.00 base and 0.65 VAT -->
				<Stopa>13.00</Stopa>
				<Osnovica>5.00</Osnovica>
				<Iznos>0.65</Iznos>
			</Porez>
		</Pdv>
		<!-- IznosUkupno: total, VAT included -->
		<IznosUkupno>10.65</IznosUkupno>
		<!-- NacinPlac (način plaćanja): payment method, see the table below -->
		<NacinPlac>G</NacinPlac>
		<!-- OibOper: the operator's OIB. Here the issuer's own OIB stands in. -->
		<OibOper>80432461483</OibOper>
		<!-- ZastKod (zaštitni kod): the ZKI, the MD5 of the RSA-SHA256 signature
		     of Oib, DatVrijeme, the three BrRac parts and IznosUkupno -->
		<ZastKod>29f16dcad4dc85441c183d3826c22985</ZastKod>
		<!-- NakDost (naknadna dostava): late delivery, true for an invoice
		     issued while CIS could not be reached and sent afterwards -->
		<NakDost>false</NakDost>
	</Racun>
</RacunZahtjev>
```

| `NacinPlac` | Payment method |
| --- | --- |
| `G` | Cash (gotovina) |
| `K` | Card (kartice) |
| `T` | Bank transfer (transakcijski račun) |
| `O` | Other, or more than one method |

The schema also has elements this example leaves out: consumption tax
(`Pnp`), other taxes (`OstaliPor`), the VAT-exempt amount (`IznosOslobPdv`),
the margin-scheme amount (`IznosMarza`), the amount not subject to tax
(`IznosNePodlOpor`), fees (`Naknade`), the number of an invoice from the
bound paper invoice book, issued while the device was down (`ParagonBrRac`),
a field the Tax Administration reserves (`SpecNamj`) and the buyer's OIB
(`OibPrimateljaRacuna`), given only when the buyer is a business.

CIS answers with `RacunOdgovor`. When it accepts the invoice, the answer holds
the JIR (`Jir`, the unique invoice identifier); when it refuses it, the answer
holds `Greske` (errors), each with a code and a message.

## Development

### Dependencies not yet released

> **Before tagging or merging**, remove the `replace` block in `go.mod`. It
> pins gobl to an unreleased branch commit, through its pseudo-version, so
> that CI can build the module until that branch is released.

| Module | Waits for | Replaced by | Pin |
| --- | --- | --- | --- |
| `github.com/invopop/gobl` | the `hr` regime (branch `eu3-18-hr-regime`, which [invopop/gobl#724](https://github.com/invopop/gobl/pull/724) carries) | branch commit `70ccde1c` | the gobl release that includes it, or a `main` pseudo-version after merge |

To follow a newer commit of the branch, run
`go mod edit -replace=github.com/invopop/gobl=github.com/invopop/gobl@<commit>`
then `go mod tidy`. To work against a local checkout instead, point the
`replace` at its path, and don't commit that.

Add the `release.yaml` workflow of the other GOBL modules at the same time:
it tags every push to `main`, and a tag made while the `replace` block is in
place cannot be built by any consumer.

### Tests

```sh
go test ./...
```

### Examples

`examples/` holds GOBL invoices with the `hr-fiskal-b2c-v1` addon, with their
expected JSON envelopes under `examples/out/`. They are verified via GOBL's
shared `pkg/examples` helpers. Regenerate the golden output after
intentional changes with:

```sh
go test . -run TestExamples -update
```

### Invoice fiscalization (`racun`)

The tests convert the examples in `racun/testdata/examples` and compare them
with `racun/testdata/out` (`-update` writes them again; the ZKI uses the
fixed test key `racun/testdata/test-key.pem`).

Every call to CIS is replayed from a cassette in `racun/testdata/cistest/`:
CIS test's real, signed answer (`<name>-answer.xml`) and what the replay
needs (`<name>.json`: the message ID, when it was sent, the HTTP status and
the JIR or error code). A normal run fiscalizes each scenario (an accepted
invoice, a refused one and each example) through the whole client, offline:
it rebuilds the request for the recorded time and message ID, and checks
CIS's signature on the recorded answer. Only CIS can sign an answer, so the
cassettes come from CIS test. To record them again, run with `-record` and
a FINA demo certificate (keep it in `.certs/`, which git ignores):

```sh
export CIS_TEST_CERT=$PWD/.certs/demo.p12
read -rs CIS_TEST_PASSWORD && export CIS_TEST_PASSWORD
go test ./racun/ -run 'TestCISTest' -record -count=1 -v
unset CIS_TEST_PASSWORD
```

`-xsd` checks signed requests and the expected invoices against
`FiskalizacijaSchema.xsd` v1.10 with xmllint in Docker. CI runs it.

## Sources

- Fiskalizacija računa u krajnjoj potrošnji, tehnička specifikacija v2.7, and `FiskalizacijaSchema.xsd` v1.10 (invoice fiscalization)

## License

Apache 2.0 — see [LICENSE](./LICENSE).
