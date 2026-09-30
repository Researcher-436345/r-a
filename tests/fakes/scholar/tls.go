package scholar

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// Hosts are the public names the fake impersonates inside the test network.
// docker-compose.test.yml aliases the fake-scholar container to each of them.
var Hosts = []string{
	"arxiv.org", "www.arxiv.org", "export.arxiv.org",
	"api.openalex.org", "api.crossref.org",
	"www.ebi.ac.uk", "api.semanticscholar.org",
}

// NewCertificates creates a throwaway CA and a server certificate for Hosts.
// The CA PEM is written to certsDir/ca.crt so other containers can trust it
// through SSL_CERT_FILE. Nothing is persisted between runs on purpose.
func NewCertificates(certsDir string) (tls.Certificate, []byte, error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "researcher-test-stand CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return tls.Certificate{}, nil, err
	}

	srvKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	srvTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: Hosts[0]},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     Hosts,
	}
	srvDER, err := x509.CreateCertificate(rand.Reader, srvTemplate, caCert, &srvKey.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, nil, err
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	if certsDir != "" {
		if err := os.MkdirAll(certsDir, 0o755); err != nil {
			return tls.Certificate{}, nil, err
		}
		if err := os.WriteFile(filepath.Join(certsDir, "ca.crt"), caPEM, 0o644); err != nil {
			return tls.Certificate{}, nil, fmt.Errorf("write ca.crt: %w", err)
		}
	}
	return tls.Certificate{Certificate: [][]byte{srvDER, caDER}, PrivateKey: srvKey}, caPEM, nil
}
