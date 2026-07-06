import { apiFetch } from "./api.js";
import { escapeHtml } from "./dom-utils.js";
import { sparkline } from "./sparkline.js";

const listEl = document.getElementById("item-list");
const emptyEl = document.getElementById("empty-state");
const modalBackdrop = document.getElementById("modal-backdrop");
const viewTabs = document.getElementById("view-tabs");
const heroDateEl = document.getElementById("hero-date");
const heroStatsEl = document.getElementById("hero-stats");
const weightTrendEl = document.getElementById("weight-trend");
const newItemBtn = document.getElementById("new-item");

const bodyForm = document.getElementById("body-form");
const sleepForm = document.getElementById("sleep-form");
const moodForm = document.getElementById("mood-form");
const allForms = [bodyForm, sleepForm, moodForm];

const state = { view: "body" };
let currentBodyMetrics = [];
let currentSleepLogs = [];
let currentMoodLogs = [];
let currentHabits = [];
let editingBodyMetricId = null;
let editingSleepLogId = null;
let editingMoodLogId = null;

function reportError(err, action = "saving") {
  console.error(err);
  alert(`Something went wrong ${action} that. Check your connection and try again.`);
}

function localDateStr(d) {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

function round1(n) {
  return Math.round(n * 10) / 10;
}

function textOrNull(value) {
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}

function renderHero() {
  heroDateEl.textContent = new Date().toLocaleDateString(undefined, {
    weekday: "long",
    month: "long",
    day: "numeric",
  });
  const parts = [];
  const latest = currentBodyMetrics[0];
  parts.push(latest ? `Last weigh-in: ${round1(latest.weight)} on ${latest.log_date}` : "No weigh-ins yet");
  const bestStreak = currentHabits.reduce((max, h) => Math.max(max, h.streak), 0);
  const pill = bestStreak > 0 ? `<span class="pill pill-accent">&#128293; ${bestStreak}d best streak</span>` : "";
  heroStatsEl.innerHTML = `<span>${parts.join(" &middot; ")}</span>${pill}`;
}

function renderWeightTrend() {
  if (state.view !== "body") {
    weightTrendEl.classList.add("hidden");
    return;
  }
  const points = [...currentBodyMetrics].slice(0, 30).reverse().map((m) => ({ date: m.log_date, value: m.weight }));
  weightTrendEl.innerHTML = sparkline(points);
  weightTrendEl.classList.remove("hidden");
}

// --- Body tab ---

function bodyRow(m) {
  const li = document.createElement("li");
  li.className = "task task-manage";
  li.dataset.id = m.id;
  li.innerHTML = `
    <div class="body">
      <div class="title">${round1(m.weight)}</div>
      <div class="meta">${m.log_date}${m.notes ? ` &middot; ${escapeHtml(m.notes)}` : ""}</div>
    </div>
    <span class="chevron" aria-hidden="true">&rsaquo;</span>
  `;
  return li;
}

async function loadBodyMetrics() {
  currentBodyMetrics = await apiFetch("/api/wellness/body-metrics");
  renderHero();
  renderWeightTrend();
  emptyEl.textContent = "No weigh-ins yet. Tap + to log one.";
  emptyEl.classList.toggle("hidden", currentBodyMetrics.length > 0);
  listEl.innerHTML = "";
  for (const m of currentBodyMetrics) listEl.appendChild(bodyRow(m));
}

// --- Sleep tab ---

function sleepRow(l) {
  const li = document.createElement("li");
  li.className = "task task-manage";
  li.dataset.id = l.id;
  const quality = l.quality != null ? ` &middot; quality ${l.quality}/5` : "";
  li.innerHTML = `
    <div class="body">
      <div class="title">${round1(l.hours)}h</div>
      <div class="meta">${l.log_date}${quality}${l.notes ? ` &middot; ${escapeHtml(l.notes)}` : ""}</div>
    </div>
    <span class="chevron" aria-hidden="true">&rsaquo;</span>
  `;
  return li;
}

async function loadSleepLogs() {
  currentSleepLogs = await apiFetch("/api/wellness/sleep");
  emptyEl.textContent = "No sleep logs yet. Tap + to log one.";
  emptyEl.classList.toggle("hidden", currentSleepLogs.length > 0);
  listEl.innerHTML = "";
  for (const l of currentSleepLogs) listEl.appendChild(sleepRow(l));
}

// --- Mood tab ---

function moodRow(m) {
  const li = document.createElement("li");
  li.className = "task task-manage";
  li.dataset.id = m.id;
  li.innerHTML = `
    <div class="body">
      <div class="title">Mood ${m.mood}/5</div>
      <div class="meta">${m.log_date}${m.notes ? ` &middot; ${escapeHtml(m.notes)}` : ""}</div>
    </div>
    <span class="chevron" aria-hidden="true">&rsaquo;</span>
  `;
  return li;
}

async function loadMoodLogs() {
  currentMoodLogs = await apiFetch("/api/wellness/mood");
  emptyEl.textContent = "No mood logs yet. Tap + to log one.";
  emptyEl.classList.toggle("hidden", currentMoodLogs.length > 0);
  listEl.innerHTML = "";
  for (const m of currentMoodLogs) listEl.appendChild(moodRow(m));
}

// --- Habits tab (reuses recurring tasks, read-only here) ---

function habitRow(h) {
  const li = document.createElement("li");
  li.className = "task";
  li.dataset.id = h.id;
  const streakPill = h.streak > 0 ? `<span class="pill pill-accent">&#128293; ${h.streak}</span>` : "";
  li.innerHTML = `
    <input type="checkbox" class="toggle" data-id="${h.id}" ${h.completed_today ? "checked" : ""} />
    <div class="body">
      <div class="title">${escapeHtml(h.title)}</div>
      <div class="meta">${h.category ? escapeHtml(h.category) : ""}</div>
    </div>
    ${streakPill}
  `;
  return li;
}

async function loadHabits() {
  currentHabits = await apiFetch("/api/tasks/habits");
  renderHero();
  emptyEl.textContent = "No habits yet. Create a recurring task from the Tasks page to see it here.";
  emptyEl.classList.toggle("hidden", currentHabits.length > 0);
  listEl.innerHTML = "";
  for (const h of currentHabits) listEl.appendChild(habitRow(h));
}

async function loadView() {
  try {
    if (state.view === "body") await loadBodyMetrics();
    else if (state.view === "sleep") {
      renderWeightTrend();
      await loadSleepLogs();
    } else if (state.view === "mood") {
      renderWeightTrend();
      await loadMoodLogs();
    } else {
      renderWeightTrend();
      await loadHabits();
    }
  } catch (err) {
    console.error(err);
    emptyEl.textContent = "Couldn't load that. Check your connection and reload the page.";
    emptyEl.classList.remove("hidden");
  }
}

viewTabs.addEventListener("click", async (e) => {
  const btn = e.target.closest("button[data-view]");
  if (!btn) return;
  state.view = btn.dataset.view;
  for (const tab of viewTabs.querySelectorAll("button")) tab.classList.toggle("active", tab === btn);
  newItemBtn.classList.toggle("hidden", state.view === "habits");
  await loadView();
});

function showModal(form) {
  for (const f of allForms) f.classList.toggle("hidden", f !== form);
  modalBackdrop.classList.remove("hidden");
}

function closeModal() {
  modalBackdrop.classList.add("hidden");
  for (const f of allForms) f.reset();
  editingBodyMetricId = null;
  editingSleepLogId = null;
  editingMoodLogId = null;
}

document.querySelectorAll(".cancel-modal").forEach((btn) => btn.addEventListener("click", closeModal));
modalBackdrop.addEventListener("click", (e) => {
  if (e.target === modalBackdrop) closeModal();
});

newItemBtn.addEventListener("click", () => {
  if (state.view === "body") openBodyModal(null);
  else if (state.view === "sleep") openSleepModal(null);
  else if (state.view === "mood") openMoodModal(null);
});

listEl.addEventListener("click", async (e) => {
  const toggle = e.target.closest(".toggle");
  if (toggle && state.view === "habits") {
    const habit = currentHabits.find((h) => h.id === toggle.dataset.id);
    if (!habit) return;
    const today = localDateStr(new Date());
    try {
      await apiFetch(`/api/tasks/${habit.id}/${toggle.checked ? "complete" : "uncomplete"}`, {
        method: "POST",
        body: JSON.stringify({ occurrence_date: today }),
      });
      await loadHabits();
    } catch (err) {
      toggle.checked = !toggle.checked;
      reportError(err);
    }
    return;
  }

  const row = e.target.closest(".task-manage");
  if (!row) return;
  if (state.view === "body") {
    const m = currentBodyMetrics.find((item) => item.id === row.dataset.id);
    if (m) openBodyModal(m);
  } else if (state.view === "sleep") {
    const l = currentSleepLogs.find((item) => item.id === row.dataset.id);
    if (l) openSleepModal(l);
  } else if (state.view === "mood") {
    const m = currentMoodLogs.find((item) => item.id === row.dataset.id);
    if (m) openMoodModal(m);
  }
});

// --- Body form ---

function openBodyModal(m) {
  editingBodyMetricId = m?.id ?? null;
  document.getElementById("body-modal-title").textContent = editingBodyMetricId ? "Edit weigh-in" : "New weigh-in";
  bodyForm.weight.value = m?.weight ?? "";
  bodyForm.log_date.value = m?.log_date ?? localDateStr(new Date());
  bodyForm.notes.value = m?.notes ?? "";
  const deleteBtn = document.getElementById("delete-body-metric");
  deleteBtn.classList.toggle("hidden", !editingBodyMetricId);
  deleteBtn.textContent = "Delete";
  deleteBtn.classList.remove("confirm");
  showModal(bodyForm);
}

bodyForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const payload = {
    weight: Number(bodyForm.weight.value) || 0,
    log_date: bodyForm.log_date.value,
    notes: textOrNull(bodyForm.notes.value),
  };
  try {
    if (editingBodyMetricId) {
      await apiFetch(`/api/wellness/body-metrics/${editingBodyMetricId}`, { method: "PUT", body: JSON.stringify(payload) });
    } else {
      await apiFetch("/api/wellness/body-metrics", { method: "POST", body: JSON.stringify(payload) });
    }
    closeModal();
    await loadBodyMetrics();
  } catch (err) {
    reportError(err);
  }
});

document.getElementById("delete-body-metric").addEventListener("click", async () => {
  const btn = document.getElementById("delete-body-metric");
  if (!editingBodyMetricId) return;
  if (!btn.classList.contains("confirm")) {
    btn.textContent = "Tap again to delete";
    btn.classList.add("confirm");
    return;
  }
  try {
    await apiFetch(`/api/wellness/body-metrics/${editingBodyMetricId}`, { method: "DELETE" });
    closeModal();
    await loadBodyMetrics();
  } catch (err) {
    reportError(err, "deleting");
  }
});

// --- Sleep form ---

function openSleepModal(l) {
  editingSleepLogId = l?.id ?? null;
  document.getElementById("sleep-modal-title").textContent = editingSleepLogId ? "Edit sleep log" : "New sleep log";
  sleepForm.hours.value = l?.hours ?? "";
  sleepForm.quality.value = l?.quality ?? "";
  sleepForm.log_date.value = l?.log_date ?? localDateStr(new Date());
  sleepForm.notes.value = l?.notes ?? "";
  const deleteBtn = document.getElementById("delete-sleep-log");
  deleteBtn.classList.toggle("hidden", !editingSleepLogId);
  deleteBtn.textContent = "Delete";
  deleteBtn.classList.remove("confirm");
  showModal(sleepForm);
}

sleepForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const payload = {
    hours: Number(sleepForm.hours.value) || 0,
    quality: sleepForm.quality.value === "" ? null : Number(sleepForm.quality.value),
    log_date: sleepForm.log_date.value,
    notes: textOrNull(sleepForm.notes.value),
  };
  try {
    if (editingSleepLogId) {
      await apiFetch(`/api/wellness/sleep/${editingSleepLogId}`, { method: "PUT", body: JSON.stringify(payload) });
    } else {
      await apiFetch("/api/wellness/sleep", { method: "POST", body: JSON.stringify(payload) });
    }
    closeModal();
    await loadSleepLogs();
  } catch (err) {
    reportError(err);
  }
});

document.getElementById("delete-sleep-log").addEventListener("click", async () => {
  const btn = document.getElementById("delete-sleep-log");
  if (!editingSleepLogId) return;
  if (!btn.classList.contains("confirm")) {
    btn.textContent = "Tap again to delete";
    btn.classList.add("confirm");
    return;
  }
  try {
    await apiFetch(`/api/wellness/sleep/${editingSleepLogId}`, { method: "DELETE" });
    closeModal();
    await loadSleepLogs();
  } catch (err) {
    reportError(err, "deleting");
  }
});

// --- Mood form ---

function openMoodModal(m) {
  editingMoodLogId = m?.id ?? null;
  document.getElementById("mood-modal-title").textContent = editingMoodLogId ? "Edit mood log" : "New mood log";
  moodForm.mood.value = m?.mood ?? "3";
  moodForm.log_date.value = m?.log_date ?? localDateStr(new Date());
  moodForm.notes.value = m?.notes ?? "";
  const deleteBtn = document.getElementById("delete-mood-log");
  deleteBtn.classList.toggle("hidden", !editingMoodLogId);
  deleteBtn.textContent = "Delete";
  deleteBtn.classList.remove("confirm");
  showModal(moodForm);
}

moodForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const payload = {
    mood: Number(moodForm.mood.value),
    log_date: moodForm.log_date.value,
    notes: textOrNull(moodForm.notes.value),
  };
  try {
    if (editingMoodLogId) {
      await apiFetch(`/api/wellness/mood/${editingMoodLogId}`, { method: "PUT", body: JSON.stringify(payload) });
    } else {
      await apiFetch("/api/wellness/mood", { method: "POST", body: JSON.stringify(payload) });
    }
    closeModal();
    await loadMoodLogs();
  } catch (err) {
    reportError(err);
  }
});

document.getElementById("delete-mood-log").addEventListener("click", async () => {
  const btn = document.getElementById("delete-mood-log");
  if (!editingMoodLogId) return;
  if (!btn.classList.contains("confirm")) {
    btn.textContent = "Tap again to delete";
    btn.classList.add("confirm");
    return;
  }
  try {
    await apiFetch(`/api/wellness/mood/${editingMoodLogId}`, { method: "DELETE" });
    closeModal();
    await loadMoodLogs();
  } catch (err) {
    reportError(err, "deleting");
  }
});

// --- Bootstrap ---

try {
  currentBodyMetrics = await apiFetch("/api/wellness/body-metrics");
  currentHabits = await apiFetch("/api/tasks/habits");
  renderHero();
  await loadView();
} catch (err) {
  console.error(err);
  emptyEl.textContent = "Couldn't load your wellness data. Check your connection and reload the page.";
  emptyEl.classList.remove("hidden");
}
