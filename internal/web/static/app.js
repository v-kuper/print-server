const toastRootEl = document.querySelector("#toast-root");
const previewEl = document.querySelector("#receipt-preview");
const fontMetricsEl = document.querySelector("#font-metrics");
const schedulerStatusEl = document.querySelector("#scheduler-status");
const motivationStatusEl = document.querySelector("#motivation-status");
const googleStatusEl = document.querySelector("#google-status");
const googleCredentialsPathEl = document.querySelector("#google-credentials-path");
const googleTokenPathEl = document.querySelector("#google-token-path");
const newsSourceListEl = document.querySelector("#news-source-list");
const denisTrendsSettingsEl = document.querySelector("#denis-trends-settings");
const weatherNameInput = document.querySelector("#weather-name");
const weatherLatitudeInput = document.querySelector("#weather-latitude");
const weatherLongitudeInput = document.querySelector("#weather-longitude");
const weatherLocationResultsEl = document.querySelector("#weather-location-results");
const weatherLocationHelpEl = document.querySelector("#weather-location-help");
const imageEditorFileInput = document.querySelector("#image-editor-file");
const imageEditorCanvasHeightInput = document.querySelector("#image-editor-canvas-height");
const imageEditorSourceCanvas = document.querySelector("#image-editor-source");
const imageEditorResultCanvas = document.querySelector("#image-editor-result");
const imageEditorStatusEl = document.querySelector("#image-editor-status");
const textPrintPreviewEl = document.querySelector("#text-print-preview");
let lastPreviewLines = [];
let weatherSearchTimer = null;
let financeDraft = {
  amountTon: document.querySelector("#ton-amount").value,
  investedUsd: document.querySelector("#ton-invested").value
};
const imageEditorWidth = 384;
let imageEditorSourceImage = null;
let imageEditorSourceObjectURL = "";
let imageEditorPixels = null;
let imageEditorPointerDown = false;
let imageEditorShapeStart = null;
let imageEditorShapeBasePixels = null;
let imageEditorLastSavedState = null;
const assetVersion = String(Date.now());
const fallbackFontMetrics = [
  { font: 0, lineLength: 32, fontWidth: 12 },
  { font: 1, lineLength: 42, fontWidth: 9 },
  { font: 2, lineLength: 38, fontWidth: 10 },
  { font: 3, lineLength: 32, fontWidth: 12 },
  { font: 4, lineLength: 32, fontWidth: 12 },
  { font: 5, lineLength: 32, fontWidth: 12 },
  { font: 6, lineLength: 32, fontWidth: 12 },
  { font: 7, lineLength: 32, fontWidth: 12 },
  { font: 8, lineLength: 32, fontWidth: 12 },
  { font: 9, lineLength: 32, fontWidth: 12 }
];
let fontMetrics = new Map(fallbackFontMetrics.map(metric => [metric.font, metric]));
let activeLoadingToast = null;
const scheduleContentOptions = [
  { key: "showWeather",         label: "Погода",          group: "Основное" },
  { key: "showWeatherAdvice",   label: "AI-совет",        group: "Основное" },
  { key: "showMotivationQuote", label: "Цитата",          group: "Основное" },
  { key: "showTonPortfolio",    label: "TON",             group: "Финансы"  },
  { key: "showUsdBynRate",      label: "USD/BYN",         group: "Финансы"  },
  { key: "showBankRates",       label: "Банки",           group: "Финансы"  },
  { key: "showMail",            label: "Почта",           group: "Google"   },
  { key: "showCalendar",        label: "Календарь",       group: "Google"   },
  { key: "showHistory",         label: "История дня",     group: "Новости"  },
  { key: "showNews",            label: "Коротко о мире",  group: "Новости"  },
  { key: "showDenisTrends",     label: "Denis Trends",    group: "Новости"  },
];
const denisTrendPeriods = [
  { key: "now", label: "Top now" },
  { key: "day", label: "Top day" },
  { key: "week", label: "Top week" },
  { key: "month", label: "Top month" }
];
const denisTrendModeOptions = [
  { key: "auto", label: "Авто" },
  { key: "now", label: "Top now" },
  { key: "day", label: "Top day" }
];


async function loadBootstrap() {
  const response = await fetch("/api/bootstrap");
  const payload = await response.json();
  if (!response.ok || !payload.ok) {
    throw new Error(payload.error || "Не удалось загрузить начальные настройки.");
  }
  return payload.data || {};
}

function valueOrEmpty(value) {
  if (value === null || value === undefined) {
    return "";
  }
  return String(value);
}

function setValue(selector, value) {
  const input = document.querySelector(selector);
  if (input) {
    input.value = valueOrEmpty(value);
  }
}

function setChecked(selector, checked) {
  const input = document.querySelector(selector);
  if (input) {
    input.checked = Boolean(checked);
  }
}

function setScheduleMode(mode) {
  const radio = document.querySelector('input[name="schedule-mode"][value="' + (mode || "interval") + '"]');
  if (radio) {
    radio.checked = true;
  }
}

function renderSelectOptions(selector, options, selectedValue) {
  const select = document.querySelector(selector);
  if (!select) {
    return;
  }
  select.replaceChildren(...options.map(option => {
    const node = document.createElement("option");
    node.value = String(option.value);
    node.textContent = option.label;
    return node;
  }));
  select.value = valueOrEmpty(selectedValue);
}

function renderFontSelectOptions(fontOptions) {
  const options = (fontOptions || []).map(font => ({ value: font, label: "Шрифт " + font }));
  document.querySelectorAll("[data-font-select]").forEach(select => {
    const selected = select.value;
    select.replaceChildren(...options.map(option => {
      const node = document.createElement("option");
      node.value = String(option.value);
      node.textContent = option.label;
      return node;
    }));
    select.value = selected;
  });
}

function renderScheduleIntervals(intervals, selectedMinutes) {
  renderSelectOptions("#schedule-interval", (intervals || []).map(option => ({
    value: option.minutes,
    label: option.label
  })), selectedMinutes);
}

function scheduleRunsFromBootstrap(times, runs) {
  if (Array.isArray(runs) && runs.length > 0) {
    return runs.map(run => ({
      time: valueOrEmpty(run.time),
      content: run.content || null
    }));
  }
  const values = Array.isArray(times) && times.length > 0 ? times : [""];
  return values.map(time => ({ time, content: null }));
}

function renderScheduleTimes(times, runs) {
  const list = document.querySelector("#schedule-time-list");
  if (!list) {
    return;
  }
  list.replaceChildren();
  const values = scheduleRunsFromBootstrap(times, runs);
  values.forEach(run => addScheduleTime(run));
}

function renderScheduleContentGroups(container, content, keyAttribute) {
  if (!container) {
    return;
  }
  const customContent = scheduleContentFromSettings(content);
  const groups = {};
  const groupOrder = [];
  for (const option of scheduleContentOptions) {
    if (!groups[option.group]) {
      groups[option.group] = [];
      groupOrder.push(option.group);
    }
    groups[option.group].push(option);
  }
  container.replaceChildren();
  for (const groupName of groupOrder) {
    const col = document.createElement("div");
    col.className = "content-group";
    const title = document.createElement("div");
    title.className = "content-group-title";
    title.textContent = groupName;
    col.appendChild(title);
    for (const option of groups[groupName]) {
      const label = document.createElement("label");
      label.className = "toggle-label";
      const checkbox = document.createElement("input");
      checkbox.type = "checkbox";
      checkbox.dataset[keyAttribute] = option.key;
      checkbox.checked = Boolean(customContent[option.key]);
      label.append(checkbox, document.createTextNode(option.label));
      col.appendChild(label);
      if (option.key === "showDenisTrends") {
        col.appendChild(renderDenisTrendsModeControl(customContent.denisTrendsMode, keyAttribute));
      }
    }
    container.appendChild(col);
  }
}

function renderDenisTrendsModeControl(value, keyAttribute) {
  const label = document.createElement("label");
  label.className = "content-mode-label";
  label.textContent = "Режим";
  const select = document.createElement("select");
  select.dataset.denisTrendsMode = "";
  if (keyAttribute === "intervalContentKey") {
    select.dataset.intervalDenisTrendsMode = "";
  } else {
    select.dataset.scheduleDenisTrendsMode = "";
  }
  for (const option of denisTrendModeOptions) {
    const item = document.createElement("option");
    item.value = option.key;
    item.textContent = option.label;
    select.appendChild(item);
  }
  select.value = denisTrendsMode(value);
  label.appendChild(select);
  return label;
}

function renderIntervalContent(content) {
  renderScheduleContentGroups(document.querySelector("#schedule-interval-content"), content, "intervalContentKey");
}

function renderNewsSources(newsSettings, presets) {
  if (!newsSourceListEl) {
    return;
  }
  const presetNames = new Map((presets || []).map(preset => [preset.preset, preset.displayName || preset.preset]));
  newsSourceListEl.replaceChildren();
  for (const source of (newsSettings && newsSettings.sources) || []) {
    const row = document.createElement("div");
    row.className = "news-source";
    row.dataset.newsSource = "";

    const enabled = document.createElement("input");
    enabled.type = "checkbox";
    enabled.dataset.newsEnabled = "";
    enabled.checked = Boolean(source.enabled);

    const preset = document.createElement("input");
    preset.type = "hidden";
    preset.dataset.newsPreset = "";
    preset.value = source.preset || "";

    const url = document.createElement("input");
    url.type = "hidden";
    url.dataset.newsUrl = "";
    url.value = source.feedUrl || "";

    const urlLabel = document.createElement("label");
    urlLabel.textContent = presetNames.get(source.preset) || source.preset || "RSS";
    const visibleURL = document.createElement("input");
    visibleURL.dataset.newsUrlVisible = "";
    visibleURL.autocomplete = "off";
    visibleURL.value = source.feedUrl || "";
    urlLabel.appendChild(visibleURL);

    const countLabel = document.createElement("label");
    countLabel.textContent = "Кол-во";
    const count = document.createElement("input");
    count.dataset.newsCount = "";
    count.autocomplete = "off";
    count.inputMode = "numeric";
    count.min = "1";
    count.max = "100";
    count.value = valueOrEmpty(source.maxItems || 1);
    countLabel.appendChild(count);

    row.append(enabled, preset, url, urlLabel, countLabel);
    newsSourceListEl.appendChild(row);
  }
}

function renderDenisTrendsSettings(settings) {
  if (!denisTrendsSettingsEl) {
    return;
  }
  const normalized = settings || {};
  const periods = normalized.periods || {};
  denisTrendsSettingsEl.replaceChildren();

  const periodTitle = document.createElement("div");
  periodTitle.className = "content-group-title";
  periodTitle.textContent = "Периоды";
  denisTrendsSettingsEl.appendChild(periodTitle);

  const hint = document.createElement("p");
  hint.className = "helper-text";
  hint.textContent = "Denis Trends: режим Top now / Top day выбирается в составе чека и расписании. Авто оставляет правило до 12:00 Top now, после 12:00 Top day.";
  denisTrendsSettingsEl.appendChild(hint);

  denisTrendPeriods.forEach(period => {
    const value = periods[period.key] || {};
    const row = document.createElement("div");
    row.className = "news-source";
    row.dataset.denisTrendPeriod = period.key;

    const enabled = document.createElement("input");
    enabled.type = "checkbox";
    enabled.dataset.denisTrendPeriodEnabled = "";
    enabled.checked = Boolean(value.enabled);

    const label = document.createElement("span");
    label.textContent = period.label;

    const countLabel = document.createElement("label");
    countLabel.textContent = "Кол-во";
    const count = document.createElement("input");
    count.dataset.denisTrendPeriodCount = "";
    count.autocomplete = "off";
    count.inputMode = "numeric";
    count.min = "1";
    count.max = "100";
    count.value = valueOrEmpty(value.maxItems || 20);
    countLabel.appendChild(count);

    row.append(enabled, label, countLabel);
    denisTrendsSettingsEl.appendChild(row);
  });
}

function renderMotivationInitialStatus(settings) {
  if (!settings) {
    setMotivationStatus("", "Модель будет использоваться для включенных AI-блоков в составе чека.");
    return;
  }
  if (settings.lastError) {
    setMotivationStatus("", "Ошибка: " + settings.lastError);
    return;
  }
  if (settings.cachedQuote) {
    setMotivationStatus("", "Последняя проверка: " + settings.cachedQuote);
    return;
  }
  setMotivationStatus("", "Модель будет использоваться для включенных AI-блоков в составе чека.");
}

function applyBootstrap(data) {
  const printer = data.printer || {};
  setValue("#host", printer.host);
  setValue("#port", printer.port);

  const content = data.receiptContent || {};
  setChecked("#content-weather", content.showWeather);
  setChecked("#content-weather-advice", content.showWeatherAdvice);
  setChecked("#content-motivation-quote", content.showMotivationQuote);
  setChecked("#content-ton-portfolio", content.showTonPortfolio);
  setChecked("#content-usd-byn-rate", content.showUsdBynRate);
  setChecked("#content-bank-rates", content.showBankRates);
  setChecked("#content-mail", content.showMail);
  setChecked("#content-calendar", content.showCalendar);
  setChecked("#content-history", content.showHistory);
  setChecked("#content-news", content.showNews);
  setChecked("#content-denis-trends", content.showDenisTrends);
  setValue("#content-denis-trends-mode", denisTrendsMode(content.denisTrendsMode));

  const weather = data.weather || {};
  setValue("#weather-name", weather.name);
  setValue("#weather-latitude", weather.latitude);
  setValue("#weather-longitude", weather.longitude);
  setWeatherLocationSelected(true);

  const motivation = data.motivation || {};
  setValue("#motivation-base-url", motivation.baseUrl);
  setValue("#motivation-model", motivation.model);
  renderMotivationInitialStatus(motivation);

  const finance = data.finance || {};
  setValue("#ton-amount", finance.amountTon);
  setValue("#ton-invested", finance.investedUsd);
  financeDraft = {
    amountTon: document.querySelector("#ton-amount").value,
    investedUsd: document.querySelector("#ton-invested").value
  };
  setFinanceEditing(false);

  const style = data.receiptStyle || {};
  renderFontSelectOptions(data.fontOptions || []);
  setValue("#calendar-font", style.calendarFont || 0);
  setChecked("#calendar-double-width", style.calendarDoubleWidth);
  setChecked("#calendar-double-height", style.calendarDoubleHeight);
  setFontRowAlign("calendar", style.calendarAlignment || "center");
  setValue("#temperature-font", style.temperatureFont || 0);
  setChecked("#temperature-double-width", style.temperatureDoubleWidth);
  setChecked("#temperature-double-height", style.temperatureDoubleHeight);
  setFontRowAlign("temperature", style.temperatureAlignment || "center");
  setValue("#normal-font", style.normalFont || 0);
  setFontRowAlign("normal", style.normalAlignment || "center");
  renderAllFontRowPreviews();
  initTextPrintBlocks();

  const news = data.news || {};
  setChecked("#news-translate", news.translateTitles);
  renderNewsSources(news, data.newsPresets || []);
  renderDenisTrendsSettings(data.denisTrends || {});

  const receiptSnapshot = data.receiptSnapshot || {};
  setValue("#receipt-snapshot-base-url", receiptSnapshot.baseUrl || defaultReceiptSnapshotBaseURL());

  const schedule = data.schedule || {};
  setScheduleMode(schedule.mode);
  renderScheduleIntervals(data.scheduleIntervals || [], schedule.intervalMinutes);
  renderIntervalContent(schedule.intervalContent || content);
  renderScheduleTimes(schedule.times || [], schedule.runs || []);

  renderGoogleStatus(data.googleStatus || {});
  updateTextPrintPreview();
}

function setBusy(busy) {
  document.querySelectorAll("button").forEach(button => button.disabled = busy);
}

function toastKind(kind, text) {
  if (kind === "ok" || kind === "error") {
    return kind;
  }
  return String(text || "").trim().endsWith("...") ? "loading" : "info";
}

function removeToast(toast) {
  if (!toast) {
    return;
  }
  if (toast === activeLoadingToast) {
    activeLoadingToast = null;
  }
  toast.remove();
}

function showToast(kind, text) {
  if (!toastRootEl) {
    return;
  }
  const message = String(text || "").trim();
  if (!message) {
    removeToast(activeLoadingToast);
    return;
  }

  const normalizedKind = toastKind(kind, message);
  if (normalizedKind === "loading" && activeLoadingToast) {
    const currentMessage = activeLoadingToast.querySelector(".toast-message");
    if (currentMessage) {
      currentMessage.textContent = message;
    }
    return;
  }

  if (normalizedKind !== "loading") {
    removeToast(activeLoadingToast);
  }

  const toast = document.createElement("div");
  toast.className = "toast-notification " + normalizedKind;
  toast.setAttribute("role", normalizedKind === "error" ? "alert" : "status");

  const icon = document.createElement("span");
  icon.className = "toast-icon";
  icon.setAttribute("aria-hidden", "true");

  const messageEl = document.createElement("div");
  messageEl.className = "toast-message";
  messageEl.textContent = message;

  const close = document.createElement("button");
  close.type = "button";
  close.className = "toast-close";
  close.setAttribute("aria-label", "Закрыть уведомление");
  close.textContent = "x";
  close.addEventListener("click", () => removeToast(toast));

  toast.append(icon, messageEl, close);
  toastRootEl.prepend(toast);

  if (normalizedKind === "loading") {
    activeLoadingToast = toast;
    return;
  }

  window.setTimeout(() => removeToast(toast), normalizedKind === "error" ? 7000 : 4200);
}

function setStatus(kind, text) {
  showToast(kind, text);
}

function setWeatherLocationSelected(selected) {
  weatherNameInput.dataset.weatherLocationSelected = selected ? "true" : "false";
  weatherNameInput.classList.toggle("invalid", !selected);
  weatherLatitudeInput.classList.toggle("invalid", !selected);
  weatherLongitudeInput.classList.toggle("invalid", !selected);
  if (!selected) {
    weatherNameInput.setAttribute("aria-invalid", "true");
  } else {
    weatherNameInput.removeAttribute("aria-invalid");
  }
}

function validateWeatherLocation() {
  const latitude = Number.parseFloat(weatherLatitudeInput.value);
  const longitude = Number.parseFloat(weatherLongitudeInput.value);
  const selected = weatherNameInput.dataset.weatherLocationSelected === "true";
  if (!selected || !Number.isFinite(latitude) || !Number.isFinite(longitude)) {
    setWeatherLocationSelected(false);
    throw new Error("Выбери город из списка, чтобы координаты погоды обновились автоматически.");
  }
}

function renderWeatherLocationResults(results) {
  weatherLocationResultsEl.replaceChildren();
  if (!results || results.length === 0) {
    weatherLocationHelpEl.textContent = "Город не найден. Уточни название или попробуй вариант на другом языке.";
    return;
  }
  weatherLocationHelpEl.textContent = "Выбери город из списка, чтобы обновить координаты.";
  for (const result of results) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "location-result";
    button.dataset.locationName = result.name || "";
    button.dataset.locationLatitude = String(result.latitude);
    button.dataset.locationLongitude = String(result.longitude);
    button.textContent = result.displayName || result.name || "Без названия";
    const meta = document.createElement("span");
    meta.className = "location-meta";
    meta.textContent = [
      Number.isFinite(result.latitude) ? Number(result.latitude).toFixed(4) : "",
      Number.isFinite(result.longitude) ? Number(result.longitude).toFixed(4) : "",
      result.timezone || ""
    ].filter(Boolean).join(" · ");
    button.appendChild(meta);
    weatherLocationResultsEl.appendChild(button);
  }
}

async function searchWeatherLocations() {
  const query = weatherNameInput.value.trim();
  if (query.length < 2) {
    weatherLocationResultsEl.replaceChildren();
    weatherLocationHelpEl.textContent = "Введи минимум 2 символа для поиска города.";
    return;
  }
  weatherLocationHelpEl.textContent = "Ищу город...";
  const response = await fetch("/api/weather/locations?q=" + encodeURIComponent(query));
  const payload = await response.json();
  if (!response.ok || !payload.ok) {
    throw new Error(payload.error || "Не удалось найти город.");
  }
  renderWeatherLocationResults(payload.results || []);
}

function queueWeatherLocationSearch() {
  setWeatherLocationSelected(false);
  window.clearTimeout(weatherSearchTimer);
  weatherSearchTimer = window.setTimeout(() => {
    searchWeatherLocations().catch(error => {
      weatherLocationHelpEl.textContent = error.message;
    });
  }, 350);
}

async function saveSettings() {
  const port = Number.parseInt(document.querySelector("#port").value, 10);
  const response = await fetch("/api/settings/printer", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      host: document.querySelector("#host").value,
      port: Number.isFinite(port) ? port : 0
    })
  });
  const payload = await response.json();
  if (!response.ok || !payload.ok) {
    throw new Error(payload.error || "Не удалось сохранить настройки.");
  }
  return payload;
}

async function saveWeatherSettings() {
  validateWeatherLocation();
  const latitude = Number.parseFloat(document.querySelector("#weather-latitude").value);
  const longitude = Number.parseFloat(document.querySelector("#weather-longitude").value);
  const response = await fetch("/api/settings/weather", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      name: document.querySelector("#weather-name").value,
      latitude: Number.isFinite(latitude) ? latitude : 999,
      longitude: Number.isFinite(longitude) ? longitude : 999
    })
  });
  const payload = await response.json();
  if (!response.ok || !payload.ok) {
    throw new Error(payload.error || "Не удалось сохранить настройки погоды.");
  }
  return payload;
}

async function saveFinanceSettings() {
  const amountTon = Number.parseFloat(document.querySelector("#ton-amount").value);
  const investedUsd = Number.parseFloat(document.querySelector("#ton-invested").value);
  const response = await fetch("/api/settings/finance", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      amountTon: Number.isFinite(amountTon) ? amountTon : -1,
      investedUsd: Number.isFinite(investedUsd) ? investedUsd : -1
    })
  });
  const payload = await response.json();
  if (!response.ok || !payload.ok) {
    throw new Error(payload.error || "Не удалось сохранить настройки TON.");
  }
  return payload;
}

async function saveMotivationSettings() {
  const response = await fetch("/api/settings/motivation", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      configured: true,
      baseUrl: document.querySelector("#motivation-base-url").value,
      model: document.querySelector("#motivation-model").value
    })
  });
  const payload = await response.json();
  if (!response.ok || !payload.ok) {
    throw new Error(payload.error || "Не удалось сохранить настройки AI-модели.");
  }
  return payload;
}

function setMotivationStatus(kind, text) {
  motivationStatusEl.className = ["motivation-status", kind].filter(Boolean).join(" ");
  motivationStatusEl.textContent = text;
}

async function testMotivation() {
  setBusy(true);
  setStatus("", "Проверяю AI-модель...");
  setMotivationStatus("", "Отправляю тестовый запрос...");
  try {
    await saveMotivationSettings();
    const response = await fetch("/api/motivation/test", { method: "POST" });
    const payload = await response.json();
    if (!response.ok || !payload.ok) {
      throw new Error(payload.error || "Не удалось проверить AI-модель.");
    }
    const text = payload.quote && payload.quote.text ? "Модель вернула: " + payload.quote.text : "AI-модель отвечает.";
    setMotivationStatus("ok", text);
    setStatus("ok", payload.message || "AI-модель проверена.");
  } catch (error) {
    setMotivationStatus("error", "Ошибка: " + error.message);
    setStatus("error", error.message);
  } finally {
    setBusy(false);
  }
}

function setGoogleStatus(kind, text) {
  googleStatusEl.className = ["motivation-status", kind].filter(Boolean).join(" ");
  googleStatusEl.textContent = text;
}

function renderGoogleStatus(status) {
  if (googleCredentialsPathEl) {
    googleCredentialsPathEl.textContent = (status && status.credentialsPath) || "/data/google/credentials.json";
  }
  if (googleTokenPathEl) {
    googleTokenPathEl.textContent = (status && status.tokenPath) || "/data/google/token.json";
  }
  if (!status || !status.credentialsAvailable) {
    setGoogleStatus("error", "Положи OAuth credentials.json в " + ((status && status.credentialsPath) || "/data/google/credentials.json") + ".");
    return;
  }
  if (status.authorized) {
    setGoogleStatus("ok", "Google подключен. В чек попадут включенные Google-блоки.");
    return;
  }
  setGoogleStatus("", "credentials.json найден. Нажми «Авторизовать», чтобы подключить почту и календарь.");
}

async function loadGoogleStatus() {
  const response = await fetch("/api/google/status");
  const payload = await response.json();
  if (!response.ok || !payload.ok) {
    throw new Error(payload.error || "Не удалось загрузить статус Google.");
  }
  renderGoogleStatus(payload.status);
  return payload.status;
}

async function disconnectGoogle() {
  setBusy(true);
  setStatus("", "Отключаю Google...");
  try {
    const response = await fetch("/api/google/disconnect", { method: "POST" });
    const payload = await response.json();
    if (!response.ok || !payload.ok) {
      throw new Error(payload.error || "Не удалось отключить Google.");
    }
    await loadGoogleStatus();
    setStatus("ok", payload.message || "Google отключен.");
  } catch (error) {
    setStatus("error", error.message);
  } finally {
    setBusy(false);
  }
}

function setFinanceEditing(editing) {
  document.querySelectorAll("[data-finance-input]").forEach(input => {
    if (editing) {
      input.removeAttribute("readonly");
    } else {
      input.setAttribute("readonly", "");
    }
  });
  document.querySelector('[data-action="edit-finance"]').classList.toggle("hidden", editing);
  document.querySelector('[data-action="save-finance"]').classList.toggle("hidden", !editing);
  document.querySelector('[data-action="cancel-finance"]').classList.toggle("hidden", !editing);
}

async function saveFinanceExplicitly() {
  setBusy(true);
  setStatus("", "Сохраняю TON...");
  try {
    await saveFinanceSettings();
    financeDraft = {
      amountTon: document.querySelector("#ton-amount").value,
      investedUsd: document.querySelector("#ton-invested").value
    };
    setFinanceEditing(false);
    setStatus("ok", "Настройки TON сохранены.");
  } catch (error) {
    setStatus("error", error.message);
  } finally {
    setBusy(false);
  }
}

function cancelFinanceEditing() {
  document.querySelector("#ton-amount").value = financeDraft.amountTon;
  document.querySelector("#ton-invested").value = financeDraft.investedUsd;
  setFinanceEditing(false);
  setStatus("", "");
}

function validateNewsSettings() {
  const errors = [];
  document.querySelectorAll("[data-news-source]").forEach(source => {
    const enabled = source.querySelector("[data-news-enabled]").checked;
    const countInput = source.querySelector("[data-news-count]");
    countInput.classList.remove("invalid");
    countInput.removeAttribute("aria-invalid");
    if (!enabled) {
      return;
    }
    const raw = countInput.value.trim();
    const count = Number.parseInt(raw, 10);
    const validInteger = /^\d+$/.test(raw);
    if (!validInteger || !Number.isFinite(count) || count < 1 || count > 100) {
      countInput.classList.add("invalid");
      countInput.setAttribute("aria-invalid", "true");
      const name = source.querySelector("label")?.textContent.trim() || "RSS";
      errors.push(name + ": укажи количество от 1 до 100");
    }
  });
  if (errors.length > 0) {
    throw new Error(errors.join("\n"));
  }
}

function validateDenisTrendsSettings() {
  const errors = [];
  document.querySelectorAll("[data-denis-trend-period]").forEach(row => {
    const enabled = row.querySelector("[data-denis-trend-period-enabled]").checked;
    const countInput = row.querySelector("[data-denis-trend-period-count]");
    countInput.classList.remove("invalid");
    countInput.removeAttribute("aria-invalid");
    if (!enabled) {
      return;
    }
    const raw = countInput.value.trim();
    const count = Number.parseInt(raw, 10);
    if (!/^\d+$/.test(raw) || !Number.isFinite(count) || count < 1 || count > 100) {
      countInput.classList.add("invalid");
      countInput.setAttribute("aria-invalid", "true");
      errors.push("Denis Trends " + (row.dataset.denisTrendPeriod || "period") + ": укажи количество от 1 до 100");
    }
  });
  if (errors.length > 0) {
    throw new Error(errors.join("\n"));
  }
}

async function saveDenisTrendsSettings() {
  validateDenisTrendsSettings();
  const periods = {};
  document.querySelectorAll("[data-denis-trend-period]").forEach(row => {
    const countInput = row.querySelector("[data-denis-trend-period-count]");
    const count = Number.parseInt(countInput.value, 10);
    periods[row.dataset.denisTrendPeriod] = {
      enabled: row.querySelector("[data-denis-trend-period-enabled]").checked,
      maxItems: Number.isFinite(count) && count > 0 ? count : 20
    };
  });
  const response = await fetch("/api/settings/denis-trends", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ periods })
  });
  const payload = await response.json();
  if (!response.ok || !payload.ok) {
    throw new Error(payload.error || "Не удалось сохранить настройки Denis Trends.");
  }
  return payload;
}

async function saveNewsSettings() {
  validateNewsSettings();
  const sources = Array.from(document.querySelectorAll("[data-news-source]")).map(source => {
    const countInput = source.querySelector("[data-news-count]");
    const count = Number.parseInt(countInput.value, 10);
    return {
      preset: source.querySelector("[data-news-preset]").value,
      enabled: source.querySelector("[data-news-enabled]").checked,
      feedUrl: source.querySelector("[data-news-url-visible]").value,
      maxItems: Number.isFinite(count) && count > 0 ? count : 1
    };
  });
  const response = await fetch("/api/settings/news", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      translateTitles: document.querySelector("#news-translate").checked,
      sources
    })
  });
  const payload = await response.json();
  if (!response.ok || !payload.ok) {
    throw new Error(payload.error || "Не удалось сохранить настройки новостей.");
  }
  return payload;
}

async function saveReceiptSnapshotSettings() {
  const baseUrlInput = document.querySelector("#receipt-snapshot-base-url");
  if (baseUrlInput && !baseUrlInput.value.trim()) {
    baseUrlInput.value = defaultReceiptSnapshotBaseURL();
  }
  const response = await fetch("/api/settings/receipt-snapshot", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      baseUrl: baseUrlInput ? baseUrlInput.value : defaultReceiptSnapshotBaseURL()
    })
  });
  const payload = await response.json();
  if (!response.ok || !payload.ok) {
    throw new Error(payload.error || "Не удалось сохранить настройки онлайн-слепка.");
  }
  return payload;
}

function defaultReceiptSnapshotBaseURL() {
  return window.location.origin.replace(/\/+$/, "");
}

function readFont(selector) {
  const value = Number.parseInt(document.querySelector(selector).value, 10);
  return Number.isFinite(value) ? value : 0;
}

function readAlignment(selector, fallback) {
  const value = document.querySelector(selector).value;
  return ["left", "center", "right"].includes(value) ? value : fallback;
}

function readFontRowAlign(name) {
  const btn = document.querySelector(`.font-align-group[data-for="${name}"] .align-btn.active`);
  return btn ? btn.dataset.align : "center";
}

function setFontRowAlign(name, value) {
  const group = document.querySelector(`.font-align-group[data-for="${name}"]`);
  if (!group) return;
  const target = ["left", "center", "right"].includes(value) ? value : "center";
  group.querySelectorAll(".align-btn").forEach(b =>
    b.classList.toggle("active", b.dataset.align === target)
  );
}

const fontRowSamples = {
  calendar: "1 июня · Пятница",
  temperature: "+24°C",
  normal: "Коротко о мире"
};

function renderFontRowPreview(card) {
  const container = card.querySelector(".font-row-preview");
  if (!container) return;
  const name = card.dataset.row;
  const fontSelect = card.querySelector("[data-font-select]");
  const font = fontSelect ? (Number.parseInt(fontSelect.value, 10) || 0) : 0;
  const metric = metricForFont(font);
  const baseWidth = fallbackFontMetrics[0].fontWidth;
  const fontWidth = metric.fontWidth || baseWidth;
  const lineSize = Math.max(10, Math.min(32, 16 * fontWidth / baseWidth));
  const dwCb = card.querySelector(".font-toggle-input[id$='-double-width']");
  const dhCb = card.querySelector(".font-toggle-input[id$='-double-height']");
  const dw = dwCb ? dwCb.checked : false;
  const dh = dhCb ? dhCb.checked : false;
  const alignBtn = card.querySelector(".font-align-group .align-btn.active");
  const alignment = alignBtn ? alignBtn.dataset.align : "center";
  container.innerHTML = "";
  const line = document.createElement("div");
  line.className = "receipt-line align-" + alignment;
  line.style.setProperty("--line-size", lineSize.toFixed(2) + "px");
  line.style.setProperty("--line-scale-x", dw ? 2 : 1);
  line.style.setProperty("--line-scale-y", dh ? 2 : 1);
  const text = document.createElement("span");
  text.className = "receipt-line-text";
  text.textContent = fontRowSamples[name] || "Пример текста";
  line.appendChild(text);
  container.appendChild(line);
}

function renderAllFontRowPreviews() {
  document.querySelectorAll(".font-row-card[data-row]").forEach(renderFontRowPreview);
}

function metricForFont(font) {
  return fontMetrics.get(font) || fontMetrics.get(0) || fallbackFontMetrics[0];
}

function fontLabel(metric) {
  const lineLength = metric.lineLength ? metric.lineLength + " симв/стр" : "длина ?";
  const fontWidth = metric.fontWidth ? metric.fontWidth + " px" : "ширина ?";
  return "Шрифт " + metric.font + " · " + lineLength + " · " + fontWidth;
}

function updateFontControls() {
  const metrics = Array.from(fontMetrics.values()).sort((a, b) => a.font - b.font);
  const visibleMetrics = metrics.length > 0 ? metrics : fallbackFontMetrics;
  document.querySelectorAll("[data-font-select]").forEach(select => {
    const selected = select.value;
    select.replaceChildren(...visibleMetrics.map(metric => {
      const option = document.createElement("option");
      option.value = String(metric.font);
      option.textContent = fontLabel(metric);
      return option;
    }));
    if (visibleMetrics.some(metric => String(metric.font) === selected)) {
      select.value = selected;
    }
  });
  fontMetricsEl.textContent = visibleMetrics.map(fontLabel).join("\n");
  renderAllFontRowPreviews();
}

async function saveReceiptStyle() {
  const response = await fetch("/api/settings/receipt-style", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      configured: true,
      normalFont: readFont("#normal-font"),
      emphasisFont: readFont("#calendar-font"),
      calendarFont: readFont("#calendar-font"),
      temperatureFont: readFont("#temperature-font"),
      calendarDoubleWidth: document.querySelector("#calendar-double-width").checked,
      calendarDoubleHeight: document.querySelector("#calendar-double-height").checked,
      temperatureDoubleWidth: document.querySelector("#temperature-double-width").checked,
      temperatureDoubleHeight: document.querySelector("#temperature-double-height").checked,
      calendarAlignment: readFontRowAlign("calendar"),
      temperatureAlignment: readFontRowAlign("temperature"),
      normalAlignment: readFontRowAlign("normal")
    })
  });
  const payload = await response.json();
  if (!response.ok || !payload.ok) {
    throw new Error(payload.error || "Не удалось сохранить настройки чека.");
  }
  return payload;
}

function checked(selector) {
  return document.querySelector(selector).checked;
}

function readReceiptContentSettings() {
  return {
    configured: true,
    showWeather: checked("#content-weather"),
    showWeatherAdvice: checked("#content-weather-advice"),
    showMotivationQuote: checked("#content-motivation-quote"),
    showTonPortfolio: checked("#content-ton-portfolio"),
    showUsdBynRate: checked("#content-usd-byn-rate"),
    showBankRates: checked("#content-bank-rates"),
    showMail: checked("#content-mail"),
    showCalendar: checked("#content-calendar"),
    showHistory: checked("#content-history"),
    showNews: checked("#content-news"),
    showDenisTrends: checked("#content-denis-trends"),
    denisTrendsMode: denisTrendsMode(document.querySelector("#content-denis-trends-mode")?.value)
  };
}

async function saveReceiptContentSettings(content = readReceiptContentSettings()) {
  const response = await fetch("/api/settings/receipt-content", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(content)
  });
  const payload = await response.json();
  if (!response.ok || !payload.ok) {
    throw new Error(payload.error || "Не удалось сохранить состав чека.");
  }
  return payload;
}

async function saveAllSettings() {
  const receiptContent = readReceiptContentSettings();
  await saveSettings();
  await saveReceiptContentSettings(receiptContent);
  if (receiptContent.showWeather || receiptContent.showWeatherAdvice) {
    await saveWeatherSettings();
  }
  await saveMotivationSettings();
  await saveNewsSettings();
  await saveDenisTrendsSettings();
  await saveReceiptSnapshotSettings();
  await saveReceiptStyle();
}

function scheduleMode() {
  const checked = document.querySelector('input[name="schedule-mode"]:checked');
  return checked ? checked.value : "interval";
}

function updateScheduleMode() {
  const mode = scheduleMode();
  document.querySelector("#schedule-interval-panel").hidden = mode !== "interval";
  document.querySelector("#schedule-times-panel").hidden = mode !== "daily_times";
}

function scheduleContentFromSettings(settings) {
  const content = settings || {};
  return {
    configured: true,
    showWeather: Boolean(content.showWeather),
    showWeatherAdvice: Boolean(content.showWeatherAdvice),
    showMotivationQuote: Boolean(content.showMotivationQuote),
    showTonPortfolio: Boolean(content.showTonPortfolio),
    showUsdBynRate: Boolean(content.showUsdBynRate),
    showBankRates: Boolean(content.showBankRates),
    showMail: Boolean(content.showMail),
    showCalendar: Boolean(content.showCalendar),
    showHistory: Boolean(content.showHistory),
    showNews: Boolean(content.showNews),
    showDenisTrends: Boolean(content.showDenisTrends),
    denisTrendsMode: denisTrendsMode(content.denisTrendsMode)
  };
}

function denisTrendsMode(value) {
  if (value === "now" || value === "day") {
    return value;
  }
  return "auto";
}

function scheduleRunFromValue(value) {
  if (typeof value === "object" && value !== null) {
    return {
      time: valueOrEmpty(value.time),
      content: value.content || null
    };
  }
  return {
    time: valueOrEmpty(value),
    content: null
  };
}

function scheduleRowContent(row) {
  const content = { configured: true };
  row.querySelectorAll("[data-schedule-content-key]").forEach(input => {
    content[input.dataset.scheduleContentKey] = input.checked;
  });
  content.denisTrendsMode = row.querySelector("[data-schedule-denis-trends-mode]")?.value || "auto";
  return scheduleContentFromSettings(content);
}

function readIntervalContentSettings() {
  const content = { configured: true };
  document.querySelectorAll("[data-interval-content-key]").forEach(input => {
    content[input.dataset.intervalContentKey] = input.checked;
  });
  content.denisTrendsMode = document.querySelector("[data-interval-denis-trends-mode]")?.value || "auto";
  return scheduleContentFromSettings(content);
}

function updateScheduleRowSummary(row) {
  const summary = row.querySelector("[data-schedule-run-summary]");
  if (!summary) {
    return;
  }
  const selected = scheduleContentOptions
    .filter(option => row.querySelector('[data-schedule-content-key="' + option.key + '"]')?.checked)
    .map(option => option.label);
  summary.textContent = selected.length > 0 ? "Печатает: " + selected.join(", ") : "Ничего не выбрано";
}

function addScheduleTime(value = "07:00") {
  const run = scheduleRunFromValue(value);
  const customContent = scheduleContentFromSettings(run.content || readReceiptContentSettings());
  const row = document.createElement("div");
  row.className = "schedule-time-row";
  row.dataset.scheduleTimeRow = "";

  const input = document.createElement("input");
  input.type = "time";
  input.dataset.scheduleTime = "";
  input.value = run.time;

  const timeField = document.createElement("label");
  timeField.className = "schedule-row-field";
  const timeTitle = document.createElement("span");
  timeTitle.className = "schedule-row-label";
  timeTitle.textContent = "Время";
  timeField.append(timeTitle, input);

  const remove = document.createElement("button");
  remove.type = "button";
  remove.className = "secondary";
  remove.dataset.action = "remove-schedule-time";
  remove.textContent = "Удалить";

  const custom = document.createElement("div");
  custom.className = "schedule-custom-content";
  renderScheduleContentGroups(custom, customContent, "scheduleContentKey");

  row.append(timeField, remove, custom);
  document.querySelector("#schedule-time-list").appendChild(row);
}

function readScheduleSettings(enabled) {
  const interval = Number.parseInt(document.querySelector("#schedule-interval").value, 10);
  const runs = Array.from(document.querySelectorAll("[data-schedule-time-row]"))
    .map(row => {
      const time = row.querySelector("[data-schedule-time]")?.value.trim() || "";
      if (!time) {
        return null;
      }
      return {
        time,
        profile: "custom",
        content: scheduleRowContent(row)
      };
    })
    .filter(Boolean);
  const times = runs.map(run => run.time);
  return {
    enabled,
    mode: scheduleMode(),
    intervalMinutes: Number.isFinite(interval) ? interval : 15,
    intervalContent: readIntervalContentSettings(),
    times,
    runs,
    timezone: "Europe/Minsk"
  };
}

function formatDateTime(value) {
  if (!value || value.startsWith("0001-01-01")) {
    return "—";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "—";
  }
  return date.toLocaleString("ru-RU", {
    day: "2-digit",
    month: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  });
}

function renderSchedulerStatus(status) {
  if (!status) {
    schedulerStatusEl.textContent = "Статус расписания недоступен.";
    return;
  }
  const enabled = status.settings && status.settings.enabled ? "включено" : "выключено";
  const mode = status.settings && status.settings.mode === "daily_times" ? "точное время" : "интервал";
  schedulerStatusEl.textContent = [
    "Состояние: " + enabled + " · " + mode + (status.running ? " · печатает сейчас" : ""),
    "Следующий запуск: " + formatDateTime(status.nextRunAt),
    "Последний запуск: " + formatDateTime(status.lastSuccessAt),
    "Ошибка: " + (status.lastError || "—")
  ].join("\n");
}

async function loadSchedulerStatus() {
  try {
    const response = await fetch("/api/scheduler/status");
    const payload = await response.json();
    if (!response.ok || !payload.ok) {
      throw new Error(payload.error || "Не удалось загрузить статус расписания.");
    }
    renderSchedulerStatus(payload.status);
  } catch (error) {
    schedulerStatusEl.textContent = error.message;
  }
}

async function saveScheduleSettings() {
  setBusy(true);
  setStatus("", "Сохраняю расписание...");
  try {
    await saveAllSettings();
    const response = await fetch("/api/settings/schedule", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(readScheduleSettings(true))
    });
    const payload = await response.json();
    if (!response.ok || !payload.ok) {
      throw new Error(payload.error || "Не удалось сохранить расписание.");
    }
    await loadSchedulerStatus();
    setStatus("ok", payload.message || "Расписание сохранено.");
  } catch (error) {
    setStatus("error", error.message);
  } finally {
    setBusy(false);
  }
}

async function stopSchedule() {
  setBusy(true);
  setStatus("", "Останавливаю расписание...");
  try {
    const response = await fetch("/api/settings/schedule/stop", { method: "POST" });
    const payload = await response.json();
    if (!response.ok || !payload.ok) {
      throw new Error(payload.error || "Не удалось остановить расписание.");
    }
    await loadSchedulerStatus();
    setStatus("ok", payload.message || "Расписание остановлено.");
  } catch (error) {
    setStatus("error", error.message);
  } finally {
    setBusy(false);
  }
}

function setImageEditorStatus(kind, message) {
  if (!imageEditorStatusEl) {
    return;
  }
  imageEditorStatusEl.className = "image-editor-status" + (kind ? " " + kind : "");
  imageEditorStatusEl.textContent = message;
}

function imageEditorCanvasHeight() {
  const value = Number.parseInt(imageEditorCanvasHeightInput && imageEditorCanvasHeightInput.value, 10);
  if (!Number.isFinite(value)) {
    return imageEditorResultCanvas.height || 160;
  }
  return Math.max(32, Math.min(2048, value));
}

function imageEditorSettings() {
  return {
    canvasHeight: imageEditorCanvasHeight(),
    rotation: Number.parseInt(document.querySelector("#image-editor-rotation").value, 10) || 0,
    brightness: Number.parseInt(document.querySelector("#image-editor-brightness").value, 10) || 0,
    contrast: Number.parseInt(document.querySelector("#image-editor-contrast").value, 10) || 0,
    threshold: Number.parseInt(document.querySelector("#image-editor-threshold").value, 10) || 128,
    dither: document.querySelector("#image-editor-dither").checked,
    invert: document.querySelector("#image-editor-invert").checked,
    tool: document.querySelector("#image-editor-tool").value,
    brush: Number.parseInt(document.querySelector("#image-editor-brush").value, 10) || 6
  };
}

function applyImageEditorSettings(settings) {
  if (!settings) {
    return;
  }
  const setControlValue = (selector, value) => {
    if (value !== undefined && value !== null) {
      document.querySelector(selector).value = String(value);
    }
  };
  setControlValue("#image-editor-rotation", settings.rotation);
  setControlValue("#image-editor-canvas-height", settings.canvasHeight);
  setControlValue("#image-editor-brightness", settings.brightness);
  setControlValue("#image-editor-contrast", settings.contrast);
  setControlValue("#image-editor-threshold", settings.threshold);
  setControlValue("#image-editor-tool", settings.tool);
  setControlValue("#image-editor-brush", settings.brush);
  if (settings.dither !== undefined) {
    document.querySelector("#image-editor-dither").checked = Boolean(settings.dither);
  }
  if (settings.invert !== undefined) {
    document.querySelector("#image-editor-invert").checked = Boolean(settings.invert);
  }
}

function resizeImageEditorCanvases(width, height) {
  const safeHeight = Math.max(1, Math.min(2048, Math.round(height || 1)));
  [imageEditorSourceCanvas, imageEditorResultCanvas].forEach(canvas => {
    canvas.width = width;
    canvas.height = safeHeight;
    canvas.style.aspectRatio = width + " / " + safeHeight;
  });
  if (imageEditorCanvasHeightInput) {
    imageEditorCanvasHeightInput.value = String(safeHeight);
  }
}

function clearImageEditorCanvas(canvas) {
  const context = canvas.getContext("2d");
  context.fillStyle = "#fff";
  context.fillRect(0, 0, canvas.width, canvas.height);
}

function createBlankImageEditorCanvas(height, options = {}) {
  const safeHeight = Math.max(32, Math.min(2048, Math.round(height || 160)));
  revokeImageEditorObjectURL();
  imageEditorSourceImage = null;
  imageEditorShapeStart = null;
  imageEditorShapeBasePixels = null;
  imageEditorLastSavedState = null;
  if (imageEditorFileInput) {
    imageEditorFileInput.value = "";
  }
  resizeImageEditorCanvases(imageEditorWidth, safeHeight);
  clearImageEditorCanvas(imageEditorSourceCanvas);
  imageEditorPixels = new Uint8Array(imageEditorResultCanvas.width * imageEditorResultCanvas.height);
  renderImageEditorResult();
  if (options.status !== false) {
    setImageEditorStatus("", "Пустой холст готов: " + imageEditorResultCanvas.width + "x" + imageEditorResultCanvas.height + " px.");
  }
}

function resetImageEditorCanvases() {
  createBlankImageEditorCanvas(imageEditorCanvasHeight(), { status: false });
}

function revokeImageEditorObjectURL() {
  if (imageEditorSourceObjectURL) {
    URL.revokeObjectURL(imageEditorSourceObjectURL);
    imageEditorSourceObjectURL = "";
  }
}

async function loadImageEditorState() {
  const response = await fetch("/api/image-editor/state");
  const payload = await response.json();
  if (!response.ok || !payload.ok) {
    throw new Error(payload.error || "Не удалось загрузить изображение.");
  }
  const state = payload.data || {};
  imageEditorLastSavedState = state;
  if (!state.available) {
    resetImageEditorCanvases();
    setImageEditorStatus("", "Пустой холст готов: можно рисовать без загрузки файла.");
    return;
  }
  applyImageEditorSettings(state.settings || {});
  await loadSavedImageEditorPreview(state);
  setImageEditorStatus("ok", "Сохранено: " + state.width + "x" + state.height + " px.");
}

function loadImageElement(src) {
  return new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => resolve(image);
    image.onerror = () => reject(new Error("Не удалось загрузить изображение."));
    image.src = src;
  });
}

async function loadSavedImageEditorPreview(state) {
  const image = await loadImageElement((state.previewUrl || "/api/image-editor/preview") + "?v=" + Date.now());
  imageEditorSourceImage = image;
  resizeImageEditorCanvases(state.width || image.width || imageEditorWidth, state.height || image.height || 160);
  const sourceContext = imageEditorSourceCanvas.getContext("2d");
  sourceContext.fillStyle = "#fff";
  sourceContext.fillRect(0, 0, imageEditorSourceCanvas.width, imageEditorSourceCanvas.height);
  sourceContext.drawImage(image, 0, 0, imageEditorSourceCanvas.width, imageEditorSourceCanvas.height);
  const resultContext = imageEditorResultCanvas.getContext("2d");
  resultContext.fillStyle = "#fff";
  resultContext.fillRect(0, 0, imageEditorResultCanvas.width, imageEditorResultCanvas.height);
  resultContext.drawImage(image, 0, 0, imageEditorResultCanvas.width, imageEditorResultCanvas.height);
  imageEditorPixels = pixelsFromResultCanvas();
}

async function loadImageEditorFile(file) {
  if (!file) {
    return;
  }
  revokeImageEditorObjectURL();
  imageEditorSourceObjectURL = URL.createObjectURL(file);
  imageEditorSourceImage = await loadImageElement(imageEditorSourceObjectURL);
  imageEditorLastSavedState = null;
  applyImageEditorProcessing();
  setImageEditorStatus("", "Готово к сохранению: " + imageEditorResultCanvas.width + "x" + imageEditorResultCanvas.height + " px.");
}

function rotatedImageCanvas(image, rotation) {
  const naturalWidth = image.naturalWidth || image.width;
  const naturalHeight = image.naturalHeight || image.height;
  const quarterTurn = rotation === 90 || rotation === 270;
  const canvas = document.createElement("canvas");
  canvas.width = quarterTurn ? naturalHeight : naturalWidth;
  canvas.height = quarterTurn ? naturalWidth : naturalHeight;
  const context = canvas.getContext("2d");
  context.fillStyle = "#fff";
  context.fillRect(0, 0, canvas.width, canvas.height);
  context.save();
  switch (rotation) {
    case 90:
      context.translate(canvas.width, 0);
      context.rotate(Math.PI / 2);
      break;
    case 180:
      context.translate(canvas.width, canvas.height);
      context.rotate(Math.PI);
      break;
    case 270:
      context.translate(0, canvas.height);
      context.rotate(-Math.PI / 2);
      break;
    default:
      break;
  }
  context.drawImage(image, 0, 0);
  context.restore();
  return canvas;
}

function drawSourceImageToCanvas() {
  if (!imageEditorSourceImage) {
    return false;
  }
  const settings = imageEditorSettings();
  const rotated = rotatedImageCanvas(imageEditorSourceImage, settings.rotation);
  const targetHeight = Math.max(1, Math.min(2048, Math.round(rotated.height * imageEditorWidth / rotated.width)));
  resizeImageEditorCanvases(imageEditorWidth, targetHeight);
  const context = imageEditorSourceCanvas.getContext("2d");
  context.imageSmoothingEnabled = true;
  context.fillStyle = "#fff";
  context.fillRect(0, 0, imageEditorSourceCanvas.width, imageEditorSourceCanvas.height);
  context.drawImage(rotated, 0, 0, imageEditorSourceCanvas.width, imageEditorSourceCanvas.height);
  return true;
}

function applyImageEditorProcessing() {
  if (!drawSourceImageToCanvas()) {
    return;
  }
  const settings = imageEditorSettings();
  const sourceContext = imageEditorSourceCanvas.getContext("2d");
  const imageData = sourceContext.getImageData(0, 0, imageEditorSourceCanvas.width, imageEditorSourceCanvas.height);
  const grayscale = new Float32Array(imageEditorSourceCanvas.width * imageEditorSourceCanvas.height);
  const contrastValue = settings.contrast * 2.55;
  const contrastFactor = (259 * (contrastValue + 255)) / (255 * (259 - contrastValue));
  for (let index = 0; index < grayscale.length; index++) {
    const offset = index * 4;
    const luminance = imageData.data[offset] * 0.299 + imageData.data[offset + 1] * 0.587 + imageData.data[offset + 2] * 0.114;
    grayscale[index] = Math.max(0, Math.min(255, contrastFactor * (luminance - 128) + 128 + settings.brightness));
  }

  const pixels = new Uint8Array(grayscale.length);
  if (settings.dither) {
    const work = Float32Array.from(grayscale);
    const width = imageEditorSourceCanvas.width;
    const height = imageEditorSourceCanvas.height;
    for (let y = 0; y < height; y++) {
      for (let x = 0; x < width; x++) {
        const index = y * width + x;
        const black = work[index] < settings.threshold;
        const quantizedLuminance = black ? 0 : 255;
        const error = work[index] - quantizedLuminance;
        pixels[index] = black ? 255 : 0;
        if (x + 1 < width) work[index + 1] += error * 7 / 16;
        if (y + 1 >= height) continue;
        if (x > 0) work[index + width - 1] += error * 3 / 16;
        work[index + width] += error * 5 / 16;
        if (x + 1 < width) work[index + width + 1] += error / 16;
      }
    }
  } else {
    for (let index = 0; index < grayscale.length; index++) {
      pixels[index] = grayscale[index] < settings.threshold ? 255 : 0;
    }
  }

  if (settings.invert) {
    for (let index = 0; index < pixels.length; index++) {
      pixels[index] = pixels[index] ? 0 : 255;
    }
  }
  imageEditorPixels = pixels;
  renderImageEditorResult();
}

function renderImageEditorResult() {
  if (!imageEditorPixels) {
    clearImageEditorCanvas(imageEditorResultCanvas);
    return;
  }
  const context = imageEditorResultCanvas.getContext("2d");
  const imageData = context.createImageData(imageEditorResultCanvas.width, imageEditorResultCanvas.height);
  for (let index = 0; index < imageEditorPixels.length; index++) {
    const color = imageEditorPixels[index] ? 0 : 255;
    const offset = index * 4;
    imageData.data[offset] = color;
    imageData.data[offset + 1] = color;
    imageData.data[offset + 2] = color;
    imageData.data[offset + 3] = 255;
  }
  context.putImageData(imageData, 0, 0);
}

function pixelsFromResultCanvas() {
  const context = imageEditorResultCanvas.getContext("2d");
  const imageData = context.getImageData(0, 0, imageEditorResultCanvas.width, imageEditorResultCanvas.height);
  const pixels = new Uint8Array(imageEditorResultCanvas.width * imageEditorResultCanvas.height);
  for (let index = 0; index < pixels.length; index++) {
    const offset = index * 4;
    const luminance = imageData.data[offset] * 0.299 + imageData.data[offset + 1] * 0.587 + imageData.data[offset + 2] * 0.114;
    pixels[index] = luminance < 128 ? 255 : 0;
  }
  return pixels;
}

function imageEditorCanvasPoint(event) {
  const rect = imageEditorResultCanvas.getBoundingClientRect();
  return {
    x: Math.floor((event.clientX - rect.left) * imageEditorResultCanvas.width / rect.width),
    y: Math.floor((event.clientY - rect.top) * imageEditorResultCanvas.height / rect.height)
  };
}

function ensureImageEditorPixelBuffer() {
  const expectedLength = imageEditorResultCanvas.width * imageEditorResultCanvas.height;
  if (!imageEditorPixels || imageEditorPixels.length !== expectedLength) {
    imageEditorPixels = pixelsFromResultCanvas();
  }
}

function imageEditorTool() {
  return document.querySelector("#image-editor-tool").value;
}

function imageEditorDrawValue(tool) {
  return tool === "eraser" ? 0 : 255;
}

function isImageEditorShapeTool(tool) {
  return tool === "line" || tool === "rectangle" || tool === "filled-rectangle" || tool === "ellipse" || tool === "filled-ellipse";
}

function imageEditorBrushRadius() {
  const size = Number.parseInt(document.querySelector("#image-editor-brush").value, 10) || 1;
  return Math.max(0, Math.floor(size / 2));
}

function setImageEditorPixel(pixels, width, height, x, y, value) {
  if (x < 0 || x >= width || y < 0 || y >= height) {
    return;
  }
  pixels[y * width + x] = value;
}

function paintImageEditorDisk(pixels, width, height, centerX, centerY, radius, value) {
  if (radius <= 0) {
    setImageEditorPixel(pixels, width, height, centerX, centerY, value);
    return;
  }
  for (let y = centerY - radius; y <= centerY + radius; y++) {
    if (y < 0 || y >= height) continue;
    for (let x = centerX - radius; x <= centerX + radius; x++) {
      if (x < 0 || x >= width) continue;
      const dx = x - centerX;
      const dy = y - centerY;
      if (dx * dx + dy * dy <= radius * radius) {
        pixels[y * width + x] = value;
      }
    }
  }
}

function drawImageEditorLine(pixels, width, height, from, to, radius, value) {
  const steps = Math.max(Math.abs(to.x - from.x), Math.abs(to.y - from.y), 1);
  for (let step = 0; step <= steps; step++) {
    const progress = step / steps;
    const x = Math.round(from.x + (to.x - from.x) * progress);
    const y = Math.round(from.y + (to.y - from.y) * progress);
    paintImageEditorDisk(pixels, width, height, x, y, radius, value);
  }
}

function normalizedImageEditorRect(from, to) {
  return {
    left: Math.min(from.x, to.x),
    top: Math.min(from.y, to.y),
    right: Math.max(from.x, to.x),
    bottom: Math.max(from.y, to.y)
  };
}

function drawImageEditorRectangle(pixels, width, height, from, to, radius, value, filled) {
  const rect = normalizedImageEditorRect(from, to);
  if (filled) {
    for (let y = rect.top; y <= rect.bottom; y++) {
      for (let x = rect.left; x <= rect.right; x++) {
        setImageEditorPixel(pixels, width, height, x, y, value);
      }
    }
    return;
  }
  drawImageEditorLine(pixels, width, height, { x: rect.left, y: rect.top }, { x: rect.right, y: rect.top }, radius, value);
  drawImageEditorLine(pixels, width, height, { x: rect.right, y: rect.top }, { x: rect.right, y: rect.bottom }, radius, value);
  drawImageEditorLine(pixels, width, height, { x: rect.right, y: rect.bottom }, { x: rect.left, y: rect.bottom }, radius, value);
  drawImageEditorLine(pixels, width, height, { x: rect.left, y: rect.bottom }, { x: rect.left, y: rect.top }, radius, value);
}

function drawImageEditorEllipse(pixels, width, height, from, to, radius, value, filled) {
  const rect = normalizedImageEditorRect(from, to);
  const centerX = (rect.left + rect.right) / 2;
  const centerY = (rect.top + rect.bottom) / 2;
  const radiusX = Math.max(1, (rect.right - rect.left) / 2);
  const radiusY = Math.max(1, (rect.bottom - rect.top) / 2);
  if (filled) {
    for (let y = rect.top; y <= rect.bottom; y++) {
      for (let x = rect.left; x <= rect.right; x++) {
        const normalized = ((x - centerX) * (x - centerX)) / (radiusX * radiusX) + ((y - centerY) * (y - centerY)) / (radiusY * radiusY);
        if (normalized <= 1) {
          setImageEditorPixel(pixels, width, height, x, y, value);
        }
      }
    }
    return;
  }
  const steps = Math.max(24, Math.ceil(Math.PI * 2 * Math.max(radiusX, radiusY)));
  for (let step = 0; step <= steps; step++) {
    const angle = (Math.PI * 2 * step) / steps;
    const x = Math.round(centerX + Math.cos(angle) * radiusX);
    const y = Math.round(centerY + Math.sin(angle) * radiusY);
    paintImageEditorDisk(pixels, width, height, x, y, radius, value);
  }
}

function applyImageEditorShape(tool, from, to, targetPixels) {
  const width = imageEditorResultCanvas.width;
  const height = imageEditorResultCanvas.height;
  const radius = imageEditorBrushRadius();
  const value = imageEditorDrawValue(tool);
  switch (tool) {
    case "line":
      drawImageEditorLine(targetPixels, width, height, from, to, radius, value);
      break;
    case "rectangle":
      drawImageEditorRectangle(targetPixels, width, height, from, to, radius, value, false);
      break;
    case "filled-rectangle":
      drawImageEditorRectangle(targetPixels, width, height, from, to, radius, value, true);
      break;
    case "ellipse":
      drawImageEditorEllipse(targetPixels, width, height, from, to, radius, value, false);
      break;
    case "filled-ellipse":
      drawImageEditorEllipse(targetPixels, width, height, from, to, radius, value, true);
      break;
    default:
      drawImageEditorLine(targetPixels, width, height, from, to, radius, value);
      break;
  }
}

function applyImageEditorBrushAt(point) {
  ensureImageEditorPixelBuffer();
  const tool = imageEditorTool();
  const value = imageEditorDrawValue(tool);
  const radius = imageEditorBrushRadius();
  const width = imageEditorResultCanvas.width;
  const height = imageEditorResultCanvas.height;
  paintImageEditorDisk(imageEditorPixels, width, height, point.x, point.y, radius, value);
  renderImageEditorResult();
  imageEditorLastSavedState = null;
}

function previewImageEditorShape(to) {
  if (!imageEditorShapeStart || !imageEditorShapeBasePixels) {
    return;
  }
  const tool = imageEditorTool();
  imageEditorPixels = imageEditorShapeBasePixels.slice();
  applyImageEditorShape(tool, imageEditorShapeStart, to, imageEditorPixels);
  renderImageEditorResult();
  imageEditorLastSavedState = null;
}

function beginImageEditorPointer(event) {
  event.preventDefault();
  ensureImageEditorPixelBuffer();
  const point = imageEditorCanvasPoint(event);
  const tool = imageEditorTool();
  imageEditorPointerDown = true;
  if (isImageEditorShapeTool(tool)) {
    imageEditorShapeStart = point;
    imageEditorShapeBasePixels = imageEditorPixels.slice();
    previewImageEditorShape(point);
    return;
  }
  imageEditorShapeStart = null;
  imageEditorShapeBasePixels = null;
  applyImageEditorBrushAt(point);
}

function moveImageEditorPointer(event) {
  if (!imageEditorPointerDown) {
    return;
  }
  event.preventDefault();
  const point = imageEditorCanvasPoint(event);
  if (imageEditorShapeStart) {
    previewImageEditorShape(point);
    return;
  }
  applyImageEditorBrushAt(point);
}

function endImageEditorPointer(event) {
  if (!imageEditorPointerDown) {
    return;
  }
  event.preventDefault();
  const point = imageEditorCanvasPoint(event);
  if (imageEditorShapeStart) {
    previewImageEditorShape(point);
    imageEditorShapeStart = null;
    imageEditorShapeBasePixels = null;
  }
  imageEditorPointerDown = false;
}

async function saveImageEditorToServer() {
  if (!imageEditorPixels || imageEditorPixels.length === 0) {
    throw new Error("Сначала создай холст или загрузи изображение.");
  }
  const payload = {
    width: imageEditorResultCanvas.width,
    height: imageEditorResultCanvas.height,
    pixels: Array.from(imageEditorPixels),
    previewPng: imageEditorResultCanvas.toDataURL("image/png"),
    settings: imageEditorSettings()
  };
  const response = await fetch("/api/image-editor/save", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });
  const result = await response.json();
  if (!response.ok || !result.ok) {
    throw new Error(result.error || "Не удалось сохранить изображение.");
  }
  imageEditorLastSavedState = result.data || null;
  return result;
}

async function saveImageEditor() {
  setBusy(true);
  setImageEditorStatus("", "Сохраняю изображение...");
  try {
    const result = await saveImageEditorToServer();
    setImageEditorStatus("ok", result.message || "Изображение сохранено.");
    setStatus("ok", result.message || "Изображение сохранено.");
  } catch (error) {
    setImageEditorStatus("error", error.message);
    setStatus("error", error.message);
  } finally {
    setBusy(false);
  }
}

async function printImageEditor() {
  setBusy(true);
  setImageEditorStatus("", "Печатаю изображение...");
  try {
    if (imageEditorPixels && (!imageEditorLastSavedState || imageEditorLastSavedState.width !== imageEditorResultCanvas.width || imageEditorLastSavedState.height !== imageEditorResultCanvas.height)) {
      await saveImageEditorToServer();
    } else if (imageEditorPixels) {
      await saveImageEditorToServer();
    }
    const response = await fetch("/api/image-editor/print", { method: "POST" });
    const payload = await response.json();
    if (!response.ok || !payload.ok) {
      throw new Error(payload.error || "Не удалось напечатать изображение.");
    }
    setImageEditorStatus("ok", payload.message || "Изображение напечатано.");
    setStatus("ok", payload.message || "Изображение напечатано.");
  } catch (error) {
    setImageEditorStatus("error", error.message);
    setStatus("error", error.message);
  } finally {
    setBusy(false);
  }
}

async function clearImageEditor() {
  setBusy(true);
  setImageEditorStatus("", "Удаляю изображение...");
  try {
    const response = await fetch("/api/image-editor/image", { method: "DELETE" });
    const payload = await response.json();
    if (!response.ok || !payload.ok) {
      throw new Error(payload.error || "Не удалось удалить изображение.");
    }
    revokeImageEditorObjectURL();
    imageEditorSourceImage = null;
    imageEditorLastSavedState = null;
    if (imageEditorFileInput) {
      imageEditorFileInput.value = "";
    }
    resetImageEditorCanvases();
    setImageEditorStatus("", "Нет сохраненного изображения.");
    setStatus("ok", payload.message || "Изображение удалено.");
  } catch (error) {
    setImageEditorStatus("error", error.message);
    setStatus("error", error.message);
  } finally {
    setBusy(false);
  }
}

function renderPreview(lines) {
  renderLinesPreview(previewEl, lines, "#normal-font");
}

function renderLinesPreview(target, lines, baseFontSelector) {
  if (!target) {
    return;
  }
  const paper = document.createElement("div");
  paper.className = "receipt-paper";
  const normalMetric = metricForFont(readFont(baseFontSelector || "#normal-font"));
  paper.style.setProperty("--paper-chars", normalMetric.lineLength || 32);
  for (const line of lines) {
    const node = document.createElement("div");
    const qrValue = typeof line.QRCode === "string" ? line.QRCode.trim() : "";
    if (qrValue) {
      node.className = [
        "receipt-line",
        "receipt-qr-line",
        "align-" + (line.Alignment || "center"),
        line.Role ? "role-" + line.Role : ""
      ].filter(Boolean).join(" ");
      try {
        node.appendChild(renderQRCodeSVG(qrValue));
      } catch (error) {
        const text = document.createElement("span");
        text.className = "receipt-line-text";
        text.textContent = qrValue;
        node.title = error.message;
        node.appendChild(text);
      }
      paper.appendChild(node);
      continue;
    }
    const imageURL = line.ImageURL || (line.ImageKey ? withAssetVersion("/assets/weather-icons/print/" + encodeURIComponent(line.ImageKey) + ".png") : "");
    if (imageURL) {
      node.className = [
        "receipt-line",
        "receipt-image-line",
        "align-" + (line.Alignment || "center"),
        line.Role ? "role-" + line.Role : ""
      ].filter(Boolean).join(" ");
      const image = document.createElement("img");
      image.className = "receipt-image";
      image.src = imageURL;
      image.alt = line.ImageKey || "receipt image";
      image.title = line.ImageKey || "receipt image";
      const imageWidth = Number.isFinite(line.ImageWidth) && line.ImageWidth > 0 ? line.ImageWidth : 96;
      const imageHeight = Number.isFinite(line.ImageHeight) && line.ImageHeight > 0 ? line.ImageHeight : imageWidth;
      const previewWidth = Math.max(48, Math.min(320, imageWidth * 0.8));
      const previewHeight = Math.max(32, previewWidth * imageHeight / imageWidth);
      image.style.width = previewWidth.toFixed(0) + "px";
      image.style.height = previewHeight.toFixed(0) + "px";
      node.style.setProperty("--image-line-height", (previewHeight + 8).toFixed(0) + "px");
      node.appendChild(image);
      paper.appendChild(node);
      continue;
    }
    const font = Number.isFinite(line.Font) ? line.Font : 0;
    const metric = metricForFont(font);
    const baseWidth = normalMetric.fontWidth || fallbackFontMetrics[0].fontWidth;
    const fontWidth = metric.fontWidth || baseWidth;
    const lineSize = Math.max(10, Math.min(32, 16 * (fontWidth / baseWidth)));
    node.className = [
      "receipt-line",
      "align-" + (line.Alignment || "left"),
      line.Role ? "role-" + line.Role : ""
    ].filter(Boolean).join(" ");
    node.style.setProperty("--line-size", lineSize.toFixed(2) + "px");
    node.style.setProperty("--line-scale-x", line.DoubleWidth ? 2 : 1);
    node.style.setProperty("--line-scale-y", line.DoubleHeight ? 2 : 1);
    node.title = fontLabel(metric) + (line.DoubleWidth ? " · double width" : "") + (line.DoubleHeight ? " · double height" : "");
    const text = document.createElement("span");
    text.className = "receipt-line-text";
    text.textContent = line.Text || " ";
    node.appendChild(text);
    paper.appendChild(node);
  }
  target.replaceChildren(paper);
}

function renderQRCodeSVG(value) {
  const matrix = qrMatrix(value);
  const quietZone = 4;
  const moduleCount = matrix.length;
  const viewSize = moduleCount + quietZone * 2;
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.classList.add("receipt-qr");
  svg.setAttribute("viewBox", "0 0 " + viewSize + " " + viewSize);
  svg.setAttribute("role", "img");
  svg.setAttribute("aria-label", "QR");
  svg.setAttribute("data-value", value);

  const background = document.createElementNS("http://www.w3.org/2000/svg", "rect");
  background.setAttribute("width", String(viewSize));
  background.setAttribute("height", String(viewSize));
  background.setAttribute("fill", "#fff");
  svg.appendChild(background);

  const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
  let data = "";
  for (let row = 0; row < moduleCount; row += 1) {
    for (let col = 0; col < moduleCount; col += 1) {
      if (matrix[row][col]) {
        data += "M" + (col + quietZone) + " " + (row + quietZone) + "h1v1h-1z";
      }
    }
  }
  path.setAttribute("d", data);
  path.setAttribute("fill", "#000");
  svg.appendChild(path);
  return svg;
}

function qrMatrix(value) {
  const version = 2;
  const size = version * 4 + 17;
  const dataCodewordCount = 34;
  const errorCodewordCount = 10;
  const mask = 0;
  const bytes = qrBytes(value);
  if (bytes.length > 32) {
    throw new Error("QR preview supports up to 32 bytes in diagnostic mode.");
  }

  const dataCodewords = qrDataCodewords(bytes, dataCodewordCount);
  const codewords = dataCodewords.concat(qrReedSolomonRemainder(dataCodewords, errorCodewordCount));
  const bits = [];
  for (const codeword of codewords) {
    qrAppendBits(bits, codeword, 8);
  }

  const matrix = Array.from({ length: size }, () => Array(size).fill(false));
  const reserved = Array.from({ length: size }, () => Array(size).fill(false));
  qrAddFunctionPatterns(matrix, reserved, version);
  qrPlaceDataBits(matrix, reserved, bits, mask);
  qrAddFormatBits(matrix, mask);
  return matrix;
}

function qrBytes(value) {
  if (window.TextEncoder) {
    return Array.from(new TextEncoder().encode(value));
  }
  return Array.from(value, char => char.charCodeAt(0) & 0xff);
}

function qrDataCodewords(bytes, codewordCount) {
  const bits = [];
  qrAppendBits(bits, 0x4, 4);
  qrAppendBits(bits, bytes.length, 8);
  for (const byte of bytes) {
    qrAppendBits(bits, byte, 8);
  }
  const capacity = codewordCount * 8;
  qrAppendBits(bits, 0, Math.min(4, capacity - bits.length));
  while (bits.length % 8 !== 0) {
    bits.push(false);
  }
  for (let padIndex = 0; bits.length < capacity; padIndex += 1) {
    qrAppendBits(bits, padIndex % 2 === 0 ? 0xec : 0x11, 8);
  }

  const codewords = [];
  for (let index = 0; index < bits.length; index += 8) {
    let value = 0;
    for (let bit = 0; bit < 8; bit += 1) {
      value = (value << 1) | (bits[index + bit] ? 1 : 0);
    }
    codewords.push(value);
  }
  return codewords;
}

function qrAppendBits(target, value, width) {
  for (let bit = width - 1; bit >= 0; bit -= 1) {
    target.push(((value >>> bit) & 1) !== 0);
  }
}

function qrAddFunctionPatterns(matrix, reserved, version) {
  const size = matrix.length;
  qrAddFinderPattern(matrix, reserved, 0, 0);
  qrAddFinderPattern(matrix, reserved, 0, size - 7);
  qrAddFinderPattern(matrix, reserved, size - 7, 0);
  for (let index = 8; index < size - 8; index += 1) {
    qrSetFunctionModule(matrix, reserved, 6, index, index % 2 === 0);
    qrSetFunctionModule(matrix, reserved, index, 6, index % 2 === 0);
  }
  if (version >= 2) {
    qrAddAlignmentPattern(matrix, reserved, size - 7, size - 7);
  }
  qrSetFunctionModule(matrix, reserved, version * 4 + 9, 8, true);
  qrReserveFormatBits(reserved);
}

function qrAddFinderPattern(matrix, reserved, top, left) {
  for (let row = -1; row <= 7; row += 1) {
    for (let col = -1; col <= 7; col += 1) {
      const targetRow = top + row;
      const targetCol = left + col;
      if (!qrInBounds(matrix, targetRow, targetCol)) {
        continue;
      }
      const dark = row >= 0 && row <= 6 && col >= 0 && col <= 6 &&
        (row === 0 || row === 6 || col === 0 || col === 6 || (row >= 2 && row <= 4 && col >= 2 && col <= 4));
      qrSetFunctionModule(matrix, reserved, targetRow, targetCol, dark);
    }
  }
}

function qrAddAlignmentPattern(matrix, reserved, centerRow, centerCol) {
  for (let row = -2; row <= 2; row += 1) {
    for (let col = -2; col <= 2; col += 1) {
      const dark = Math.max(Math.abs(row), Math.abs(col)) === 2 || (row === 0 && col === 0);
      qrSetFunctionModule(matrix, reserved, centerRow + row, centerCol + col, dark);
    }
  }
}

function qrReserveFormatBits(reserved) {
  const size = reserved.length;
  const reserve = (row, col) => {
    if (row >= 0 && row < size && col >= 0 && col < size) {
      reserved[row][col] = true;
    }
  };
  for (let index = 0; index <= 5; index += 1) {
    reserve(index, 8);
    reserve(8, index);
  }
  reserve(7, 8);
  reserve(8, 8);
  reserve(8, 7);
  for (let index = 0; index < 8; index += 1) {
    reserve(8, size - 1 - index);
  }
  for (let index = 8; index < 15; index += 1) {
    reserve(size - 15 + index, 8);
  }
}

function qrSetFunctionModule(matrix, reserved, row, col, dark) {
  if (!qrInBounds(matrix, row, col)) {
    return;
  }
  matrix[row][col] = dark;
  reserved[row][col] = true;
}

function qrInBounds(matrix, row, col) {
  return row >= 0 && row < matrix.length && col >= 0 && col < matrix.length;
}

function qrPlaceDataBits(matrix, reserved, bits, mask) {
  const size = matrix.length;
  let bitIndex = 0;
  let upward = true;
  for (let rightCol = size - 1; rightCol > 0; rightCol -= 2) {
    if (rightCol === 6) {
      rightCol -= 1;
    }
    for (let rowOffset = 0; rowOffset < size; rowOffset += 1) {
      const row = upward ? size - 1 - rowOffset : rowOffset;
      for (let colOffset = 0; colOffset < 2; colOffset += 1) {
        const col = rightCol - colOffset;
        if (reserved[row][col]) {
          continue;
        }
        let dark = bitIndex < bits.length ? bits[bitIndex] : false;
        if (qrMaskApplies(mask, row, col)) {
          dark = !dark;
        }
        matrix[row][col] = dark;
        bitIndex += 1;
      }
    }
    upward = !upward;
  }
}

function qrMaskApplies(mask, row, col) {
  switch (mask) {
    case 0:
      return (row + col) % 2 === 0;
    case 1:
      return row % 2 === 0;
    case 2:
      return col % 3 === 0;
    case 3:
      return (row + col) % 3 === 0;
    case 4:
      return (Math.floor(row / 2) + Math.floor(col / 3)) % 2 === 0;
    case 5:
      return ((row * col) % 2) + ((row * col) % 3) === 0;
    case 6:
      return (((row * col) % 2) + ((row * col) % 3)) % 2 === 0;
    case 7:
      return (((row + col) % 2) + ((row * col) % 3)) % 2 === 0;
    default:
      return false;
  }
}

function qrAddFormatBits(matrix, mask) {
  const size = matrix.length;
  const bits = qrFormatBits(mask);
  const bit = index => ((bits >>> index) & 1) !== 0;
  for (let index = 0; index <= 5; index += 1) {
    matrix[index][8] = bit(index);
    matrix[8][index] = bit(index);
  }
  matrix[7][8] = bit(6);
  matrix[8][8] = bit(7);
  matrix[8][7] = bit(8);
  for (let index = 9; index < 15; index += 1) {
    matrix[8][14 - index] = bit(index);
  }
  for (let index = 0; index < 8; index += 1) {
    matrix[8][size - 1 - index] = bit(index);
  }
  for (let index = 8; index < 15; index += 1) {
    matrix[size - 15 + index][8] = bit(index);
  }
  matrix[size - 8][8] = true;
}

function qrFormatBits(mask) {
  const errorCorrectionLevelBits = 1;
  const data = (errorCorrectionLevelBits << 3) | mask;
  let remainder = data << 10;
  for (let bit = 14; bit >= 10; bit -= 1) {
    if (((remainder >>> bit) & 1) !== 0) {
      remainder ^= 0x537 << (bit - 10);
    }
  }
  return ((data << 10) | remainder) ^ 0x5412;
}

function qrReedSolomonRemainder(data, degree) {
  const generator = qrReedSolomonGenerator(degree);
  const result = Array(degree).fill(0);
  for (const byte of data) {
    const factor = byte ^ result.shift();
    result.push(0);
    for (let index = 0; index < generator.length; index += 1) {
      result[index] ^= qrGaloisMultiply(generator[index], factor);
    }
  }
  return result;
}

function qrReedSolomonGenerator(degree) {
  let result = [1];
  for (let degreeIndex = 0; degreeIndex < degree; degreeIndex += 1) {
    const next = Array(result.length + 1).fill(0);
    const root = qrGaloisPow(degreeIndex);
    for (let index = 0; index < result.length; index += 1) {
      next[index] ^= result[index];
      next[index + 1] ^= qrGaloisMultiply(result[index], root);
    }
    result = next;
  }
  return result.slice(1);
}

function qrGaloisPow(power) {
  let value = 1;
  for (let index = 0; index < power; index += 1) {
    value = qrGaloisMultiply(value, 2);
  }
  return value;
}

function qrGaloisMultiply(left, right) {
  let result = 0;
  let value = left;
  let multiplier = right;
  while (multiplier > 0) {
    if ((multiplier & 1) !== 0) {
      result ^= value;
    }
    value <<= 1;
    if ((value & 0x100) !== 0) {
      value ^= 0x11d;
    }
    multiplier >>>= 1;
  }
  return result & 0xff;
}

function withAssetVersion(url) {
  return url + (url.includes("?") ? "&" : "?") + "v=" + encodeURIComponent(assetVersion);
}

function styledPreviewLines(lines) {
  return lines.map(line => {
    const next = { ...line };
    switch (next.Role) {
      case "calendar":
        next.Font = readFont("#calendar-font");
        next.DoubleWidth = document.querySelector("#calendar-double-width").checked;
        next.DoubleHeight = document.querySelector("#calendar-double-height").checked;
        next.Alignment = readFontRowAlign("calendar");
        break;
      case "temperature":
        next.Font = readFont("#temperature-font");
        next.DoubleWidth = document.querySelector("#temperature-double-width").checked;
        next.DoubleHeight = document.querySelector("#temperature-double-height").checked;
        next.Alignment = readFontRowAlign("temperature");
        break;
      case "original":
        next.Font = 1;
        next.DoubleWidth = false;
        next.DoubleHeight = false;
        break;
      default:
        next.Font = readFont("#normal-font");
        next.DoubleWidth = false;
        next.DoubleHeight = false;
        next.Alignment = readFontRowAlign("normal");
        break;
    }
    return next;
  });
}

function refreshPreviewStyle() {
  if (lastPreviewLines.length === 0) {
    return;
  }
  renderPreview(styledPreviewLines(lastPreviewLines));
}

// ─── Block-based text print editor ───────────────────────────

function initTextPrintBlocks() {
  const container = document.querySelector("#text-print-blocks");
  if (container && container.children.length === 0) {
    addTextBlock("text");
  }
}

function populateBlockFontSelect(selectEl, selectedFont) {
  const metrics = Array.from(fontMetrics.values()).sort((a, b) => a.font - b.font);
  const visible = metrics.length > 0 ? metrics : fallbackFontMetrics;
  selectEl.replaceChildren(...visible.map(m => {
    const opt = document.createElement("option");
    opt.value = String(m.font);
    opt.textContent = fontLabel(m);
    return opt;
  }));
  selectEl.value = String(selectedFont != null ? selectedFont : 0);
}

function buildTextBlockToolbar(opts) {
  const toolbar = document.createElement("div");
  toolbar.className = "text-block-toolbar";

  const fontSelect = document.createElement("select");
  fontSelect.className = "text-block-font-select";
  fontSelect.dataset.fontSelect = "";
  fontSelect.addEventListener("change", updateTextPrintPreview);
  populateBlockFontSelect(fontSelect, opts.font || 0);

  const alignGroup = document.createElement("div");
  alignGroup.className = "align-btn-group";
  for (const align of ["left", "center", "right"]) {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "align-btn" + ((opts.alignment || "left") === align ? " active" : "");
    btn.dataset.align = align;
    btn.textContent = align === "left" ? "←" : align === "center" ? "≡" : "→";
    btn.addEventListener("click", () => {
      alignGroup.querySelectorAll(".align-btn").forEach(b => b.classList.remove("active"));
      btn.classList.add("active");
      updateTextPrintPreview();
    });
    alignGroup.append(btn);
  }

  const dwLabel = document.createElement("label");
  dwLabel.className = "toggle-label";
  const dwCb = document.createElement("input");
  dwCb.type = "checkbox";
  dwCb.className = "text-block-dw";
  dwCb.checked = Boolean(opts.doubleWidth);
  dwCb.addEventListener("change", updateTextPrintPreview);
  dwLabel.append(dwCb, " 2W");

  const dhLabel = document.createElement("label");
  dhLabel.className = "toggle-label";
  const dhCb = document.createElement("input");
  dhCb.type = "checkbox";
  dhCb.className = "text-block-dh";
  dhCb.checked = Boolean(opts.doubleHeight);
  dhCb.addEventListener("change", updateTextPrintPreview);
  dhLabel.append(dhCb, " 2H");

  toolbar.append(fontSelect, alignGroup, dwLabel, dhLabel);
  return toolbar;
}

function createTextBlock(type, opts = {}) {
  const block = document.createElement("div");
  block.className = "text-block text-block-" + type;

  const top = document.createElement("div");
  top.className = "text-block-top";

  const deleteBtn = document.createElement("button");
  deleteBtn.type = "button";
  deleteBtn.className = "text-block-delete secondary";
  deleteBtn.textContent = "×";
  deleteBtn.addEventListener("click", () => {
    block.remove();
    updateTextPrintPreview();
  });

  if (type === "text") {
    const textarea = document.createElement("textarea");
    textarea.className = "text-block-textarea";
    textarea.rows = 2;
    textarea.placeholder = "Текст...";
    textarea.value = opts.text || "";
    textarea.addEventListener("input", updateTextPrintPreview);
    top.append(textarea, deleteBtn);
    block.append(top, buildTextBlockToolbar(opts));
  } else {
    const label = document.createElement("span");
    label.className = "text-block-label";
    label.textContent = type === "separator" ? "--------------------------------" : "Пустая строка";
    top.append(label, deleteBtn);
    block.append(top);
  }
  return block;
}

function addTextBlock(type, opts = {}) {
  const container = document.querySelector("#text-print-blocks");
  if (!container) return;
  container.append(createTextBlock(type, opts));
  updateTextPrintPreview();
}

function textPrintPayload() {
  const blocks = [];
  for (const block of document.querySelectorAll("#text-print-blocks .text-block")) {
    if (block.classList.contains("text-block-separator")) {
      blocks.push({ text: "--------------------------------", font: 0, alignment: "center", doubleWidth: false, doubleHeight: false });
    } else if (block.classList.contains("text-block-empty")) {
      blocks.push({ text: "", font: 0, alignment: "left", doubleWidth: false, doubleHeight: false });
    } else {
      const fontSelect = block.querySelector(".text-block-font-select");
      const activeAlign = block.querySelector(".align-btn.active");
      const dw = block.querySelector(".text-block-dw");
      const dh = block.querySelector(".text-block-dh");
      blocks.push({
        text: (block.querySelector(".text-block-textarea") || {}).value || "",
        font: fontSelect ? (parseInt(fontSelect.value, 10) || 0) : 0,
        alignment: activeAlign ? activeAlign.dataset.align : "left",
        doubleWidth: dw ? dw.checked : false,
        doubleHeight: dh ? dh.checked : false,
      });
    }
  }
  return { blocks };
}

function textPrintRuneLength(value) {
  return Array.from(String(value || "")).length;
}

function textPrintRuneSlice(value, start, end) {
  return Array.from(String(value || "")).slice(start, end).join("");
}

function effectiveTextPrintLineLength(font, doubleWidth) {
  const metric = metricForFont(font);
  const lineLength = metric.lineLength || fallbackFontMetrics[0].lineLength || 32;
  const scale = doubleWidth ? 2 : 1;
  return Math.max(1, Math.floor(lineLength / scale));
}

function wrapTextPrintLine(lineText, limit) {
  const text = String(lineText || "");
  if (text === "" || textPrintRuneLength(text) <= limit) {
    return [text];
  }

  const result = [];
  let current = "";
  for (const word of text.trim().split(/\s+/)) {
    if (current === "") {
      current = word;
    } else if (textPrintRuneLength(current) + 1 + textPrintRuneLength(word) <= limit) {
      current += " " + word;
    } else {
      result.push(current);
      current = word;
    }

    while (textPrintRuneLength(current) > limit) {
      result.push(textPrintRuneSlice(current, 0, limit));
      current = textPrintRuneSlice(current, limit);
    }
  }
  if (current !== "") {
    result.push(current);
  }
  return result.length > 0 ? result : [text];
}

function textPrintPreviewLines(payload = textPrintPayload()) {
  const lines = [];
  for (const block of payload.blocks) {
    const font = metricForFont(block.font);
    const text = (block.text || "").replace(/\r\n/g, "\n").replace(/\r/g, "\n");
    for (const lineText of text.split("\n")) {
      for (const wrappedLineText of wrapTextPrintLine(lineText, effectiveTextPrintLineLength(block.font, block.doubleWidth))) {
        lines.push({
          Text: wrappedLineText,
          Alignment: block.alignment || "left",
          Role: "normal",
          Font: font,
          DoubleWidth: block.doubleWidth,
          DoubleHeight: block.doubleHeight,
        });
      }
    }
  }
  return lines;
}

function updateTextPrintPreview() {
  renderLinesPreview(textPrintPreviewEl, textPrintPreviewLines(), null);
}

function clearTextPrint() {
  const container = document.querySelector("#text-print-blocks");
  if (container) container.replaceChildren();
  addTextBlock("text");
  updateTextPrintPreview();
  setStatus("", "");
}

async function printText() {
  const payload = textPrintPayload();
  const hasText = payload.blocks && payload.blocks.some(b => (b.text || "").trim() !== "");
  if (!hasText) {
    setStatus("error", "Добавь хотя бы один блок с текстом.");
    return;
  }
  updateTextPrintPreview();
  setBusy(true);
  setStatus("", "Печатаю текст...");
  try {
    await saveSettings();
    const response = await fetch("/api/print/text", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload)
    });
    const result = await response.json();
    if (!response.ok || !result.ok) {
      throw new Error(result.error || "Не удалось напечатать текст.");
    }
    setStatus("ok", result.message || "Текст напечатан.");
  } catch (error) {
    setStatus("error", error.message);
  } finally {
    setBusy(false);
  }
}

async function runAction(path, successPrefix) {
  setBusy(true);
  setStatus("", "Работаю...");
  try {
    await saveAllSettings();
    const response = await fetch(path, { method: "POST" });
    const payload = await response.json();
    if (!response.ok || !payload.ok) {
      throw new Error(payload.error || "Операция не выполнена.");
    }
    setStatus("ok", successPrefix ? successPrefix + "\n" + payload.message : payload.message);
  } catch (error) {
    setStatus("error", error.message);
  } finally {
    setBusy(false);
  }
}

async function previewReceipt() {
  setBusy(true);
  setStatus("", "Собираю превью...");
  try {
    await saveAllSettings();
    const response = await fetch("/api/receipt/preview", { method: "POST" });
    const payload = await response.json();
    if (!response.ok || !payload.ok) {
      throw new Error(payload.error || "Не удалось собрать превью.");
    }
    lastPreviewLines = (payload.lines || []).map(line => ({ ...line }));
    renderPreview(styledPreviewLines(lastPreviewLines));
    setStatus(payload.warnings && payload.warnings.length > 0 ? "" : "ok", payload.message || "Превью собрано.");
  } catch (error) {
    setStatus("error", error.message);
  } finally {
    setBusy(false);
  }
}

async function loadFontMetrics() {
  setBusy(true);
  setStatus("", "Читаю параметры шрифтов ККТ...");
  try {
    await saveSettings();
    const response = await fetch("/api/printer/fonts", { method: "POST" });
    const payload = await response.json();
    if (!response.ok || !payload.ok) {
      throw new Error(payload.error || "Не удалось загрузить параметры шрифтов.");
    }
    if (!payload.fonts || payload.fonts.length === 0) {
      throw new Error("ККТ не вернула параметры шрифтов.");
    }
    fontMetrics = new Map(payload.fonts.map(metric => [metric.font, metric]));
    updateFontControls();
    refreshPreviewStyle();
    updateTextPrintPreview();
    setStatus("ok", payload.message || "Метрики шрифтов ККТ загружены.");
  } catch (error) {
    setStatus("error", error.message);
  } finally {
    setBusy(false);
  }
}

function bindEventListeners() {
  document.querySelector('[data-action="check"]').addEventListener("click", () => {
    runAction("/api/printer/check", "");
  });
  document.querySelector('[data-action="fonts"]').addEventListener("click", () => {
    loadFontMetrics();
  });
  document.querySelector('[data-action="search-weather-location"]').addEventListener("click", () => {
    searchWeatherLocations().catch(error => {
      setStatus("error", error.message);
    });
  });
  document.querySelector('[data-action="test-motivation"]').addEventListener("click", () => {
    testMotivation();
  });
  document.querySelector('[data-action="google-disconnect"]').addEventListener("click", () => {
    disconnectGoogle();
  });
  weatherNameInput.addEventListener("input", queueWeatherLocationSearch);
  weatherLocationResultsEl.addEventListener("click", event => {
    const button = event.target.closest(".location-result");
    if (!button) {
      return;
    }
    weatherNameInput.value = button.dataset.locationName || button.textContent.trim();
    weatherLatitudeInput.value = button.dataset.locationLatitude || "";
    weatherLongitudeInput.value = button.dataset.locationLongitude || "";
    weatherLocationResultsEl.replaceChildren();
    weatherLocationHelpEl.textContent = "Координаты обновлены из выбранного города.";
    setWeatherLocationSelected(true);
  });
  document.querySelector('[data-action="edit-finance"]').addEventListener("click", () => {
    financeDraft = {
      amountTon: document.querySelector("#ton-amount").value,
      investedUsd: document.querySelector("#ton-invested").value
    };
    setFinanceEditing(true);
  });
  document.querySelector('[data-action="save-finance"]').addEventListener("click", () => {
    saveFinanceExplicitly();
  });
  document.querySelector('[data-action="cancel-finance"]').addEventListener("click", () => {
    cancelFinanceEditing();
  });
  document.querySelector('[data-action="print"]').addEventListener("click", () => {
    runAction("/api/print/test", "");
  });
  document.querySelector('[data-action="preview"]').addEventListener("click", () => {
    previewReceipt();
  });
  document.querySelector('[data-action="weather"]').addEventListener("click", () => {
    runAction("/api/print/weather", "");
  });
  document.querySelector('[data-action="print-text"]').addEventListener("click", () => {
    printText();
  });
  document.querySelector('[data-action="clear-text-print"]').addEventListener("click", () => {
    clearTextPrint();
  });
  document.querySelector('[data-action="add-text-block"]').addEventListener("click", () => {
    addTextBlock("text");
  });
  document.querySelector('[data-action="add-separator-block"]').addEventListener("click", () => {
    addTextBlock("separator");
  });
  document.querySelector('[data-action="add-empty-block"]').addEventListener("click", () => {
    addTextBlock("empty");
  });
  imageEditorFileInput.addEventListener("change", event => {
    loadImageEditorFile(event.target.files && event.target.files[0]).catch(error => {
      setImageEditorStatus("error", error.message);
      setStatus("error", error.message);
    });
  });
  [
    "#image-editor-rotation",
    "#image-editor-brightness",
    "#image-editor-contrast",
    "#image-editor-threshold",
    "#image-editor-dither",
    "#image-editor-invert"
  ].forEach(selector => {
    document.querySelector(selector).addEventListener("input", applyImageEditorProcessing);
    document.querySelector(selector).addEventListener("change", applyImageEditorProcessing);
  });
  imageEditorResultCanvas.addEventListener("pointerdown", event => {
    imageEditorResultCanvas.setPointerCapture(event.pointerId);
    beginImageEditorPointer(event);
  });
  imageEditorResultCanvas.addEventListener("pointermove", event => {
    moveImageEditorPointer(event);
  });
  imageEditorResultCanvas.addEventListener("pointerup", event => {
    endImageEditorPointer(event);
    imageEditorResultCanvas.releasePointerCapture(event.pointerId);
  });
  imageEditorResultCanvas.addEventListener("pointercancel", () => {
    imageEditorPointerDown = false;
    imageEditorShapeStart = null;
    imageEditorShapeBasePixels = null;
  });
  document.querySelector('[data-action="new-image-editor-canvas"]').addEventListener("click", () => {
    createBlankImageEditorCanvas(imageEditorCanvasHeight());
  });
  document.querySelector('[data-action="save-image-editor"]').addEventListener("click", () => {
    saveImageEditor();
  });
  document.querySelector('[data-action="print-image-editor"]').addEventListener("click", () => {
    printImageEditor();
  });
  document.querySelector('[data-action="clear-image-editor"]').addEventListener("click", () => {
    clearImageEditor();
  });
  document.querySelector("#news-source-list").addEventListener("input", clearNewsCountValidation);
  document.querySelector("#news-source-list").addEventListener("change", clearNewsCountValidation);
  document.querySelector("#denis-trends-settings").addEventListener("input", clearNewsCountValidation);
  document.querySelector("#denis-trends-settings").addEventListener("change", clearNewsCountValidation);
  document.querySelector("#news-translate").addEventListener("input", clearNewsCountValidation);
  document.querySelector("#news-translate").addEventListener("change", clearNewsCountValidation);
  document.querySelector('[data-action="save-schedule"]').addEventListener("click", () => {
    saveScheduleSettings();
  });
  document.querySelector('[data-action="stop-schedule"]').addEventListener("click", () => {
    stopSchedule();
  });
  document.querySelector('[data-action="add-schedule-time"]').addEventListener("click", () => {
    addScheduleTime("");
  });
  document.querySelector("#schedule-time-list").addEventListener("click", event => {
    const button = event.target.closest('[data-action="remove-schedule-time"]');
    if (!button) {
      return;
    }
    const row = button.closest("[data-schedule-time-row]");
    if (row) {
      row.remove();
    }
    if (document.querySelectorAll("[data-schedule-time-row]").length === 0) {
      addScheduleTime("");
    }
  });
  document.querySelector("#schedule-time-list").addEventListener("change", event => {
    const row = event.target.closest("[data-schedule-time-row]");
    if (row && event.target.matches("[data-schedule-content-key]")) {
      updateScheduleRowSummary(row);
    }
  });
  document.querySelectorAll('input[name="schedule-mode"]').forEach(input => {
    input.addEventListener("change", updateScheduleMode);
  });

  [
    "#calendar-font",
    "#calendar-double-width",
    "#calendar-double-height",
    "#temperature-font",
    "#temperature-double-width",
    "#temperature-double-height",
    "#normal-font"
  ].forEach(selector => {
    const el = document.querySelector(selector);
    const card = el.closest(".font-row-card[data-row]");
    const handler = () => { refreshPreviewStyle(); if (card) renderFontRowPreview(card); };
    el.addEventListener("input", handler);
    el.addEventListener("change", handler);
  });
}

function clearNewsCountValidation(event) {
  const input = event.target;
  const row = input.closest("[data-news-source]") || input.closest("[data-denis-trend-period]");
  const countInput = row?.querySelector("[data-news-count]") || row?.querySelector("[data-denis-trend-period-count]");
  countInput?.classList.remove("invalid");
  countInput?.removeAttribute("aria-invalid");
}

async function initializeApp() {
  const bootstrap = await loadBootstrap();
  applyBootstrap(bootstrap);
  bindEventListeners();
  updateFontControls();
  updateTextPrintPreview();
  updateScheduleMode();
  loadImageEditorState().catch(error => {
    setImageEditorStatus("error", error.message);
  });
  loadSchedulerStatus();
  loadGoogleStatus().catch(error => {
    setGoogleStatus("error", error.message);
  });
}

initializeApp().catch(error => {
  setStatus("error", error.message);
});

// Tab navigation
(function () {
  const tabBtns = document.querySelectorAll("[data-tab-target]");
  const tabPanels = document.querySelectorAll("[data-tab-panel]");

  tabBtns.forEach(btn => {
    btn.addEventListener("click", () => {
      tabPanels.forEach(p => { p.hidden = true; });
      tabBtns.forEach(b => b.classList.remove("active"));
      const target = document.querySelector(btn.dataset.tabTarget);
      if (target) target.hidden = false;
      btn.classList.add("active");
    });
  });

  if (tabBtns.length) tabBtns[0].click();
}());

// Font row alignment buttons
document.querySelectorAll(".font-align-group .align-btn").forEach(btn => {
  btn.addEventListener("click", () => {
    btn.closest(".font-align-group").querySelectorAll(".align-btn")
      .forEach(b => b.classList.remove("active"));
    btn.classList.add("active");
    refreshPreviewStyle();
    const card = btn.closest(".font-row-card[data-row]");
    if (card) renderFontRowPreview(card);
  });
});
