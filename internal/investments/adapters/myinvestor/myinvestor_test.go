package myinvestor

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestGetBestSellers(t *testing.T) {
	my := &MyInvestor{}
	res, err := my.GetBestSellers(t.Context())
	require.NoError(t, err)
	assert.NotEmpty(t, res)
}

func TestRefreshBestSellers(t *testing.T) {
	data, err := os.ReadFile("best-sellers.json")
	require.NoError(t, err)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /BEST_SELLERS", func(w http.ResponseWriter, _ *http.Request) {
		_, werr := w.Write(data)
		assert.NoError(t, werr)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	my := &MyInvestor{
		client: srv.Client(),
		url:    srv.URL,
	}

	res, err := my.RefreshBestSellers(t.Context())
	require.NoError(t, err)
	assert.NotEmpty(t, res)
}
