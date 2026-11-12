package racun

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCertificateCheck(t *testing.T) {
	cert := testCertificate(t)
	now := time.Now()

	t.Run("a valid certificate", func(t *testing.T) {
		assert.NoError(t, cert.check(now))
	})
	t.Run("no certificate", func(t *testing.T) {
		var nilCert *Certificate
		assert.EqualError(t, nilCert.check(now), "no certificate")
		assert.EqualError(t, (&Certificate{Key: cert.Key}).check(now), "no certificate")
		assert.EqualError(t, (&Certificate{Cert: cert.Cert}).check(now), "no certificate")
	})
	t.Run("a key that is not RSA", func(t *testing.T) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		assert.EqualError(t, (&Certificate{Cert: cert.Cert, Key: key}).check(now), "the private key is not RSA")
	})
	t.Run("a key from another certificate", func(t *testing.T) {
		other := &Certificate{Cert: cert.Cert, Key: testCertificate(t).Key}
		assert.EqualError(t, other.check(now), "the private key does not belong to the certificate")
	})
	t.Run("not valid yet", func(t *testing.T) {
		assert.ErrorContains(t, cert.check(cert.Cert.NotBefore.Add(-time.Second)), "the certificate is not valid until")
	})
	t.Run("expired", func(t *testing.T) {
		assert.ErrorContains(t, cert.check(cert.Cert.NotAfter.Add(time.Second)), "the certificate expired at")
	})
	t.Run("no OIB", func(t *testing.T) {
		noOIB := withSubject(t, cert, pkix.Name{CommonName: "FISKAL TEST", Country: []string{"HR"}})
		assert.EqualError(t, noOIB.check(now), "the certificate has no organizationIdentifier with an OIB")
	})
}

func TestCertificateOIB(t *testing.T) {
	cert := testCertificate(t)
	for name, tc := range map[string]struct {
		organizationID string
		oib            string
		err            string
	}{
		"FINA Fiskal":                           {organizationID: "HR80432461483", oib: "80432461483"},
		"ETSI VAT identifier":                   {organizationID: "VATHR-80432461483", oib: "80432461483"},
		"ETSI national registration identifier": {organizationID: "NTRHR-80432461483", oib: "80432461483"},
		"another country":                       {organizationID: "VATES-B12345678", err: `the certificate's organizationIdentifier "VATES-B12345678" holds no OIB`},
		"too few digits":                        {organizationID: "HR8043246148", err: `the certificate's organizationIdentifier "HR8043246148" holds no OIB`},
	} {
		t.Run(name, func(t *testing.T) {
			oib, err := withSubject(t, cert, finaSubject("FISKAL TEST", tc.organizationID)).OIB()
			if tc.err != "" {
				assert.EqualError(t, err, tc.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.oib, oib)
		})
	}
}

// withSubject is the certificate issued again, with the same key, to
// another subject.
func withSubject(t *testing.T, cert *Certificate, subject pkix.Name) *Certificate {
	t.Helper()
	tmpl := *cert.Cert
	tmpl.Subject = subject
	tmpl.RawSubject = nil // it would win over Subject
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, cert.Key.Public(), cert.Key)
	require.NoError(t, err)
	c, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return &Certificate{Cert: c, Key: cert.Key}
}

// New, Sign and ZKI refuse a certificate or key they can't use, before
// anything reaches CIS.
func TestCertificateUse(t *testing.T) {
	expired := testCertificate(t)
	expired.Cert.NotAfter = time.Now().Add(-time.Minute)

	t.Run("New refuses an expired certificate", func(t *testing.T) {
		_, err := New(&Config{Environment: EnvironmentSandbox, Certificate: expired})
		assert.ErrorContains(t, err, "signing certificate: the certificate expired at")
	})
	t.Run("Sign refuses an expired certificate", func(t *testing.T) {
		_, err := Sign(expired, testRequest())
		assert.ErrorContains(t, err, "the certificate expired at")
	})
	t.Run("ZKI refuses a key that is not RSA", func(t *testing.T) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		_, err = ZKI(key, testZKIInput())
		assert.EqualError(t, err, "the key is not RSA")
	})
	t.Run("ZKI takes any RSA signer", func(t *testing.T) {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		_, err = ZKI(signer{key}, testZKIInput())
		assert.NoError(t, err)
	})
}

// signer is an RSA key behind the crypto.Signer interface alone, as a key
// in a KMS or an HSM is.
type signer struct{ *rsa.PrivateKey }
