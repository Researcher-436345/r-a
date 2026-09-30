// fake-scholar impersonates arXiv / OpenAlex / Crossref inside the test
// network: :80 and :443 (TLS with a self-issued CA written to CERTS_DIR) for
// the backend, :8093 for health and the control API.
package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"

	"github.com/centraluniversity/researcher/tests/fakes/scholar"
)

func main() {
	certsDir := os.Getenv("CERTS_DIR")
	if certsDir == "" {
		certsDir = "/certs"
	}
	cert, caPEM, err := scholar.NewCertificates(certsDir)
	if err != nil {
		log.Fatalf("fake-scholar: certificates: %v", err)
	}
	srv := scholar.New(caPEM)
	public := srv.Public()

	go func() {
		log.Printf("fake-scholar: http on :80")
		log.Fatal(http.ListenAndServe(":80", public))
	}()
	go func() {
		tlsSrv := &http.Server{Addr: ":443", Handler: public, TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}}}
		log.Printf("fake-scholar: https on :443 for %v", scholar.Hosts)
		log.Fatal(tlsSrv.ListenAndServeTLS("", ""))
	}()
	log.Printf("fake-scholar: control on :8093, CA at %s/ca.crt", certsDir)
	log.Fatal(http.ListenAndServe(":8093", srv.Control()))
}
