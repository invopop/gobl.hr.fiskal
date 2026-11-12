package racun

import (
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

// oidOrganizationIdentifier is the subject attribute where a certificate
// for a legal person carries its identifier (ETSI EN 319 412-1, §5.1.4),
// which the spec requires to be the OIB (v2.7, §5).
var oidOrganizationIdentifier = asn1.ObjectIdentifier{2, 5, 4, 97}

// organizationOIB matches an organizationIdentifier that holds an OIB:
// "HR" and the OIB, as FINA's Fiskal certificates write it, or the ETSI
// semantics identifiers "VATHR-" and "NTRHR-" and the OIB.
var organizationOIB = regexp.MustCompile(`^(?:HR|VATHR-|NTRHR-)(\d{11})$`)

// Certificate is the issuer's fiscalization certificate and its private key.
// It signs requests and computes the ZKI.
type Certificate struct {
	Cert *x509.Certificate
	Key  crypto.Signer
	// Chain holds the intermediate certificates, if any.
	Chain []*x509.Certificate
}

// OIB returns the OIB the certificate is issued to, from its
// organizationIdentifier. CIS refuses a request whose issuer has another
// OIB (s005).
func (c *Certificate) OIB() (string, error) {
	for _, attr := range c.Cert.Subject.Names {
		if !attr.Type.Equal(oidOrganizationIdentifier) {
			continue
		}
		id, _ := attr.Value.(string)
		if m := organizationOIB.FindStringSubmatch(id); m != nil {
			return m[1], nil
		}
		return "", fmt.Errorf("the certificate's organizationIdentifier %q holds no OIB", id)
	}
	return "", errors.New("the certificate has no organizationIdentifier with an OIB")
}

// check returns an error unless the certificate can sign at now: it has a
// certificate and a key, the key is RSA and belongs to the certificate, now
// is within the certificate's validity dates, and it carries an OIB. CIS
// would refuse a request signed otherwise (s002, s003, s004), but only after
// the invoice was issued.
func (c *Certificate) check(now time.Time) error {
	if c == nil || c.Cert == nil || c.Key == nil {
		return errors.New("no certificate")
	}
	if _, err := c.OIB(); err != nil {
		return err
	}
	pub, ok := c.Key.Public().(*rsa.PublicKey)
	if !ok {
		return errors.New("the private key is not RSA")
	}
	if !pub.Equal(c.Cert.PublicKey) {
		return errors.New("the private key does not belong to the certificate")
	}
	if now.Before(c.Cert.NotBefore) {
		return fmt.Errorf("the certificate is not valid until %s", c.Cert.NotBefore.Format(time.RFC3339))
	}
	if now.After(c.Cert.NotAfter) {
		return fmt.Errorf("the certificate expired at %s", c.Cert.NotAfter.Format(time.RFC3339))
	}
	return nil
}

// LoadCertificate reads a certificate and its private key from a PKCS#12
// file (.p12, .pfx) opened with the password.
func LoadCertificate(path, password string) (*Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseCertificate(data, password)
}

// ParseCertificate reads a certificate and its private key from PKCS#12
// data opened with the password. The key must be RSA, which is what the
// specification allows (§7).
func ParseCertificate(data []byte, password string) (*Certificate, error) {
	key, cert, chain, err := pkcs12.DecodeChain(data, password)
	if err != nil {
		return nil, fmt.Errorf("decoding PKCS#12: %w", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("the private key is not RSA")
	}
	return &Certificate{Cert: cert, Key: rsaKey, Chain: chain}, nil
}
