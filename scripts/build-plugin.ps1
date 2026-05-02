param(
    [switch]$SkipBuild
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$pluginDir = Join-Path $repoRoot "com.exension.stocks.sdPlugin"
$windowsBinary = Join-Path $pluginDir "sdplugin-stocks.exe"
$macBinary = Join-Path $pluginDir "sdplugin-stocks"

if (-not $SkipBuild) {
    Push-Location $repoRoot
    try {
        go build -o $windowsBinary github.com/shayne/stock-ticker-stream-deck-plugin/cmd/stock_ticker_stream_deck_plugin
        $env:GOOS = "darwin"
        $env:GOARCH = "amd64"
        go build -o $macBinary github.com/shayne/stock-ticker-stream-deck-plugin/cmd/stock_ticker_stream_deck_plugin
    }
    finally {
        Remove-Item Env:GOOS -ErrorAction SilentlyContinue
        Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
        Pop-Location
    }
}

if (-not (Test-Path $windowsBinary)) {
    throw "Build output not found: $windowsBinary"
}

if (-not (Test-Path $macBinary)) {
    throw "Build output not found: $macBinary"
}

Write-Host "Plugin binaries ready at $windowsBinary and $macBinary"
