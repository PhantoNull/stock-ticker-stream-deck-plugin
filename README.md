# stock-ticker-stream-deck-plugin
A stock ticker plugin for Stream Deck

![StreamDeck_W1KAB8ful2](https://user-images.githubusercontent.com/46971999/60206582-cd1e8300-9808-11e9-9506-fe2e9bec466f.png)

![image](https://user-images.githubusercontent.com/46971999/60206645-f3dcb980-9808-11e9-9e2f-50ad67290a71.png)

![StreamDeck_AllnfL2xFA](https://user-images.githubusercontent.com/46971999/60206605-dad40880-9808-11e9-8eb3-12a58c1420c4.png)

## Modern local pipeline

This repository now supports the current Elgato CLI workflow on Windows.

### Requirements

- Go installed
- Node.js installed
- `@elgato/cli` available through `npm install` or `npm install -g @elgato/cli`
- Stream Deck desktop app installed

### Commands

```powershell
npm run build
npm run validate
npm run link
npm run pack
```

### Workflow

1. `npm run build` compiles `com.exension.stocks.sdPlugin\sdplugin-stocks.exe`
   and `com.exension.stocks.sdPlugin\sdplugin-stocks`
2. `npm run validate` checks the plugin manifest and assets with the Elgato CLI
3. `npm run link` links the plugin into the local Stream Deck Plugins directory for development
4. `npm run pack` creates a distributable `.streamDeckPlugin` package in `release`

### Notes

- The legacy `Makefile` and `DistributionTool` flow is retained for reference, but the supported local workflow is the Elgato CLI pipeline above.
- The plugin source lives in `com.exension.stocks.sdPlugin` and the Go entrypoint is `cmd/stock_ticker_stream_deck_plugin`.
