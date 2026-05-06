package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shayne/go-streamdeck-sdk"

	"github.com/shayne/stock-ticker-stream-deck-plugin/pkg/api"
)

type tile struct {
	context  string
	title    string
	symbol   string
	apikey   string
	provider string
	view     int
}

type plugin struct {
	sd    *streamdeck.StreamDeck
	tiles map[string]*tile
}

type evSdpiCollection struct {
	Group     bool     `json:"group"`
	Index     int      `json:"index"`
	Key       string   `json:"key"`
	Selection []string `json:"selection"`
	Value     string   `json:"value"`
}

type actionSettings struct {
	Symbol   string `json:"symbol,omitempty"`
	APIKey   string `json:"apikey,omitempty"`
	Provider string `json:"provider,omitempty"`
	View     int    `json:"view,omitempty"`
}

const (
	viewCurrent = 1
	viewDay     = 2
	viewMonth   = 3
	viewYear    = 4
)

const (
	statusRegular = "sun"
	statusPre     = "pre"
	statusAfter   = "moon"
	arrowUp       = "^"
	arrowDown     = "v"
)

func newPlugin(port, uuid, event, info string) *plugin {
	sd := streamdeck.NewStreamDeck(port, uuid, event, info)
	p := &plugin{sd: sd, tiles: make(map[string]*tile)}
	sd.SetDelegate(p)
	return p
}

func normalizeProvider(provider string) string {
	if strings.EqualFold(provider, string(api.ProviderYahoo)) {
		return string(api.ProviderYahoo)
	}
	return string(api.ProviderFinnhub)
}

func normalizeSettings(settings actionSettings) actionSettings {
	settings.Symbol = strings.ToUpper(strings.TrimSpace(settings.Symbol))
	settings.Provider = normalizeProvider(settings.Provider)
	if settings.View < viewCurrent || settings.View > viewYear {
		settings.View = viewCurrent
	}
	return settings
}

func settingsFromTile(t *tile) actionSettings {
	return actionSettings{
		Symbol:   t.symbol,
		APIKey:   t.apikey,
		Provider: t.provider,
		View:     t.view,
	}
}

func (p *plugin) renderTile(t *tile, symbol string, result *api.Result) (*[]byte, string) {
	price := result.Quote.Price
	change := result.Quote.Change
	changePercent := result.Quote.ChangePercent

	statusColor := orange
	status := statusAfter
	if t.provider == string(api.ProviderYahoo) {
		switch result.StatusText {
		case "OPEN":
			status = statusRegular
		case "PRE":
			status = statusPre
		default:
			statusColor = blue
			status = statusAfter
		}
	} else {
		switch result.MarketStatus.GetSession() {
		case "regular":
			status = statusRegular
		case "pre-market":
			status = statusPre
		default:
			statusColor = blue
			status = statusAfter
		}
	}

	arrow := arrowDown
	arrowColor := red
	if change > 0 {
		arrow = arrowUp
		arrowColor = green
	} else if change == 0 {
		arrow = ""
	}

	title := symbol
	if t.title != "" {
		title = t.title
	}

	if t.view != viewCurrent {
		seriesRange := api.RangeDay
		rangeLabel := "D"
		switch t.view {
		case viewMonth:
			seriesRange = api.RangeMonth
			rangeLabel = "M"
		case viewYear:
			seriesRange = api.RangeYear
			rangeLabel = "Y"
		}
		series, err := api.GetSeries(symbol, t.provider, seriesRange, price)
		if err != nil {
			log.Printf("error getting series for %s: %v", symbol, err)
		} else if len(series.Points) > 1 {
			return nil, BuildHistoryTileSVG(title, price, series.Points, series.ChangePercent, rangeLabel)
		}
	}

	return DrawTile(title, price, change, changePercent, status, statusColor, arrow, arrowColor), ""
}

var (
	updateMu = sync.Mutex{}
)

func (p *plugin) updateTiles(tiles []*tile) {
	go p.goUpdateTiles(tiles)
}

func (p *plugin) goUpdateTiles(tiles []*tile) {
	updateMu.Lock()
	defer updateMu.Unlock()

	for _, t := range tiles {
		if t.symbol == "" {
			continue
		}

		result, err := api.GetQuote(t.symbol, t.provider)
		if err != nil {
			log.Printf("error getting quote for %s: %v", t.symbol, err)
			continue
		}

		b, svg := p.renderTile(t, t.symbol, result)
		if svg != "" {
			err = p.sd.SetImageDataURL(t.context, fmt.Sprintf("data:image/svg+xml,%s", url.PathEscape(svg)))
		} else {
			err = p.sd.SetImage(t.context, *b)
		}
		if err != nil {
			log.Printf("sd.SetImage for %s: %v", t.symbol, err)
		}
	}
}

func (p *plugin) startUpdateLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		p.updateAllTiles()
	}
}

func (p *plugin) Run() {
	err := p.sd.Connect()
	if err != nil {
		log.Fatalf("Connect: %v\n", err)
	}
	go p.startUpdateLoop()
	p.sd.ListenAndWait()
}

func (p plugin) OnConnected(*websocket.Conn) {}

func (p plugin) OnWillAppear(ev *streamdeck.EvWillAppear) {
	if t, ok := p.tiles[ev.Context]; ok {
		p.updateTiles([]*tile{t})
		return
	}

	var settings actionSettings
	err := json.Unmarshal(*ev.Payload.Settings, &settings)
	if err != nil {
		log.Println("OnWillAppear settings unmarshal", err)
	}
	settings = normalizeSettings(settings)

	var updateAll bool
	apiKey := api.APIKey()
	if apiKey == "" && settings.APIKey != "" {
		apiKey = settings.APIKey
		api.SetAPIKey(apiKey)
		updateAll = true
	}

	t := &tile{
		context:  ev.Context,
		symbol:   settings.Symbol,
		apikey:   apiKey,
		provider: settings.Provider,
		view:     settings.View,
	}
	p.tiles[ev.Context] = t

	if updateAll {
		for _, tile := range p.tiles {
			tile.apikey = apiKey
		}
		p.updateAllTiles()
		return
	}
	p.updateTiles([]*tile{t})
}

func (p plugin) OnKeyDown(ev *streamdeck.EvKeyPress) {
	t := p.tiles[ev.Context]
	if t == nil {
		return
	}

	t.view++
	if t.view > viewYear {
		t.view = viewCurrent
	}

	settings := settingsFromTile(t)
	err := p.sd.SetSettings(ev.Context, &settings)
	if err != nil {
		log.Printf("setSettings on keyDown: %v", err)
	}
	p.updateTiles([]*tile{t})
}

func (p plugin) updateAllTiles() {
	var tiles []*tile
	for _, t := range p.tiles {
		tiles = append(tiles, t)
	}
	p.updateTiles(tiles)
}

func (p plugin) OnTitleParametersDidChange(ev *streamdeck.EvTitleParametersDidChange) {
	t := p.tiles[ev.Context]
	if t == nil {
		log.Println("OnTitleParametersDidChange: tile not found")
		return
	}
	t.title = ev.Payload.Title
}

func (p plugin) OnPropertyInspectorConnected(ev *streamdeck.EvSendToPlugin) {
	if t, ok := p.tiles[ev.Context]; ok {
		settings := settingsFromTile(t)
		settings.APIKey = api.APIKey()
		p.sd.SendToPropertyInspector(ev.Action, ev.Context, &settings)
	}
}

func (p plugin) OnSendToPlugin(ev *streamdeck.EvSendToPlugin) {
	var payload map[string]*json.RawMessage
	err := json.Unmarshal(*ev.Payload, &payload)
	if err != nil {
		log.Println("OnSendToPlugin unmarshal", err)
		return
	}

	data, ok := payload["sdpi_collection"]
	if !ok {
		return
	}

	var sdpi evSdpiCollection
	err = json.Unmarshal(*data, &sdpi)
	if err != nil {
		log.Println("SDPI unmarshal", err)
		return
	}

	t := p.tiles[ev.Context]
	if t == nil {
		log.Printf("tile was nil, creating new tile for %s", ev.Context)
		t = &tile{
			context:  ev.Context,
			provider: string(api.ProviderFinnhub),
			view:     viewCurrent,
		}
		p.tiles[ev.Context] = t
	}

	settings := settingsFromTile(t)
	settings.APIKey = api.APIKey()

	var updateAll bool
	switch sdpi.Key {
	case "symbol":
		t.symbol = strings.ToUpper(strings.TrimSpace(sdpi.Value))
		settings.Symbol = t.symbol
	case "apikey":
		t.apikey = strings.TrimSpace(sdpi.Value)
		settings.APIKey = t.apikey
		api.SetAPIKey(t.apikey)
		updateAll = true
	case "provider":
		t.provider = normalizeProvider(sdpi.Value)
		settings.Provider = t.provider
	}

	if updateAll {
		apikey := api.APIKey()
		for _, tile := range p.tiles {
			tile.apikey = apikey
			current := settingsFromTile(tile)
			current.APIKey = apikey
			err := p.sd.SetSettings(tile.context, current)
			if err != nil {
				log.Printf("error setting settings: %v", err)
			}
		}
		p.updateAllTiles()
		return
	}

	log.Printf("saving settings: %+v", settings)
	err = p.sd.SetSettings(ev.Context, &settings)
	if err != nil {
		log.Printf("setSettings: %v", err)
		return
	}
	p.updateTiles([]*tile{t})
}

func (p plugin) OnApplicationDidLaunch(*streamdeck.EvApplication) {}

func (p plugin) OnApplicationDidTerminate(*streamdeck.EvApplication) {}
