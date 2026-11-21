package myinvestor

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-openapi/testify/v2/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestGetBestSellers(t *testing.T) {
	my := &MyInvestor{}
	res, err := my.GetBestSellers(t.Context())
	fmt.Println(res, err)
}

func TestRefreshBestSellers(t *testing.T) {
	data, err := os.ReadFile("best-sellers.json")
	require.NoError(t, err)
	if err != nil {
		t.Fatalf("ReadFile() = %v, want nil", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /BEST_SELLERS", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "%s", string(data))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	my := &MyInvestor{
		client: srv.Client(),
		url:    srv.URL,
	}

	my.RefreshBestSellers(t.Context())
}
