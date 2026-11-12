module github.com/invopop/gobl.hr.fiskal

go 1.25.0

// UNRELEASED DEPENDENCIES: remove before tagging or merging. This pins the
// unreleased hr regime (gobl) to a branch commit. Pin it to a released
// version (or a main pseudo-version) once it is merged; the README says what
// the pin waits for.
replace github.com/invopop/gobl => github.com/invopop/gobl v0.510.1-0.20261027164024-70ccde1c0e68

require (
	github.com/beevik/etree v1.8.1
	github.com/google/uuid v1.6.0
	github.com/invopop/gobl v0.512.0
	github.com/russellhaering/goxmldsig v1.6.1
	github.com/stretchr/testify v1.11.1
	software.sslmate.com/src/go-pkcs12 v0.7.4
)

require (
	cloud.google.com/go v0.116.0 // indirect
	github.com/Masterminds/semver/v3 v3.3.0 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/buger/jsonparser v1.1.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/expr-lang/expr v1.17.8 // indirect
	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
	github.com/invopop/jsonschema v0.14.0 // indirect
	github.com/invopop/yaml v0.3.1 // indirect
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/magefile/mage v1.15.0 // indirect
	github.com/pb33f/ordered-map/v2 v2.3.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.yaml.in/yaml/v4 v4.0.0-rc.2 // indirect
	golang.org/x/crypto v0.27.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
