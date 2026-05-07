import streamDeck, {
  type DidReceiveSettingsEvent,
  type KeyAction,
  type KeyDownEvent,
  type PropertyInspectorDidAppearEvent,
  SingletonAction,
  type TitleParametersDidChangeEvent,
  type WillAppearEvent
} from "@elgato/streamdeck";
import { getQuote, getSeries, type Provider } from "../api/providers";
import { buildTileSvg } from "../render/tiles";

export type StocksActionSettings = {
  symbol?: string;
  provider?: Provider;
  apiKey?: string;
  view?: number;
  displayTitle?: string;
};

const VIEW_CURRENT = 1;
const VIEW_DAY = 2;
const VIEW_MONTH = 3;
const VIEW_YEAR = 4;

const DEFAULT_SETTINGS: Required<StocksActionSettings> = {
  symbol: "",
  provider: "finnhub",
  apiKey: "",
  view: VIEW_CURRENT,
  displayTitle: ""
};

export class StocksAction extends SingletonAction<StocksActionSettings> {
  readonly manifestId = "com.exension.stocks.v2.symbol";

  constructor() {
    super();
    setInterval(() => {
      void this.refreshVisibleActions();
    }, 5 * 60 * 1000);
  }

  override async onWillAppear(ev: WillAppearEvent<StocksActionSettings>): Promise<void> {
    if (!ev.action.isKey()) {
      return;
    }
    const settings = normalizeSettings(ev.payload.settings);
    streamDeck.logger.info(
      `onWillAppear ${ev.action.id} settings=${JSON.stringify(settings)}`
    );
    await ev.action.setSettings(settings);
    await this.render(ev.action, settings);
  }

  override async onDidReceiveSettings(
    ev: DidReceiveSettingsEvent<StocksActionSettings>
  ): Promise<void> {
    if (!ev.action.isKey()) {
      return;
    }
    const settings = normalizeSettings(ev.payload.settings);
    streamDeck.logger.info(
      `onDidReceiveSettings ${ev.action.id} settings=${JSON.stringify(settings)}`
    );
    await this.render(ev.action, settings);
  }

  override async onKeyDown(ev: KeyDownEvent<StocksActionSettings>): Promise<void> {
    const settings = normalizeSettings(ev.payload.settings);
    const next = {
      ...settings,
      view: cycleView(settings.view)
    };

    await ev.action.setSettings(next);
    await this.render(ev.action, next);
  }

  override async onTitleParametersDidChange(
    ev: TitleParametersDidChangeEvent<StocksActionSettings>
  ): Promise<void> {
    if (!ev.action.isKey()) {
      return;
    }

    const current = normalizeSettings(await ev.action.getSettings<StocksActionSettings>());
    const title = "title" in ev.payload ? String((ev.payload as { title?: string }).title ?? "") : "";
    const next = normalizeSettings({
      ...current,
      displayTitle: title
    });

    await ev.action.setSettings(next);
    await this.render(ev.action, next);
  }

  override async onPropertyInspectorDidAppear(
    ev: PropertyInspectorDidAppearEvent<StocksActionSettings>
  ): Promise<void> {
    const settings = normalizeSettings(await ev.action.getSettings<StocksActionSettings>());
    streamDeck.logger.info(
      `onPropertyInspectorDidAppear ${ev.action.id} settings=${JSON.stringify(settings)}`
    );
    await streamDeck.ui.sendToPropertyInspector(settings);
  }

  private async render(
    actionRef: KeyAction<StocksActionSettings>,
    settings: StocksActionSettings
  ): Promise<void> {
    const normalized = normalizeSettings(settings);
    if (!normalized.symbol) {
      await actionRef.setTitle(undefined);
      await actionRef.setImage(undefined);
      return;
    }

    try {
      const quoteResult = await getQuote(normalized.symbol, normalized.provider, normalized.apiKey);
      const series =
        normalized.view === VIEW_CURRENT
          ? undefined
          : await getSeries(
              normalized.symbol,
              normalized.provider,
              toSeriesRange(normalized.view),
              quoteResult.quote,
              normalized.apiKey
            );

      const svg = buildTileSvg({
        symbol: normalized.displayTitle || normalized.symbol,
        provider: normalized.provider,
        view: normalized.view,
        quoteResult,
        series,
      });
      const image = `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;

      await actionRef.setTitle("");
      await actionRef.setImage(image);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      streamDeck.logger.error(`render failed for ${normalized.symbol}: ${message}`);
      await actionRef.setTitle("ERR");
      await actionRef.showAlert();
    }
  }

  private async refreshVisibleActions(): Promise<void> {
    for (const actionRef of this.actions) {
      if (!actionRef.isKey()) {
        continue;
      }
      const settings = normalizeSettings(await actionRef.getSettings<StocksActionSettings>());
      if (!settings.symbol) {
        continue;
      }
      await this.render(actionRef, settings);
    }
  }
}

function normalizeSettings(settings?: StocksActionSettings): Required<StocksActionSettings> {
  const input = settings ?? {};
  const provider = input.provider === "yahoo" ? "yahoo" : DEFAULT_SETTINGS.provider;
  const view = normalizeView(input.view);

  return {
    symbol: (input.symbol ?? DEFAULT_SETTINGS.symbol).trim().toUpperCase(),
    provider,
    apiKey: (input.apiKey ?? DEFAULT_SETTINGS.apiKey).trim(),
    view,
    displayTitle: (input.displayTitle ?? DEFAULT_SETTINGS.displayTitle).trim()
  };
}

function normalizeView(view?: number): number {
  switch (view) {
    case VIEW_DAY:
    case VIEW_MONTH:
    case VIEW_YEAR:
      return view;
    default:
      return VIEW_CURRENT;
  }
}

function cycleView(view: number): number {
  switch (view) {
    case VIEW_CURRENT:
      return VIEW_DAY;
    case VIEW_DAY:
      return VIEW_MONTH;
    case VIEW_MONTH:
      return VIEW_YEAR;
    default:
      return VIEW_CURRENT;
  }
}

function toSeriesRange(view: number): "day" | "month" | "year" {
  switch (view) {
    case VIEW_DAY:
      return "day";
    case VIEW_MONTH:
      return "month";
    case VIEW_YEAR:
      return "year";
    default:
      return "day";
  }
}
