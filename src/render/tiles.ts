import type { Provider, QuoteResult, Series } from "../api/providers";

const VIEW_CURRENT = 1;
const VIEW_DAY = 2;
const VIEW_MONTH = 3;
const VIEW_YEAR = 4;

export function buildTileSvg(input: {
  symbol: string;
  provider: Provider;
  view: number;
  quoteResult: QuoteResult;
  series?: Series;
}): string {
  const symbol = input.symbol || "STOCKS";
  if (input.view === VIEW_CURRENT || !input.series || input.series.points.length < 2) {
    return buildCurrentTileSvg(symbol, input.quoteResult);
  }

  const rangeLabel = input.view === VIEW_DAY ? "D" : input.view === VIEW_MONTH ? "M" : "Y";
  return buildHistoryTileSvg(symbol, input.quoteResult.quote.price, input.series, rangeLabel);
}

function buildCurrentTileSvg(symbol: string, quoteResult: QuoteResult): string {
  const { quote, statusText } = quoteResult;
  const positive = quote.changePercent >= 0;
  const gradient = positive ? "#0E2013" : "#28070A";
  const changeColor = positive ? "#3E9E3E" : "#B51A28";
  const priceText = quote.price.toFixed(2);
  const priceFontSize = fitPriceFontSize(priceText);
  const changeText = `${quote.changePercent.toFixed(2)}%`;

  return `<svg width="72" height="72" viewBox="0 0 72 72" xmlns="http://www.w3.org/2000/svg">
<defs>
<linearGradient id="bg" x1="0" y1="0" x2="0" y2="1">
<stop offset="0%" stop-color="#000000"/>
<stop offset="100%" stop-color="${gradient}"/>
</linearGradient>
</defs>
<rect width="72" height="72" fill="url(#bg)"/>
${buildStatusIcon(statusText)}
${buildArrowIcon(quote.change, changeColor, 60, 45)}
<text x="4" y="19" font-size="14" font-weight="900" fill="#ffffff" font-family="Arial">${escapeSvg(symbol)}</text>
<rect x="5" y="29" width="11" height="2" fill="#666666"/>
<text x="4" y="50" font-size="${priceFontSize}" font-weight="700" fill="#ffffff" font-family="Arial">${escapeSvg(priceText)}</text>
<text x="4" y="65" font-size="11" font-weight="900" fill="${changeColor}" font-family="Arial">${escapeSvg(changeText)}</text>
</svg>`;
}

function buildHistoryTileSvg(symbol: string, price: number, series: Series, rangeLabel: string): string {
  const points = aggregateSeries(series.points, 10);
  const positive = series.changePercent >= 0;
  const stroke = positive ? "#34C759" : "#FF3B30";
  const fillAccent = positive ? "#275C35" : "#650212";
  const arrow = series.changePercent > 0 ? "▲" : series.changePercent < 0 ? "▼" : "■";

  const plotBottom = 86;
  const plotHeight = 22;
  const minValue = Math.min(...points);
  const maxValue = Math.max(...points);
  const valueRange = maxValue - minValue || 1;
  const stepX = 78 / Math.max(points.length - 1, 1);

  const linePoints: string[] = [];
  const areaPoints: string[] = [];
  let average = 0;

  points.forEach((point, index) => {
    average += point;
    const x = stepX * index;
    const y = plotBottom - ((point - minValue) / valueRange) * plotHeight;
    linePoints.push(`${x.toFixed(2)},${y.toFixed(2)}`);
    areaPoints.push(`${x.toFixed(2)},${y.toFixed(2)}`);
  });

  average /= points.length;
  const baselineY = plotBottom - ((average - minValue) / valueRange) * plotHeight;
  areaPoints.push("78,100", "0,100");
  const changeText = `${formatCompactPercent(Math.abs(series.changePercent))}%`;

  return `<svg width="100" height="100" viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
<defs>
<linearGradient id="grad" x1="0" y1="0" x2="0" y2="1">
<stop offset="0%" stop-color="#000000"/>
<stop offset="30%" stop-color="#000000"/>
<stop offset="100%" stop-color="${fillAccent}"/>
</linearGradient>
<linearGradient id="areaGradient" x1="0" y1="0" x2="0" y2="1">
<stop offset="0%" stop-color="${fillAccent}"/>
<stop offset="100%" stop-color="#000000"/>
</linearGradient>
</defs>
<rect width="100" height="100" fill="url(#grad)"/>
<line x1="0" y1="58" x2="0" y2="100" stroke="#ffffff" stroke-width="1" opacity="0.25"/>
<line x1="25" y1="58" x2="25" y2="100" stroke="#ffffff" stroke-width="1" opacity="0.25"/>
<line x1="50" y1="58" x2="50" y2="100" stroke="#ffffff" stroke-width="1" opacity="0.25"/>
<line x1="75" y1="58" x2="75" y2="100" stroke="#ffffff" stroke-width="1" opacity="0.25"/>
<line x1="100" y1="58" x2="100" y2="100" stroke="#ffffff" stroke-width="1" opacity="0.25"/>
<polygon points="${areaPoints.join(" ")}" fill="url(#areaGradient)" />
<polyline points="${linePoints.join(" ")}" fill="none" stroke="${stroke}" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" />
<line x1="0" y1="${baselineY.toFixed(2)}" x2="100" y2="${baselineY.toFixed(2)}" stroke="${stroke}" stroke-width="1.5" stroke-dasharray="6,2" opacity="0.55" />
<text x="6" y="20" font-size="17" font-weight="900" fill="#ffffff" font-family="Arial">${escapeSvg(symbol)}</text>
<text x="84" y="20" font-size="12" font-weight="900" fill="#ffffff" font-family="Arial">${escapeSvg(rangeLabel)}</text>
<text x="6" y="42" font-size="17" font-weight="700" fill="#ffffff" font-family="Arial">${escapeSvg(formatHistoryPrice(price))}</text>
<rect x="3" y="73" width="46" height="18" rx="4" ry="4" fill="#000000" fill-opacity="0.42"/>
<text x="6" y="88" font-size="17" font-weight="700" fill="${stroke}" font-family="Arial">${escapeSvg(changeText)}</text>
<text x="81" y="88" font-size="17" font-weight="900" fill="${stroke}" font-family="Arial">${escapeSvg(arrow)}</text>
</svg>`;
}

function buildStatusIcon(statusText: QuoteResult["statusText"]): string {
  switch (statusText) {
    case "OPEN":
      return `<g transform="translate(50 6)">
<circle cx="8" cy="8" r="8" fill="#000000" fill-opacity="0.38"/>
<g transform="translate(8 8)" stroke="#FF9A22" fill="#FF9A22" stroke-width="1.4">
<circle cx="0" cy="0" r="2.6"/>
<line x1="0" y1="-5" x2="0" y2="-3.7"/><line x1="0" y1="3.7" x2="0" y2="5"/>
<line x1="-5" y1="0" x2="-3.7" y2="0"/><line x1="3.7" y1="0" x2="5" y2="0"/>
<line x1="-3.5" y1="-3.5" x2="-2.6" y2="-2.6"/><line x1="3.5" y1="-3.5" x2="2.6" y2="-2.6"/>
<line x1="-3.5" y1="3.5" x2="-2.6" y2="2.6"/><line x1="3.5" y1="3.5" x2="2.6" y2="2.6"/>
</g>
</g>`;
    case "PRE":
      return `<g transform="translate(50 6)">
<circle cx="8" cy="8" r="8" fill="#000000" fill-opacity="0.38"/>
<g transform="translate(8 8)" stroke="#FF9A22" fill="none" stroke-width="1.5" stroke-linecap="round">
<circle cx="0" cy="0" r="4"/>
<line x1="0" y1="0" x2="0" y2="-2.8"/>
<line x1="0" y1="0" x2="2.6" y2="0"/>
</g>
</g>`;
    default:
      return `<g transform="translate(50 6)">
<circle cx="8" cy="8" r="8" fill="#000000" fill-opacity="0.38"/>
<g transform="translate(8 8)">
<circle cx="-1.2" cy="0" r="4.8" fill="#EAF6FF"/>
<circle cx="1.8" cy="0" r="4.8" fill="#000000" fill-opacity="0.38"/>
</g>
</g>`;
  }
}

function buildArrowIcon(change: number, color: string, x: number, y: number): string {
  if (change === 0) {
    return "";
  }

  if (change > 0) {
    return `<polygon points="${x},${y} ${x - 6},${y + 8} ${x + 6},${y + 8}" fill="${color}"/>`;
  }

  return `<polygon points="${x},${y + 8} ${x - 6},${y} ${x + 6},${y}" fill="${color}"/>`;
}

function fitPriceFontSize(priceText: string): number {
  if (priceText.length <= 6) {
    return 14;
  }
  if (priceText.length === 7) {
    return 12.5;
  }
  if (priceText.length === 8) {
    return 11;
  }
  return 9.5;
}

function formatHistoryPrice(price: number): string {
  return price >= 1000 ? price.toFixed(0) : price.toFixed(2);
}

function formatCompactPercent(value: number): string {
  return Math.abs(value) >= 10 ? value.toFixed(1) : value.toFixed(2);
}

function aggregateSeries(points: number[], target: number): number[] {
  if (points.length <= target || target < 2) {
    return [...points];
  }
  if (target === 2) {
    return [points[0], points[points.length - 1]];
  }

  const innerPoints = points.slice(1, -1);
  const innerTarget = target - 2;
  const result = [points[0]];

  if (innerTarget > 0 && innerPoints.length > 0) {
    const bucketSize = innerPoints.length / innerTarget;
    for (let index = 0; index < innerTarget; index += 1) {
      const start = Math.floor(index * bucketSize);
      let end = Math.floor((index + 1) * bucketSize);
      if (index === innerTarget - 1) {
        end = innerPoints.length;
      }
      if (end <= start) {
        end = start + 1;
      }
      const bucket = innerPoints.slice(start, end);
      const average = bucket.reduce((sum, point) => sum + point, 0) / bucket.length;
      result.push(average);
    }
  }

  result.push(points[points.length - 1]);
  return result;
}

function escapeSvg(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll("\"", "&quot;");
}
