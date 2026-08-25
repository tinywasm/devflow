package devflow

import (
	"fmt"
	"net/http"
	"net/url"
)

// SumDBClient reports whether a module version was ever indexed by the
// public Go checksum database (sum.golang.org). A version that appears
// there was consulted by someone, at some point, possibly with different
// content than what is about to be tagged now — reusing it risks the
// exact "SECURITY ERROR: checksum mismatch" failure this guards against.
type SumDBClient interface {
	Lookup(modulePath, version string) (burned bool, err error)
}

// HTTPSumDB is the real SumDBClient, backed by sum.golang.org's lookup
// endpoint (https://go.dev/design/25530-sumdb#lookup).
type HTTPSumDB struct {
	Client *http.Client // nil = http.DefaultClient
}

func (s *HTTPSumDB) Lookup(modulePath, version string) (bool, error) {
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	u := fmt.Sprintf("https://sum.golang.org/lookup/%s@%s",
		url.PathEscape(modulePath), url.PathEscape(version))
	resp, err := client.Get(u)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, nil // never indexed — free to use
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("sumdb lookup %s@%s: unexpected status %d", modulePath, version, resp.StatusCode)
	}
	return true, nil
}
