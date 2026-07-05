import { apiFetch } from "./api.js";

const listEl = document.getElementById("task-list");
const emptyEl = document.getElementById("empty-state");
const modalBackdrop = document.getElementById("modal-backdrop");
const form = document.getElementById("task-form");
const deleteBtn = document.getElementById("delete-task");
const modalTitle = document.getElementById("modal-title");
const viewTabs = document.getElementById("view-tabs");
const categorySelect = document.getElementById("category-select");
const recurrenceType = document.getElementById("recurrence-type");
const recurrenceDays = document.getElementById("recurrence-days");
const reminderPreset = document.getElementById("reminder-preset");
const reminderCustom = document.getElementById("reminder-custom");
const weekSummaryEl = document.getElementById("week-summary");
const weekDaysEl = document.getElementById("week-days");
const heroDateEl = document.getElementById("hero-date");
const heroStatsEl = document.getElementById("hero-stats");
const habitPresetsEl = document.getElementById("habit-presets");

const HABIT_PRESETS = [
  { title: "Workout", category: "fitness", recurrence_type: "weekly", recurrence_days: [1, 2, 4, 6] },
  { title: "Hit 10k steps", category: "fitness", recurrence_type: "daily" },
  { title: "Eat enough protein", category: "nutrition", recurrence_type: "daily" },
  { title: "Skincare", category: "self-care", recurrence_type: "daily" },
  { title: "Exfoliate", category: "self-care", recurrence_type: "weekly", recurrence_days: [7] },
  { title: "Cut nails", category: "self-care", recurrence_type: "weekly", recurrence_days: [7] },
  { title: "Study", category: "growth", recurrence_type: "daily" },
  { title: "Sleep routine", category: "wellness", recurrence_type: "daily" },
];

let editingId = null;
let currentTasks = [];
const state = { view: "today", category: "", weekDay: null };

function localDateStr(d) {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

function mondayOf(d) {
  const monday = new Date(d);
  monday.setDate(d.getDate() - ((d.getDay() + 6) % 7));
  return monday;
}

function escapeHtml(s) {
  const div = document.createElement("div");
  div.textContent = s;
  return div.innerHTML;
}

const EMPTY_MESSAGES = {
  today: "Nothing due today. Tap + to add one.",
  week: "Nothing planned this week.",
  overdue: "Nothing overdue. Nice work!",
  upcoming: "Nothing coming up yet.",
  all: "No tasks yet. Tap + to add one.",
};

function emptyMessage() {
  if (state.view === "week" && state.weekDay) return "Nothing due on this day.";
  return EMPTY_MESSAGES[state.view] ?? EMPTY_MESSAGES.all;
}

function renderTasks(tasks) {
  listEl.innerHTML = "";
  emptyEl.textContent = emptyMessage();
  emptyEl.classList.toggle("hidden", tasks.length > 0);

  for (const t of tasks) {
    const li = document.createElement("li");
    li.className = "task" + (t.completed ? " done" : "");

    const due = [t.due_date, t.due_time?.slice(0, 5)].filter(Boolean).join(" ");
    const recurLabel = t.recurrence_type === "daily" ? "daily" : t.recurrence_type === "weekly" ? "weekly" : "";
    li.innerHTML = `
      <input type="checkbox" ${t.completed ? "checked" : ""} data-id="${t.id}" data-date="${t.due_date ?? ""}" class="toggle" />
      <div class="body" data-id="${t.id}">
        <div class="title">${escapeHtml(t.title)}</div>
        ${due || t.category || recurLabel ? `<div class="meta">${[due, t.category, recurLabel].filter(Boolean).map(escapeHtml).join(" · ")}</div>` : ""}
      </div>
    `;
    listEl.appendChild(li);
  }
}

function renderWeekStrip(occurrences) {
  const todayStr = localDateStr(new Date());
  const monday = mondayOf(new Date());
  const byDate = {};
  for (const o of occurrences) {
    (byDate[o.due_date] ??= []).push(o);
  }
  const dows = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];
  weekDaysEl.innerHTML = dows
    .map((dow, i) => {
      const d = new Date(monday);
      d.setDate(monday.getDate() + i);
      const dateStr = localDateStr(d);
      const dayTasks = byDate[dateStr] ?? [];
      const done = dayTasks.filter((t) => t.completed).length;
      const classes = [dateStr === todayStr ? "today" : "", dateStr === state.weekDay ? "selected" : ""].filter(Boolean).join(" ");
      return `<button type="button" class="${classes}" data-date="${dateStr}">
        <span class="dow">${dow}</span>
        <span class="num">${d.getDate()}</span>
        <span class="count">${dayTasks.length ? `${done}/${dayTasks.length}` : "-"}</span>
      </button>`;
    })
    .join("");
  weekDaysEl.classList.remove("hidden");
}

weekDaysEl.addEventListener("click", (e) => {
  const btn = e.target.closest("button[data-date]");
  if (!btn) return;
  state.weekDay = state.weekDay === btn.dataset.date ? null : btn.dataset.date;
  renderWeekStrip(currentTasks);
  renderTasks(state.weekDay ? currentTasks.filter((t) => t.due_date === state.weekDay) : currentTasks);
});

async function loadTasks() {
  const params = new URLSearchParams({ view: state.view });
  if (state.category) params.set("category", state.category);
  currentTasks = await apiFetch(`/api/tasks?${params}`);
  if (state.view === "week") {
    renderWeekStrip(currentTasks);
    renderTasks(state.weekDay ? currentTasks.filter((t) => t.due_date === state.weekDay) : currentTasks);
  } else {
    weekDaysEl.classList.add("hidden");
    renderTasks(currentTasks);
  }
}

async function refreshHero() {
  heroDateEl.textContent = new Date().toLocaleDateString(undefined, {
    weekday: "long",
    month: "long",
    day: "numeric",
  });
  const [today, overdue] = await Promise.all([apiFetch("/api/tasks?view=today"), apiFetch("/api/tasks?view=overdue")]);
  const done = today.filter((t) => t.completed).length;
  const overduePill = overdue.length
    ? `<button type="button" class="overdue-pill" id="hero-overdue">${overdue.length} overdue</button>`
    : "";
  heroStatsEl.innerHTML = `<span>${done}/${today.length} done today</span>${overduePill}`;
}

heroStatsEl.addEventListener("click", (e) => {
  if (e.target.id !== "hero-overdue") return;
  state.view = "overdue";
  state.weekDay = null;
  for (const b of viewTabs.querySelectorAll("button")) b.classList.toggle("active", b.dataset.view === "overdue");
  loadTasks();
});

async function refreshCategories() {
  const all = await apiFetch("/api/tasks?view=all");
  const categories = [...new Set(all.map((t) => t.category).filter(Boolean))].sort();
  const selected = categorySelect.value;
  categorySelect.innerHTML = '<option value="">All categories</option>';
  for (const c of categories) {
    const opt = document.createElement("option");
    opt.value = c;
    opt.textContent = c;
    categorySelect.appendChild(opt);
  }
  categorySelect.value = categories.includes(selected) ? selected : "";
}

async function refreshWeekSummary() {
  if (state.view !== "week") {
    weekSummaryEl.classList.add("hidden");
    return;
  }
  const summary = await apiFetch("/api/summary/week");
  const pct = summary.expected ? Math.round((100 * summary.completed) / summary.expected) : 0;
  const categories = (summary.by_category ?? [])
    .map((c) => `<span>${escapeHtml(c.category)}: ${c.completed}/${c.expected}</span>`)
    .join("");
  weekSummaryEl.innerHTML = `
    <div class="total">${summary.completed}/${summary.expected} done this week (${pct}%)</div>
    <div class="categories">${categories}</div>
  `;
  weekSummaryEl.classList.remove("hidden");
}

viewTabs.addEventListener("click", async (e) => {
  const btn = e.target.closest("button[data-view]");
  if (!btn) return;
  state.view = btn.dataset.view;
  state.weekDay = null;
  for (const b of viewTabs.querySelectorAll("button")) {
    b.classList.toggle("active", b === btn);
  }
  await loadTasks();
  await refreshWeekSummary();
});

categorySelect.addEventListener("change", async () => {
  state.category = categorySelect.value;
  await loadTasks();
});

function setSelectedDays(days) {
  for (const btn of recurrenceDays.querySelectorAll("button[data-day]")) {
    btn.classList.toggle("active", days.includes(Number(btn.dataset.day)));
  }
}

function getSelectedDays() {
  return [...recurrenceDays.querySelectorAll("button[data-day].active")].map((b) => Number(b.dataset.day));
}

function updateRecurrenceDaysVisibility() {
  recurrenceDays.classList.toggle("hidden", recurrenceType.value !== "weekly");
}

recurrenceType.addEventListener("change", updateRecurrenceDaysVisibility);

recurrenceDays.addEventListener("click", (e) => {
  const dayBtn = e.target.closest("button[data-day]");
  if (dayBtn) {
    dayBtn.classList.toggle("active");
    return;
  }
  if (e.target.id === "preset-weekdays") {
    recurrenceType.value = "weekly";
    updateRecurrenceDaysVisibility();
    setSelectedDays([1, 2, 3, 4, 5]);
  }
});

const REMINDER_PRESETS = ["10", "30", "60", "1440"];

function setReminderMinutes(minutes) {
  if (minutes == null) {
    reminderPreset.value = "";
  } else if (REMINDER_PRESETS.includes(String(minutes))) {
    reminderPreset.value = String(minutes);
  } else {
    reminderPreset.value = "custom";
    reminderCustom.value = minutes;
  }
  reminderCustom.classList.toggle("hidden", reminderPreset.value !== "custom");
}

function getReminderMinutes() {
  if (reminderPreset.value === "") return null;
  if (reminderPreset.value === "custom") {
    const n = Number(reminderCustom.value);
    return Number.isFinite(n) && n > 0 ? n : null;
  }
  return Number(reminderPreset.value);
}

reminderPreset.addEventListener("change", () => {
  reminderCustom.classList.toggle("hidden", reminderPreset.value !== "custom");
});

function openModal(task) {
  editingId = task?.id ?? null;
  modalTitle.textContent = editingId ? "Edit task" : "New task";
  form.title.value = task?.title ?? "";
  form.notes.value = task?.notes ?? "";
  form.category.value = task?.category ?? "";
  form.due_date.value = task?.due_date ?? "";
  form.due_time.value = task?.due_time?.slice(0, 5) ?? "";
  recurrenceType.value = task?.recurrence_type ?? "none";
  setSelectedDays(task?.recurrence_days ?? []);
  updateRecurrenceDaysVisibility();
  setReminderMinutes(task?.reminder_minutes_before ?? null);
  deleteBtn.classList.toggle("hidden", !editingId);

  habitPresetsEl.classList.toggle("hidden", !!editingId);
  if (!editingId && habitPresetsEl.childElementCount === 0) {
    habitPresetsEl.innerHTML = HABIT_PRESETS.map(
      (p, i) => `<button type="button" data-preset="${i}">${escapeHtml(p.title)}</button>`
    ).join("");
  }

  modalBackdrop.classList.remove("hidden");
}

function closeModal() {
  modalBackdrop.classList.add("hidden");
  editingId = null;
  form.reset();
  setSelectedDays([]);
  updateRecurrenceDaysVisibility();
  setReminderMinutes(null);
}

habitPresetsEl.addEventListener("click", (e) => {
  const btn = e.target.closest("button[data-preset]");
  if (!btn) return;
  const preset = HABIT_PRESETS[Number(btn.dataset.preset)];
  form.title.value = preset.title;
  form.category.value = preset.category;
  recurrenceType.value = preset.recurrence_type;
  setSelectedDays(preset.recurrence_days ?? []);
  updateRecurrenceDaysVisibility();
});

document.getElementById("new-task").addEventListener("click", () => openModal(null));
document.getElementById("cancel-modal").addEventListener("click", closeModal);
modalBackdrop.addEventListener("click", (e) => {
  if (e.target === modalBackdrop) closeModal();
});

listEl.addEventListener("click", async (e) => {
  const checkbox = e.target.closest(".toggle");
  if (checkbox) {
    const id = checkbox.dataset.id;
    const action = checkbox.checked ? "complete" : "uncomplete";
    const body = checkbox.dataset.date ? JSON.stringify({ occurrence_date: checkbox.dataset.date }) : undefined;
    await apiFetch(`/api/tasks/${id}/${action}`, { method: "POST", body });
    await loadTasks();
    await refreshWeekSummary();
    await refreshHero();
    return;
  }

  const body = e.target.closest(".body");
  if (body) {
    const task = currentTasks.find((t) => t.id === body.dataset.id);
    if (task) openModal(task);
  }
});

form.addEventListener("submit", async (e) => {
  e.preventDefault();
  const payload = {
    title: form.title.value.trim(),
    notes: form.notes.value.trim() || null,
    category: form.category.value.trim() || null,
    due_date: form.due_date.value || null,
    due_time: form.due_time.value || null,
    recurrence_type: recurrenceType.value,
    recurrence_days: recurrenceType.value === "weekly" ? getSelectedDays() : [],
    reminder_minutes_before: getReminderMinutes(),
  };
  if (!payload.title) return;
  if (payload.recurrence_type === "weekly" && payload.recurrence_days.length === 0) {
    alert("Pick at least one day for a weekly repeat.");
    return;
  }
  if (payload.reminder_minutes_before != null && !payload.due_time) {
    alert("Add a due time to use a reminder - without one there's no specific moment to count back from.");
    return;
  }

  if (editingId) {
    await apiFetch(`/api/tasks/${editingId}`, { method: "PUT", body: JSON.stringify(payload) });
  } else {
    await apiFetch("/api/tasks", { method: "POST", body: JSON.stringify(payload) });
  }
  closeModal();
  await loadTasks();
  await refreshCategories();
  await refreshWeekSummary();
  await refreshHero();
});

deleteBtn.addEventListener("click", async () => {
  if (!editingId) return;
  await apiFetch(`/api/tasks/${editingId}`, { method: "DELETE" });
  closeModal();
  await loadTasks();
  await refreshCategories();
  await refreshWeekSummary();
  await refreshHero();
});

await loadTasks();
await refreshCategories();
await refreshWeekSummary();
await refreshHero();
