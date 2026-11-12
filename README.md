# GOBL ➡️ Croatia Fiskal

Croatian Fiskalizacija 1.0 (B2C invoices) addon and CIS client for [GOBL](https://github.com/invopop/gobl).

Released under the Apache 2.0 [LICENSE](https://github.com/invopop/gobl.hr.fiskal/blob/main/LICENSE), Copyright 2026 [Invopop S.L.](https://invopop.com).

[![Lint](https://github.com/invopop/gobl.hr.fiskal/actions/workflows/lint.yaml/badge.svg)](https://github.com/invopop/gobl.hr.fiskal/actions/workflows/lint.yaml)
[![Test Go](https://github.com/invopop/gobl.hr.fiskal/actions/workflows/test.yaml/badge.svg)](https://github.com/invopop/gobl.hr.fiskal/actions/workflows/test.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/invopop/gobl.hr.fiskal)](https://goreportcard.com/report/github.com/invopop/gobl.hr.fiskal)
[![codecov](https://codecov.io/gh/invopop/gobl.hr.fiskal/graph/badge.svg)](https://codecov.io/gh/invopop/gobl.hr.fiskal)
[![GoDoc](https://godoc.org/github.com/invopop/gobl.hr.fiskal?status.svg)](https://godoc.org/github.com/invopop/gobl.hr.fiskal)
![Latest Tag](https://img.shields.io/github/v/tag/invopop/gobl.hr.fiskal)

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

## License

Apache 2.0 — see [LICENSE](./LICENSE).
