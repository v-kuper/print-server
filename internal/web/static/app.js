const statusEl = document.querySelector("#status");
const previewEl = document.querySelector("#receipt-preview");
const fontMetricsEl = document.querySelector("#font-metrics");
const schedulerStatusEl = document.querySelector("#scheduler-status");
const motivationStatusEl = document.querySelector("#motivation-status");
const googleStatusEl = document.querySelector("#google-status");
const googleCredentialsPathEl = document.querySelector("#google-credentials-path");
const googleTokenPathEl = document.querySelector("#google-token-path");
const newsSourceListEl = document.querySelector("#news-source-list");
const weatherNameInput = document.querySelector("#weather-name");
const weatherLatitudeInput = document.querySelector("#weather-latitude");
const weatherLongitudeInput = document.querySelector("#weather-longitude");
const weatherLocationResultsEl = document.querySelector("#weather-location-results");
const weatherLocationHelpEl = document.querySelector("#weather-location-help");
let lastPreviewLines = [];
let weatherSearchTimer = null;
let financeDraft = {
  amountTon: document.querySelector("#ton-amount").value,
  investedUsd: document.querySelector("#ton-invested").value
};
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

function renderScheduleTimes(times) {
  const list = document.querySelector("#schedule-time-list");
  if (!list) {
    return;
  }
  list.replaceChildren();
  const values = Array.isArray(times) && times.length > 0 ? times : [""];
  values.forEach(value => addScheduleTime(value));
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
  setChecked("#content-news", content.showNews);

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
  setValue("#temperature-font", style.temperatureFont || 0);
  setChecked("#temperature-double-width", style.temperatureDoubleWidth);
  setChecked("#temperature-double-height", style.temperatureDoubleHeight);
  setValue("#normal-font", style.normalFont || 0);

  const news = data.news || {};
  setChecked("#news-translate", news.translateTitles);
  renderNewsSources(news, data.newsPresets || []);

  const schedule = data.schedule || {};
  setScheduleMode(schedule.mode);
  renderScheduleIntervals(data.scheduleIntervals || [], schedule.intervalMinutes);
  renderScheduleTimes(schedule.times || []);

  renderGoogleStatus(data.googleStatus || {});
}

function setBusy(busy) {
  document.querySelectorAll("button").forEach(button => button.disabled = busy);
}

function setStatus(kind, text) {
  statusEl.className = kind;
  statusEl.textContent = text;
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

function readFont(selector) {
  const value = Number.parseInt(document.querySelector(selector).value, 10);
  return Number.isFinite(value) ? value : 0;
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
      temperatureDoubleHeight: document.querySelector("#temperature-double-height").checked
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
    showNews: checked("#content-news")
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

function addScheduleTime(value = "07:00") {
  const row = document.createElement("div");
  row.className = "schedule-time-row";
  row.dataset.scheduleTimeRow = "";
  const input = document.createElement("input");
  input.type = "time";
  input.dataset.scheduleTime = "";
  input.value = value;
  const remove = document.createElement("button");
  remove.type = "button";
  remove.className = "secondary";
  remove.dataset.action = "remove-schedule-time";
  remove.textContent = "Удалить";
  row.append(input, remove);
  document.querySelector("#schedule-time-list").appendChild(row);
}

function readScheduleSettings(enabled) {
  const interval = Number.parseInt(document.querySelector("#schedule-interval").value, 10);
  const times = Array.from(document.querySelectorAll("[data-schedule-time]"))
    .map(input => input.value.trim())
    .filter(Boolean);
  return {
    enabled,
    mode: scheduleMode(),
    intervalMinutes: Number.isFinite(interval) ? interval : 15,
    times,
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

function renderPreview(lines) {
  const paper = document.createElement("div");
  paper.className = "receipt-paper";
  const normalMetric = metricForFont(readFont("#normal-font"));
  paper.style.setProperty("--paper-chars", normalMetric.lineLength || 32);
  for (const line of lines) {
    const node = document.createElement("div");
    const imageURL = line.ImageURL || (line.ImageKey ? "/assets/weather-icons/print/" + encodeURIComponent(line.ImageKey) + ".png" : "");
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
  previewEl.replaceChildren(paper);
}

function styledPreviewLines(lines) {
  return lines.map(line => {
    const next = { ...line };
    switch (next.Role) {
      case "calendar":
        next.Font = readFont("#calendar-font");
        next.DoubleWidth = document.querySelector("#calendar-double-width").checked;
        next.DoubleHeight = document.querySelector("#calendar-double-height").checked;
        break;
      case "temperature":
        next.Font = readFont("#temperature-font");
        next.DoubleWidth = document.querySelector("#temperature-double-width").checked;
        next.DoubleHeight = document.querySelector("#temperature-double-height").checked;
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
  document.querySelector("#news-source-list").addEventListener("input", clearNewsCountValidation);
  document.querySelector("#news-source-list").addEventListener("change", clearNewsCountValidation);
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
    document.querySelector(selector).addEventListener("input", refreshPreviewStyle);
    document.querySelector(selector).addEventListener("change", refreshPreviewStyle);
  });
}

function clearNewsCountValidation(event) {
  const input = event.target;
  const row = input.closest("[data-news-source]");
  const countInput = row?.querySelector("[data-news-count]");
  countInput?.classList.remove("invalid");
  countInput?.removeAttribute("aria-invalid");
}

async function initializeApp() {
  const bootstrap = await loadBootstrap();
  applyBootstrap(bootstrap);
  bindEventListeners();
  updateFontControls();
  updateScheduleMode();
  loadSchedulerStatus();
  loadGoogleStatus().catch(error => {
    setGoogleStatus("error", error.message);
  });
}

initializeApp().catch(error => {
  setStatus("error", error.message);
});
