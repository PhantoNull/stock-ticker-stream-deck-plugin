let websocket = null;
let uuid = null;
let actionInfo = {};
let currentSettings = {};

const fields = {
  symbol: null,
  provider: null,
  apikey: null,
  help: null,
};
const saveTimers = {
  symbol: null,
  apikey: null,
};

function connectElgatoStreamDeckSocket(
  inPort,
  inUUID,
  inRegisterEvent,
  inInfo,
  inActionInfo
) {
  uuid = inUUID;
  actionInfo = JSON.parse(inActionInfo);

  JSON.parse(inInfo);
  cacheElements();
  bindEvents();
  applySettings(actionInfo && actionInfo.payload ? actionInfo.payload.settings || {} : {});

  websocket = new WebSocket(`ws://localhost:${inPort}`);
  websocket.onopen = function () {
    websocket.send(
      JSON.stringify({
        event: inRegisterEvent,
        uuid: inUUID,
      })
    );
  };

  websocket.onmessage = function (evt) {
    const message = JSON.parse(evt.data);
    if (message.event !== "sendToPropertyInspector" || !message.payload) {
      return;
    }

    applySettings(message.payload);
  };
}

function cacheElements() {
  fields.symbol = document.querySelector("#symbol input");
  fields.provider = document.querySelector("#provider select");
  fields.apikey = document.querySelector("#apikey input");
  fields.help = document.getElementById("provider-help");
  document.querySelector(".sdpi-wrapper").classList.remove("hidden");
}

function bindEvents() {
  fields.symbol.addEventListener("input", function () {
    const next = fields.symbol.value.trim().toUpperCase();
    fields.symbol.value = next;
    queueSetting("symbol", next, "symbol");
  });

  fields.provider.addEventListener("change", function () {
    sendSetting("provider", fields.provider.value);
    syncProviderUI(fields.provider.value);
  });

  fields.apikey.addEventListener("input", function () {
    const next = fields.apikey.value.trim();
    fields.apikey.value = next;
    queueSetting("apikey", next, "apikey");
  });
}

function applySettings(payload) {
  currentSettings = {
    ...currentSettings,
    ...payload,
  };

  if (typeof payload.symbol === "string") {
    fields.symbol.value = payload.symbol;
  }
  if (typeof payload.provider === "string") {
    fields.provider.value = payload.provider;
  }
  if (typeof payload.apiKey === "string") {
    fields.apikey.value = payload.apiKey;
  } else if (typeof payload.apikey === "string") {
    fields.apikey.value = payload.apikey;
  }

  syncProviderUI(fields.provider.value);
}

function syncProviderUI(provider) {
  const isYahoo = provider === "yahoo";
  fields.apikey.disabled = isYahoo;
  fields.apikey.placeholder = isYahoo ? "Not required for Yahoo" : "Enter Finnhub API key";
  fields.help.textContent = isYahoo
    ? "Yahoo does not require an API key."
    : "Finnhub requires an API key.";
}

function queueSetting(key, value, timerKey) {
  if (saveTimers[timerKey]) {
    clearTimeout(saveTimers[timerKey]);
  }

  saveTimers[timerKey] = setTimeout(function () {
    sendSetting(key, value);
    saveTimers[timerKey] = null;
  }, 250);
}

function sendSetting(key, value) {
  const nextSettings = {
    ...currentSettings,
    [key === "apikey" ? "apiKey" : key]: value,
  };

  currentSettings = nextSettings;

  if (!websocket || websocket.readyState !== WebSocket.OPEN) {
    return;
  }

  websocket.send(
    JSON.stringify({
      event: "setSettings",
      context: uuid,
      payload: nextSettings,
    })
  );
}
