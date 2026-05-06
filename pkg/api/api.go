package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Finnhub-Stock-API/finnhub-go/v2"
)

type Provider string

const (
	ProviderFinnhub Provider = "finnhub"
	ProviderYahoo   Provider = "yahoo"
)

type SeriesRange string

const (
	RangeDay   SeriesRange = "day"
	RangeMonth SeriesRange = "month"
	RangeYear  SeriesRange = "year"
)

type Series struct {
	Points        []float32
	BasePrice     float32
	ChangePercent float32
}

type Quote struct {
	Price         float32
	Change        float32
	ChangePercent float32
}

type Result struct {
	Quote        Quote
	MarketStatus finnhub.MarketStatus
	StatusText   string
	StatusIsOpen bool
}

const defaultHTTPTimeout = 10 * time.Second

var (
	apiMu      sync.Mutex
	apiKey     string
	apiClient  *finnhub.APIClient
	httpClient = &http.Client{Timeout: defaultHTTPTimeout}
)

func APIKey() string {
	apiMu.Lock()
	defer apiMu.Unlock()
	return apiKey
}

func SetAPIKey(key string) {
	apiMu.Lock()
	defer apiMu.Unlock()
	if key == "" {
		log.Println("Clearing API key")
	} else {
		log.Printf("Setting API key (%d chars)", len(key))
	}
	apiKey = key
}

func normalizeProvider(provider string) Provider {
	switch strings.ToLower(provider) {
	case string(ProviderYahoo):
		return ProviderYahoo
	default:
		return ProviderFinnhub
	}
}

var (
	clientMu sync.Mutex
)

func finnhubService() *finnhub.DefaultApiService {
	clientMu.Lock()
	defer clientMu.Unlock()

	currentAPIKey := APIKey()
	if currentAPIKey == "" {
		return nil
	}
	config := finnhub.NewConfiguration()
	config.AddDefaultHeader("X-Finnhub-Token", currentAPIKey)
	apiClient = finnhub.NewAPIClient(config)
	return apiClient.DefaultApi
}

var (
	marketStatus     *finnhub.MarketStatus
	marketStatusTime time.Time
	lastCallMu       sync.Mutex
	lastCallTime     time.Time
)

type cachedSeries struct {
	series     Series
	expiration time.Time
}

var (
	seriesMu    sync.Mutex
	seriesCache = make(map[string]cachedSeries)
)

func throttle() {
	lastCallMu.Lock()
	timeSinceLast := time.Since(lastCallTime)
	if timeSinceLast < 1*time.Second {
		time.Sleep(1*time.Second - timeSinceLast)
	}
	lastCallTime = time.Now()
	lastCallMu.Unlock()
}

func GetQuote(symbol string, provider string) (*Result, error) {
	switch normalizeProvider(provider) {
	case ProviderYahoo:
		return getYahooQuote(symbol)
	default:
		return getFinnhubQuote(symbol)
	}
}

func getFinnhubQuote(symbol string) (*Result, error) {
	service := finnhubService()
	if service == nil {
		return nil, fmt.Errorf("API client was nil")
	}
	ctx := context.Background()
	if marketStatus == nil || time.Since(marketStatusTime) >= 5*time.Minute {
		for {
			throttle()
			ms, response, err := service.MarketStatus(ctx).Exchange("US").Execute()
			marketStatus = &ms
			if err != nil {
				if response != nil && response.StatusCode == 429 {
					time.Sleep(5 * time.Second)
					continue
				}
				return nil, fmt.Errorf("market status: %w", err)
			}
			break
		}
		marketStatusTime = time.Now()
	}
	for {
		throttle()
		q, response, err := service.Quote(ctx).Symbol(symbol).Execute()
		if err != nil {
			if response != nil && response.StatusCode == 429 {
				time.Sleep(5 * time.Second)
				continue
			}
			return nil, fmt.Errorf("quote %s: %w", symbol, err)
		}
		return &Result{
			Quote: Quote{
				Price:         q.GetC(),
				Change:        q.GetD(),
				ChangePercent: q.GetDp(),
			},
			MarketStatus: *marketStatus,
		}, nil
	}
}

func GetSeries(symbol, provider string, seriesRange SeriesRange, quote Quote) (*Series, error) {
	cacheKey := fmt.Sprintf("%s:%s:%s", normalizeProvider(provider), symbol, seriesRange)
	seriesMu.Lock()
	cached, ok := seriesCache[cacheKey]
	if ok && time.Now().Before(cached.expiration) {
		series := cached.series
		if len(series.Points) > 0 {
			series.Points[len(series.Points)-1] = quote.Price
		}
		if series.BasePrice != 0 {
			series.ChangePercent = ((quote.Price - series.BasePrice) / series.BasePrice) * 100
		}
		seriesMu.Unlock()
		return &series, nil
	}
	seriesMu.Unlock()

	var (
		series     *Series
		expiration time.Duration
		err        error
	)
	switch normalizeProvider(provider) {
	case ProviderYahoo:
		series, expiration, err = getYahooSeries(symbol, seriesRange, quote)
	default:
		series, expiration, err = getFinnhubSeries(symbol, seriesRange, quote)
	}
	if err != nil {
		return nil, err
	}
	seriesMu.Lock()
	seriesCache[cacheKey] = cachedSeries{
		series:     *series,
		expiration: time.Now().Add(expiration),
	}
	seriesMu.Unlock()
	return series, nil
}

func getFinnhubSeries(symbol string, seriesRange SeriesRange, quote Quote) (*Series, time.Duration, error) {
	service := finnhubService()
	if service == nil {
		return nil, 0, fmt.Errorf("API client was nil")
	}
	resolution := "D"
	now := time.Now()
	from := now.AddDate(0, 0, -30)
	expiration := 30 * time.Minute

	switch seriesRange {
	case RangeDay:
		resolution = "5"
		from = now.Add(-24 * time.Hour)
		expiration = 5 * time.Minute
	case RangeMonth:
		resolution = "D"
		from = now.AddDate(0, -1, 0)
		expiration = 1 * time.Hour
	case RangeYear:
		resolution = "D"
		from = now.AddDate(-1, 0, 0)
		expiration = 6 * time.Hour
	}

	ctx := context.Background()
	for {
		throttle()
		candles, response, err := service.StockCandles(ctx).
			Symbol(symbol).
			Resolution(resolution).
			From(from.Unix()).
			To(now.Unix()).
			Execute()
		if err != nil {
			if response != nil && response.StatusCode == 429 {
				time.Sleep(5 * time.Second)
				continue
			}
			return nil, 0, fmt.Errorf("error getting candles for %s: %w", symbol, err)
		}
		points := append([]float32(nil), candles.GetC()...)
		if len(points) == 0 {
			return nil, 0, fmt.Errorf("no candles returned for %s", symbol)
		}
		points[len(points)-1] = quote.Price
		base := points[0]
		if seriesRange == RangeDay {
			base = quote.Price - quote.Change
		}
		if base == 0 {
			base = points[0]
		}
		return &Series{
			Points:        points,
			BasePrice:     base,
			ChangePercent: ((quote.Price - base) / base) * 100,
		}, expiration, nil
	}
}

type yahooChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				RegularMarketPrice   float64 `json:"regularMarketPrice"`
				PreviousClose        float64 `json:"previousClose"`
				ChartPreviousClose   float64 `json:"chartPreviousClose"`
				CurrentTradingPeriod struct {
					Regular struct {
						Start int64 `json:"start"`
						End   int64 `json:"end"`
					} `json:"regular"`
				} `json:"currentTradingPeriod"`
			} `json:"meta"`
			Indicators struct {
				Quote []struct {
					Open   []*float64 `json:"open"`
					Close  []*float64 `json:"close"`
					High   []*float64 `json:"high"`
					Low    []*float64 `json:"low"`
					Volume []*float64 `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error interface{} `json:"error"`
	} `json:"chart"`
}

func yahooChartURL(symbol, interval, rangeParam string) string {
	values := url.Values{}
	values.Set("interval", interval)
	values.Set("range", rangeParam)
	values.Set("includePrePost", "true")
	return fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?%s", url.PathEscape(symbol), values.Encode())
}

func fetchYahooChart(symbol, interval, rangeParam string) (*yahooChartResponse, error) {
	req, err := http.NewRequest(http.MethodGet, yahooChartURL(symbol, interval, rangeParam), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "stock-ticker-stream-deck-plugin/1.0")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", resp.Status)
	}
	var chart yahooChartResponse
	if err := json.NewDecoder(resp.Body).Decode(&chart); err != nil {
		return nil, err
	}
	if len(chart.Chart.Result) == 0 {
		return nil, fmt.Errorf("no chart result")
	}
	return &chart, nil
}

func getYahooQuote(symbol string) (*Result, error) {
	chart, err := fetchYahooChart(symbol, "1d", "5d")
	if err != nil {
		return nil, fmt.Errorf("yahoo quote %s: %w", symbol, err)
	}
	result := chart.Chart.Result[0]
	price := float32(result.Meta.RegularMarketPrice)
	prevClose := float32(result.Meta.PreviousClose)
	if prevClose == 0 {
		prevClose = float32(result.Meta.ChartPreviousClose)
	}
	change := price - prevClose
	changePercent := float32(0)
	if prevClose != 0 {
		changePercent = (change / prevClose) * 100
	}
	now := time.Now().Unix()
	statusText := "POST"
	isOpen := false
	if now >= result.Meta.CurrentTradingPeriod.Regular.Start && now <= result.Meta.CurrentTradingPeriod.Regular.End {
		statusText = "OPEN"
		isOpen = true
	} else if now < result.Meta.CurrentTradingPeriod.Regular.Start {
		statusText = "PRE"
	}
	return &Result{
		Quote: Quote{
			Price:         price,
			Change:        change,
			ChangePercent: changePercent,
		},
		StatusText:   statusText,
		StatusIsOpen: isOpen,
	}, nil
}

func getYahooSeries(symbol string, seriesRange SeriesRange, quote Quote) (*Series, time.Duration, error) {
	interval := "1d"
	rangeParam := "1mo"
	expiration := 1 * time.Hour
	switch seriesRange {
	case RangeDay:
		interval = "5m"
		rangeParam = "1d"
		expiration = 5 * time.Minute
	case RangeMonth:
		interval = "1d"
		rangeParam = "1mo"
		expiration = 1 * time.Hour
	case RangeYear:
		interval = "1d"
		rangeParam = "1y"
		expiration = 6 * time.Hour
	}
	chart, err := fetchYahooChart(symbol, interval, rangeParam)
	if err != nil {
		return nil, 0, fmt.Errorf("yahoo series %s: %w", symbol, err)
	}
	result := chart.Chart.Result[0]
	if len(result.Indicators.Quote) == 0 {
		return nil, 0, fmt.Errorf("missing quote series")
	}
	closes := result.Indicators.Quote[0].Close
	points := make([]float32, 0, len(closes))
	base := float32(0)
	for _, closeValue := range closes {
		if closeValue == nil {
			continue
		}
		v := float32(*closeValue)
		if base == 0 {
			base = v
		}
		points = append(points, v)
	}
	if len(points) == 0 {
		return nil, 0, fmt.Errorf("no closes returned")
	}
	points[len(points)-1] = quote.Price
	if seriesRange == RangeDay {
		base = quote.Price - quote.Change
	}
	if base == 0 {
		base = points[0]
	}
	return &Series{
		Points:        points,
		BasePrice:     base,
		ChangePercent: ((quote.Price - base) / base) * 100,
	}, expiration, nil
}
