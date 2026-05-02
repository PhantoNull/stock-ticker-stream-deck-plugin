package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strconv"
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

type settingsType map[string]string

const (
	viewCurrent = 1
	viewDay     = 2
	viewMonth   = 3
	viewYear    = 4
)

func newPlugin(port, uuid, event, info string) *plugin {
	sd := streamdeck.NewStreamDeck(port, uuid, event, info)
	p := &plugin{sd: sd, tiles: make(map[string]*tile)}
	sd.SetDelegate(p)
	return p
}

func normalizeProvider(provider string) string {
	if strings.ToLower(provider) == string(api.ProviderYahoo) {
		return string(api.ProviderYahoo)
	}
	return string(api.ProviderFinnhub)
}

func (p *plugin) renderTile(t *tile, symbol string, result *api.Result) (*[]byte, string) {
	price := result.Quote.Price
	change := result.Quote.Change
	changePercent := result.Quote.ChangePercent

	statusColor := orange
	status := "A"
	if t.provider == string(api.ProviderYahoo) {
		switch result.StatusText {
		case "OPEN":
			status = "O"
		case "PRE":
			status = "P"
		default:
			statusColor = blue
			status = "A"
		}
	} else {
		switch result.MarketStatus.GetSession() {
		case "regular":
			status = "î¤£"
		case "pre-market":
			status = "î¤¶"
		default:
			statusColor = blue
			status = "î¤µ"
		}
	}

	arrow := "î¤ˆ"
	arrowColor := red
	if change > 0 {
		arrow = "î¤‰"
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
			log.Printf("Error getting quote for %s: %v\n", t.symbol, err)
			continue
		}
		b, svg := p.renderTile(t, t.symbol, result)
		if svg != "" {
			err = p.sd.SetImageDataURL(t.context, fmt.Sprintf("data:image/svg+xml,%s", url.PathEscape(svg)))
		} else {
			err = p.sd.SetImage(t.context, *b)
		}
		if err != nil {
			log.Fatalf("sd.SetImage: %v\n", err)
		}
	}
}

func (p *plugin) startUpdateLoop() {
	tick := time.Tick(5 * time.Minute)
	for range tick {
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

func parseView(settings settingsType) int {
	view, err := strconv.Atoi(settings["view"])
	if err != nil || view < viewCurrent || view > viewYear {
		return viewCurrent
	}
	return view
}

func (p plugin) OnWillAppear(ev *streamdeck.EvWillAppear) {
	if t, ok := p.tiles[ev.Context]; ok {
		p.updateTiles([]*tile{t})
		return
	}

	var settings settingsType
	err := json.Unmarshal(*ev.Payload.Settings, &settings)
	if err != nil {
		log.Println("OnWillAppear settings unmarshal", err)
	}

	var updateAll bool
	apiKey := api.APIKey()
	if apiKey == "" && settings["apikey"] != "" {
		apiKey = settings["apikey"]
		api.SetAPIKey(apiKey)
		updateAll = true
	}

	t := &tile{
		context:  ev.Context,
		symbol:   settings["symbol"],
		apikey:   apiKey,
		provider: normalizeProvider(settings["provider"]),
		view:     parseView(settings),
	}
	p.tiles[ev.Context] = t

	if updateAll {
		for _, tile := range p.tiles {
			tile.apikey = apiKey
		}
		p.updateAllTiles()
	} else {
		p.updateTiles([]*tile{t})
	}
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
	settings := settingsType{
		"symbol":   t.symbol,
		"apikey":   t.apikey,
		"provider": t.provider,
		"view":     strconv.Itoa(t.view),
	}
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
		log.Println("OnTitleParametersDidChange: Tile not found")
		return
	}
	t.title = ev.Payload.Title
}

func (p plugin) OnPropertyInspectorConnected(ev *streamdeck.EvSendToPlugin) {
	if t, ok := p.tiles[ev.Context]; ok {
		settings := make(settingsType)
		settings["symbol"] = t.symbol
		settings["apikey"] = api.APIKey()
		settings["provider"] = t.provider
		settings["view"] = strconv.Itoa(t.view)
		p.sd.SendToPropertyInspector(ev.Action, ev.Context, &settings)
	}
}

func (p plugin) OnSendToPlugin(ev *streamdeck.EvSendToPlugin) {
	var payload map[string]*json.RawMessage
	err := json.Unmarshal(*ev.Payload, &payload)
	if err != nil {
		log.Println("OnSendToPlugin unmarshal", err)
	}
	data, ok := payload["sdpi_collection"]
	if !ok {
		return
	}

	sdpi := evSdpiCollection{}
	err = json.Unmarshal(*data, &sdpi)
	if err != nil {
		log.Println("SDPI unmarshal", err)
	}

	t := p.tiles[ev.Context]
	if t == nil {
		log.Printf("Tile was nil, creating new tile for %s\n", ev.Context)
		p.tiles[ev.Context] = &tile{context: ev.Context, provider: string(api.ProviderFinnhub), view: viewCurrent}
		t = p.tiles[ev.Context]
	}

	settings := make(settingsType)
	settings["apikey"] = api.APIKey()
	settings["symbol"] = t.symbol
	settings["provider"] = t.provider
	settings["view"] = strconv.Itoa(t.view)

	var updateAll bool
	switch sdpi.Key {
	case "symbol":
		symbol := strings.ToUpper(sdpi.Value)
		t.symbol = symbol
		settings["symbol"] = symbol
	case "apikey":
		apikey := sdpi.Value
		api.SetAPIKey(apikey)
		updateAll = true
		t.apikey = apikey
		settings["apikey"] = apikey
	case "provider":
		provider := normalizeProvider(sdpi.Value)
		t.provider = provider
		settings["provider"] = provider
	}

	if updateAll {
		apikey := api.APIKey()
		for _, tile := range p.tiles {
			tile.apikey = apikey
			err := p.sd.SetSettings(tile.context, settingsType{
				"symbol":   tile.symbol,
				"apikey":   apikey,
				"provider": tile.provider,
				"view":     strconv.Itoa(tile.view),
			})
			if err != nil {
				log.Printf("error setting settings: %v", err)
			}
		}
		p.updateAllTiles()
		return
	}

	log.Printf("saving settings: %v", settings)
	err = p.sd.SetSettings(ev.Context, &settings)
	if err != nil {
		log.Fatalf("setSettings: %v", err)
	}
	p.updateTiles([]*tile{t})
}

func (p plugin) OnApplicationDidLaunch(*streamdeck.EvApplication) {}

func (p plugin) OnApplicationDidTerminate(*streamdeck.EvApplication) {}
