package update

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/HuskerMinion/techo5/echod/internal/lib/sysclock"
)

// The clock, from the update server. A device that can reach no NTP server and has no Home Assistant
// to ask stays years behind, and the update check waits for a clock that never comes: a channel picked
// on the screen then does nothing at all. The one server an update check reaches on any network where
// updates can work is the one serving the manifest, and its every HTTPS reply says what time it is.
//
// Its certificate cannot be checked against the device's clock, since that clock is what is wrong. So
// the chain is checked as of the moment the certificate was issued, which proves a trusted CA issued
// it for this host, and then the server's time has to fall inside the certificate's validity. That is
// the ordinary check, made at the time the server gives. The manifest is still believed only with the
// release key's signature. The time can be a minute or two old when a cache in front of the server
// answers, which is nothing to a certificate, and NTP corrects it as soon as it gets an answer.

// setClock is sysclock.Set, a variable so a test does not set the machine's clock.
var setClock = sysclock.Set

// clockRoots are the CAs the server's certificate is checked against: nil is the system's, which is
// what the updater's client uses too. A test sets its own server's.
var clockRoots *x509.CertPool

// clockFromServer sets the clock from the Date of a reply from the manifest's host.
func clockFromServer(ctx context.Context, url string) error {
	at, err := serverTime(ctx, url)
	if err != nil {
		return err
	}
	if err := setClock(at); err != nil {
		return err
	}
	slog.Info("clock set from the update server", "now", at.UTC().Format(time.RFC3339))
	return nil
}

// serverTime asks url's host for the time, with a HEAD that does not follow a redirect: GitHub answers
// a release download with one, and the redirect carries the time as well as anything would.
func serverTime(ctx context.Context, url string) (time.Time, error) {
	var leaf *x509.Certificate
	tr := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		TLSHandshakeTimeout: 15 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			// Not skipped: VerifyConnection makes the check, at the certificate's issue time.
			InsecureSkipVerify: true,
			VerifyConnection: func(cs tls.ConnectionState) error {
				if len(cs.PeerCertificates) == 0 {
					return errors.New("update: the server sent no certificate")
				}
				l := cs.PeerCertificates[0]
				inter := x509.NewCertPool()
				for _, c := range cs.PeerCertificates[1:] {
					inter.AddCert(c)
				}
				if _, err := l.Verify(x509.VerifyOptions{
					DNSName:       cs.ServerName,
					Roots:         clockRoots,
					Intermediates: inter,
					CurrentTime:   l.NotBefore,
				}); err != nil {
					return err
				}
				leaf = l
				return nil
			},
		},
	}
	defer tr.CloseIdleConnections()
	c := &http.Client{
		Transport:     tr,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return time.Time{}, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return time.Time{}, fmt.Errorf("update: asking %s the time: %w", req.URL.Host, err)
	}
	resp.Body.Close()
	if leaf == nil {
		return time.Time{}, fmt.Errorf("update: %s is not HTTPS; its time is not taken", req.URL.Host)
	}
	at, err := http.ParseTime(resp.Header.Get("Date"))
	if err != nil {
		return time.Time{}, fmt.Errorf("update: %s sent no time: %w", req.URL.Host, err)
	}
	if at.Before(leaf.NotBefore) || at.After(leaf.NotAfter) || at.Year() < 2025 {
		return time.Time{}, fmt.Errorf("update: %s says it is %s, outside its own certificate", req.URL.Host, at.UTC().Format(time.RFC3339))
	}
	return at, nil
}
