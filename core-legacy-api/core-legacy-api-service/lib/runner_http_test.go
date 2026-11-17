package lib

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProbeEndpointsContentType(t *testing.T) {
	for _, path := range []string{"/health", "/readiness"} {
		t.Run(path, func(t *testing.T) {
			resp, err := newProbeTestApp(t).Test(httptest.NewRequest(http.MethodGet, path, nil), -1)

			require.NoError(t, err, "GET %s", path)
			_ = resp.Body.Close()
			assert.Equal(t, "application/json", resp.Header.Get("Content-Type"), "Content-Type of GET %s", path)
		})
	}
}
