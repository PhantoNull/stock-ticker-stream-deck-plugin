let websocket = null;
let uuid = null;
let actionInfo = {};

const fields = {
  symbol: null,
  provider: null,
  apikey: null,
  help: null,
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

  const info = JSON.parse(inInfo);
  addDynamicStyles(info.colors);
  cacheElements();
  bindEvents();

  websocket = new WebSocket(`ws://localhost:${inPort}`);
  websocket.onopen = function () {
    websocket.send(
      JSON.stringify({
        event: inRegisterEvent,
        uuid: inUUID,
      })
    );
    sendValueToPlugin("propertyInspectorConnected", "property_inspector");
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
  fields.symbol.addEventListener("change", function () {
    sendSetting("symbol", fields.symbol.value.trim().toUpperCase());
    fields.symbol.value = fields.symbol.value.trim().toUpperCase();
  });

  fields.provider.addEventListener("change", function () {
    sendSetting("provider", fields.provider.value);
    syncProviderUI(fields.provider.value);
  });

  fields.apikey.addEventListener("change", function () {
    sendSetting("apikey", fields.apikey.value.trim());
  });
}

function applySettings(payload) {
  if (typeof payload.symbol === "string") {
    fields.symbol.value = payload.symbol;
  }
  if (typeof payload.provider === "string") {
    fields.provider.value = payload.provider;
  }
  if (typeof payload.apikey === "string") {
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

function sendSetting(key, value) {
  sendValueToPlugin(
    {
      key: key,
      value: value,
      group: false,
      index: 0,
      selection: [],
      checked: false,
    },
    "sdpi_collection"
  );
}

function sendValueToPlugin(value, param) {
  if (!websocket || websocket.readyState !== WebSocket.OPEN) {
    return;
  }

  websocket.send(
    JSON.stringify({
      action: actionInfo.action,
      event: "sendToPlugin",
      context: uuid,
      payload: {
        [param]: value,
      },
    })
  );
}

window.addEventListener("beforeunload", function (event) {
  event.preventDefault();
  sendValueToPlugin("propertyInspectorWillDisappear", "property_inspector");
});

function addDynamicStyles(colors) {
  const style = document.createElement("style");
  style.id = "sdpi-dynamic-styles";
  style.textContent = `
    td.selected,
    td.selected:hover,
    li.selected:hover,
    li.selected {
      color: white;
      background-color: ${colors.highlightColor};
    }
  `;
  document.head.appendChild(style);
}
