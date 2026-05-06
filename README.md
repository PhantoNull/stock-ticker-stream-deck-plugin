# stock-ticker-stream-deck-plugin

Stream Deck plugin that renders stock quotes on key tiles, with provider selection and multi-view charts.

## Current capabilities

- provider per key: `Finnhub` or `Yahoo`
- tap-to-cycle views on the device:
  - `1` quote
  - `2` daily chart
  - `3` monthly chart
  - `4` yearly chart
- auto-refresh for configured tiles
- local development flow based on the current Elgato CLI

## Requirements

- Go
- Node.js
- Stream Deck desktop app
- `@elgato/cli` installed locally or globally

## Development workflow

```powershell
npm install
npm run build
npm run validate
npm run link
```

Package a distributable plugin:

```powershell
npm run pack
```

## Notes

- `Yahoo` does not require an API key.
- `Finnhub` requires an API key and some historical endpoints may depend on the account tier.
- The supported local workflow is the Elgato CLI pipeline above; the legacy `Makefile` flow is retained only for reference.

## Repository layout

- [cmd/stock_ticker_stream_deck_plugin](./cmd/stock_ticker_stream_deck_plugin): plugin runtime and rendering
- [pkg/api](./pkg/api): provider integrations and history fetching
- [com.exension.stocks.sdPlugin](./com.exension.stocks.sdPlugin): Stream Deck manifest, assets, and property inspector
- [scripts/build-plugin.ps1](./scripts/build-plugin.ps1): local build entrypoint

## Known follow-ups

- align `D/M/Y` percentage anchors with stricter financial semantics
- add optional currency conversion for EUR-based views
- evaluate migration away from the vendored SDK if a maintained Go SDK becomes available
