module github.com/invopop/gobl.hr.fiskal

go 1.25.0

// UNRELEASED DEPENDENCIES: remove before tagging or merging. This pins the
// unreleased hr regime (gobl) to a branch commit. Pin it to a released
// version (or a main pseudo-version) once it is merged; the README says what
// the pin waits for.
replace github.com/invopop/gobl => github.com/invopop/gobl v0.510.1-0.20261027164024-70ccde1c0e68
