package myinvestor

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type MyInvestor struct {
	client *http.Client
	url    string
}

func NewMyInvestor() *MyInvestor {
	return &MyInvestor{
		client: &http.Client{},
		url:    "https://api.myinvestor.es/myinvestor-server/rest/public/funds/top",
	}
}

//nolint:tagliatelle,misspell // JSON keys are dictated by the MyInvestor API.
type BestSeller struct {
	ISIN           string   `json:"isin"`
	Name           string   `json:"name"`
	Manager        string   `json:"entity-management"`
	Currency       string   `json:"currency"`
	TER            float64  `json:"ter"`
	Distributing   bool     `json:"fondo-distribucion"`
	ESG            bool     `json:"socialmente-responsable"`
	NoRetrocession bool     `json:"no-retrocession"`
	Return1Y       *float64 `json:"1-year-return"`
	Return3Y       *float64 `json:"3-year-return"`
	Return5Y       *float64 `json:"5-year-return"`
	ReturnsDate    *string  `json:"returns-update-date"`
}

//go:embed best-sellers.json
var bestSellersJSON []byte

func (*MyInvestor) GetBestSellers(context.Context) (_ []BestSeller, err error) {
	var res []BestSeller
	if err := json.Unmarshal(bestSellersJSON, &res); err != nil {
		return nil, fmt.Errorf("json decode: %w", err)
	}
	return res, nil
}

func (my *MyInvestor) RefreshBestSellers(ctx context.Context) (_ []BestSeller, err error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, my.url+"/BEST_SELLERS", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	res, err := my.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do: %w", err)
	}
	defer func() { err = errors.Join(err, res.Body.Close()) }()

	if res.StatusCode != http.StatusOK {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, fmt.Errorf("wrong status: %d, read body: %w", res.StatusCode, err)
		}
		return nil, fmt.Errorf("wrong status: %d, body: %s", res.StatusCode, body)
	}

	var body []BestSeller
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode body: %w", err)
	}
	return body, nil
}
