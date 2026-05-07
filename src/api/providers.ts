export type Provider = "finnhub" | "yahoo";

export type SeriesRange = "day" | "month" | "year";

export type Quote = {
  price: number;
  change: number;
  changePercent: number;
};

export type QuoteResult = {
  quote: Quote;
  statusText: "OPEN" | "PRE" | "POST";
};

export type Series = {
  points: number[];
  basePrice: number;
  changePercent: number;
};

type CachedValue<T> = {
  expiration: number;
  value: T;
};

type YahooChartResponse = {
  chart?: {
    result?: Array<{
      meta?: {
        regularMarketPrice?: number;
        previousClose?: number;
        chartPreviousClose?: number;
        currentTradingPeriod?: {
          regular?: {
            start?: number;
            end?: number;
          };
        };
      };
      indicators?: {
        quote?: Array<{
          close?: Array<number | null>;
          open?: Array<number | null>;
        }>;
      };
    }>;
  };
};

type FinnhubQuoteResponse = {
  c?: number;
  d?: number;
  dp?: number;
};

type FinnhubMarketStatusResponse = {
  session?: string;
};

type FinnhubCandlesResponse = {
  c?: number[];
};

const DEFAULT_HTTP_TIMEOUT_MS = 10_000;

const seriesCache = new Map<string, CachedValue<Series>>();
let finnhubMarketStatusCache: CachedValue<FinnhubMarketStatusResponse> | undefined;

export async function getQuote(
  symbol: string,
  provider: Provider,
  apiKey?: string
): Promise<QuoteResult> {
  if (provider === "yahoo") {
    return getYahooQuote(symbol);
  }

  return getFinnhubQuote(symbol, apiKey);
}

export async function getSeries(
  symbol: string,
  provider: Provider,
  range: SeriesRange,
  quote: Quote,
  apiKey?: string
): Promise<Series> {
  const cacheKey = `${provider}:${symbol}:${range}`;
  const cached = seriesCache.get(cacheKey);
  if (cached && cached.expiration > Date.now()) {
    const next = cloneSeries(cached.value);
    if (next.points.length > 0) {
      next.points[next.points.length - 1] = quote.price;
    }
    if (next.basePrice !== 0) {
      next.changePercent = ((quote.price - next.basePrice) / next.basePrice) * 100;
    }
    return next;
  }

  const fetched =
    provider === "yahoo"
      ? await getYahooSeries(symbol, range, quote)
      : await getFinnhubSeries(symbol, range, quote, apiKey);

  seriesCache.set(cacheKey, {
    expiration: Date.now() + fetched.expirationMs,
    value: fetched.series,
  });

  return cloneSeries(fetched.series);
}

async function getYahooQuote(symbol: string): Promise<QuoteResult> {
  const chart = await fetchYahooChart(symbol, "1d", "5d");
  const result = chart.chart?.result?.[0];
  const meta = result?.meta;

  if (!meta?.regularMarketPrice) {
    throw new Error(`Missing Yahoo quote for ${symbol}`);
  }

  const price = meta.regularMarketPrice;
  const previousClose = meta.previousClose ?? meta.chartPreviousClose ?? 0;
  const change = price - previousClose;
  const changePercent = previousClose !== 0 ? (change / previousClose) * 100 : 0;
  const now = Math.floor(Date.now() / 1000);
  const regularStart = meta.currentTradingPeriod?.regular?.start ?? 0;
  const regularEnd = meta.currentTradingPeriod?.regular?.end ?? 0;

  let statusText: QuoteResult["statusText"] = "POST";
  if (now >= regularStart && now <= regularEnd) {
    statusText = "OPEN";
  } else if (now < regularStart) {
    statusText = "PRE";
  }

  return {
    quote: {
      price,
      change,
      changePercent,
    },
    statusText,
  };
}

async function getYahooSeries(
  symbol: string,
  range: SeriesRange,
  quote: Quote
): Promise<{ series: Series; expirationMs: number }> {
  let interval = "1d";
  let rangeParam = "1mo";
  let expirationMs = 60 * 60 * 1000;

  switch (range) {
    case "day":
      interval = "5m";
      rangeParam = "1d";
      expirationMs = 5 * 60 * 1000;
      break;
    case "month":
      interval = "1d";
      rangeParam = "1mo";
      break;
    case "year":
      interval = "1d";
      rangeParam = "1y";
      expirationMs = 6 * 60 * 60 * 1000;
      break;
  }

  const chart = await fetchYahooChart(symbol, interval, rangeParam);
  const quoteSeries = chart.chart?.result?.[0]?.indicators?.quote?.[0];
  const points = (quoteSeries?.close ?? [])
    .filter((value): value is number => typeof value === "number")
    .map((value) => Number(value));

  if (points.length === 0) {
    throw new Error(`Missing Yahoo series for ${symbol}`);
  }

  points[points.length - 1] = quote.price;
  let basePrice = points[0];
  if (range === "day") {
    basePrice = quote.price - quote.change;
  }
  if (basePrice === 0) {
    basePrice = points[0];
  }

  return {
    expirationMs,
    series: {
      points,
      basePrice,
      changePercent: ((quote.price - basePrice) / basePrice) * 100,
    },
  };
}

async function getFinnhubQuote(symbol: string, apiKey?: string): Promise<QuoteResult> {
  const token = requireApiKey(apiKey, "Finnhub quote");
  const [quoteData, marketStatus] = await Promise.all([
    fetchJson<FinnhubQuoteResponse>(withQuery("https://finnhub.io/api/v1/quote", { symbol, token })),
    getFinnhubMarketStatus(token),
  ]);

  const price = quoteData.c ?? 0;
  const change = quoteData.d ?? 0;
  const changePercent = quoteData.dp ?? 0;
  let statusText: QuoteResult["statusText"] = "POST";

  switch (marketStatus.session) {
    case "regular":
      statusText = "OPEN";
      break;
    case "pre-market":
      statusText = "PRE";
      break;
    default:
      statusText = "POST";
      break;
  }

  return {
    quote: {
      price,
      change,
      changePercent,
    },
    statusText,
  };
}

async function getFinnhubSeries(
  symbol: string,
  range: SeriesRange,
  quote: Quote,
  apiKey?: string
): Promise<{ series: Series; expirationMs: number }> {
  const token = requireApiKey(apiKey, "Finnhub series");
  const now = Math.floor(Date.now() / 1000);

  let resolution = "D";
  let from = now - 30 * 24 * 60 * 60;
  let expirationMs = 30 * 60 * 1000;

  switch (range) {
    case "day":
      resolution = "5";
      from = now - 24 * 60 * 60;
      expirationMs = 5 * 60 * 1000;
      break;
    case "month":
      resolution = "D";
      from = now - 31 * 24 * 60 * 60;
      expirationMs = 60 * 60 * 1000;
      break;
    case "year":
      resolution = "D";
      from = now - 366 * 24 * 60 * 60;
      expirationMs = 6 * 60 * 60 * 1000;
      break;
  }

  const url = withQuery("https://finnhub.io/api/v1/stock/candle", {
    symbol,
    resolution,
    from: String(from),
    to: String(now),
    token,
  });
  const payload = await fetchJson<FinnhubCandlesResponse>(url);
  const points = (payload.c ?? []).map((value) => Number(value));
  if (points.length === 0) {
    throw new Error(`Missing Finnhub series for ${symbol}`);
  }

  points[points.length - 1] = quote.price;
  let basePrice = points[0];
  if (range === "day") {
    basePrice = quote.price - quote.change;
  }
  if (basePrice === 0) {
    basePrice = points[0];
  }

  return {
    expirationMs,
    series: {
      points,
      basePrice,
      changePercent: ((quote.price - basePrice) / basePrice) * 100,
    },
  };
}

async function getFinnhubMarketStatus(token: string): Promise<FinnhubMarketStatusResponse> {
  if (finnhubMarketStatusCache && finnhubMarketStatusCache.expiration > Date.now()) {
    return finnhubMarketStatusCache.value;
  }

  const value = await fetchJson<FinnhubMarketStatusResponse>(
    withQuery("https://finnhub.io/api/v1/stock/market-status", {
      exchange: "US",
      token,
    })
  );
  finnhubMarketStatusCache = {
    expiration: Date.now() + 5 * 60 * 1000,
    value,
  };
  return value;
}

async function fetchYahooChart(
  symbol: string,
  interval: string,
  rangeParam: string
): Promise<YahooChartResponse> {
  const url = withQuery(`https://query1.finance.yahoo.com/v8/finance/chart/${encodeURIComponent(symbol)}`, {
    interval,
    range: rangeParam,
    includePrePost: "true",
  });
  return fetchJson<YahooChartResponse>(url, {
    "User-Agent": "stock-ticker-stream-deck-plugin-v2/0.0.1",
  });
}

async function fetchJson<T>(url: string, headers?: Record<string, string>): Promise<T> {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), DEFAULT_HTTP_TIMEOUT_MS);

  try {
    const response = await fetch(url, {
      headers,
      signal: controller.signal,
    });
    if (!response.ok) {
      throw new Error(`${response.status} ${response.statusText}`);
    }
    return (await response.json()) as T;
  } finally {
    clearTimeout(timeout);
  }
}

function withQuery(baseUrl: string, query: Record<string, string>): string {
  const url = new URL(baseUrl);
  for (const [key, value] of Object.entries(query)) {
    url.searchParams.set(key, value);
  }
  return url.toString();
}

function requireApiKey(apiKey: string | undefined, operation: string): string {
  const token = apiKey?.trim() ?? "";
  if (token.length === 0) {
    throw new Error(`${operation} requires an API key`);
  }
  return token;
}

function cloneSeries(series: Series): Series {
  return {
    points: [...series.points],
    basePrice: series.basePrice,
    changePercent: series.changePercent,
  };
}
