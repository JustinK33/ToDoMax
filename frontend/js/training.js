import { apiFetch } from "./api.js";
import { escapeHtml } from "./dom-utils.js";
import { sparkline } from "./sparkline.js";

const listEl = document.getElementById("item-list");
const emptyEl = document.getElementById("empty-state");
const modalBackdrop = document.getElementById("modal-backdrop");
const viewTabs = document.getElementById("view-tabs");
const heroDateEl = document.getElementById("hero-date");
const heroStatsEl = document.getElementById("hero-stats");
const prListEl = document.getElementById("pr-list");
const volumeTrendEl = document.getElementById("volume-trend");
const setForm = document.getElementById("set-form");
const exerciseForm = document.getElementById("exercise-form");
const templateForm = document.getElementById("template-form");
const allForms = [setForm, exerciseForm, templateForm];

const state = { view: "today" };
let currentExercises = [];
let currentTemplates = [];
let editingExerciseId = null;
let editingTemplateId = null;
let activeTemplate = null;

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

function numOrNull(value) {
  return value === "" ? null : Number(value);
}

function textOrNull(value) {
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
}

function renderHero(summary) {
  heroDateEl.textContent = new Date().toLocaleDateString(undefined, {
    weekday: "long",
    month: "long",
    day: "numeric",
  });

  const setWord = summary.week_sets === 1 ? "set" : "sets";
  const todayWord = summary.today_sets.length === 1 ? "set" : "sets";
  const parts = [
    `${summary.today_sets.length} ${todayWord} today`,
    `${summary.week_sets} ${setWord} this week`,
    `${Math.round(summary.week_volume)} lb volume`,
  ];
  const pills = [];
  if (summary.prs.length > 0) pills.push(`<span class="pill pill-accent">${summary.prs.length} PR</span>`);
  if (summary.streak > 0) pills.push(`<span class="pill pill-accent">&#128293; ${summary.streak}d streak</span>`);
  heroStatsEl.innerHTML = `<span>${parts.join(" &middot; ")}</span>${pills.join("")}`;
}

function renderPRs(prs) {
  prListEl.classList.toggle("hidden", prs.length === 0 || state.view !== "today");
  prListEl.innerHTML = prs.map((set) => `
    <div class="pill pill-accent">
      ${escapeHtml(set.exercise_name)} ${round1(set.weight)} x ${set.reps}
    </div>
  `).join("");
}

function setRow(set) {
  const li = document.createElement("li");
  li.className = "task";
  const rpe = set.rpe == null ? "" : ` &middot; RPE ${round1(set.rpe)}`;
  li.innerHTML = `
    <div class="body">
      <div class="title">${escapeHtml(set.exercise_name)}</div>
      <div class="meta">${round1(set.weight)} lb x ${set.reps} &middot; ${Math.round(set.volume)} lb volume${rpe}</div>
    </div>
    <button type="button" class="delete-set" data-id="${set.id}" aria-label="Remove">&times;</button>
  `;
  return li;
}

function exerciseRow(exercise) {
  const li = document.createElement("li");
  li.className = "task task-manage";
  li.dataset.id = exercise.id;
  li.innerHTML = `
    <div class="body">
      <div class="title">${escapeHtml(exercise.name)}</div>
      <div class="meta">${escapeHtml(exercise.category)}${exercise.notes ? ` &middot; ${escapeHtml(exercise.notes)}` : ""}</div>
    </div>
    <span class="chevron" aria-hidden="true">&rsaquo;</span>
  `;
  return li;
}

async function loadToday() {
  const currentSummary = await apiFetch("/api/training/summary");
  renderHero(currentSummary);
  renderPRs(currentSummary.prs ?? []);
  const sets = currentSummary.today_sets ?? [];
  emptyEl.textContent = "No sets logged today. Tap + after your next working set.";
  emptyEl.classList.toggle("hidden", sets.length > 0);
  listEl.innerHTML = "";
  for (const set of sets) listEl.appendChild(setRow(set));

  const history = await apiFetch("/api/training/history?days=14");
  volumeTrendEl.innerHTML = sparkline(history.map((h) => ({ date: h.performed_on, value: h.volume })));
  volumeTrendEl.classList.remove("hidden");
}

async function loadExercises() {
  prListEl.classList.add("hidden");
  volumeTrendEl.classList.add("hidden");
  currentExercises = await apiFetch("/api/exercises");
  emptyEl.textContent = "No exercises yet. Tap + to add bench, squat, deadlift, rows, or anything you track.";
  emptyEl.classList.toggle("hidden", currentExercises.length > 0);
  listEl.innerHTML = "";
  for (const exercise of currentExercises) listEl.appendChild(exerciseRow(exercise));
}

async function loadView() {
  try {
    if (state.view === "today") await loadToday();
    else if (state.view === "workouts") {
      if (activeTemplate) renderActiveTemplate();
      else await loadTemplates();
    } else await loadExercises();
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
  activeTemplate = null;
  for (const tab of viewTabs.querySelectorAll("button")) tab.classList.toggle("active", tab === btn);
  await loadView();
});

function showModal(form) {
  for (const f of allForms) f.classList.toggle("hidden", f !== form);
  modalBackdrop.classList.remove("hidden");
}

function closeModal() {
  modalBackdrop.classList.add("hidden");
  for (const f of allForms) f.reset();
  editingExerciseId = null;
  editingTemplateId = null;
}

document.querySelectorAll(".cancel-modal").forEach((btn) => btn.addEventListener("click", closeModal));
modalBackdrop.addEventListener("click", (e) => {
  if (e.target === modalBackdrop) closeModal();
});

document.getElementById("new-item").addEventListener("click", () => {
  if (state.view === "today") openSetModal();
  else if (state.view === "workouts") openTemplateModal(null);
  else openExerciseModal(null);
});

listEl.addEventListener("click", async (e) => {
  const del = e.target.closest(".delete-set");
  if (del) {
    try {
      await apiFetch(`/api/workout-sets/${del.dataset.id}`, { method: "DELETE" });
      await loadToday();
    } catch (err) {
      reportError(err, "deleting");
    }
    return;
  }

  const back = e.target.closest(".back-to-workouts");
  if (back) {
    activeTemplate = null;
    await loadTemplates();
    return;
  }

  const startExercise = e.target.closest(".start-exercise");
  if (startExercise) {
    openSetModal(startExercise.dataset.id);
    return;
  }

  const start = e.target.closest(".start-template");
  if (start) {
    activeTemplate = currentTemplates.find((t) => t.id === start.dataset.id);
    renderActiveTemplate();
    return;
  }

  const row = e.target.closest(".task-manage");
  if (!row) return;
  if (state.view === "exercises") {
    const exercise = currentExercises.find((item) => item.id === row.dataset.id);
    if (exercise) openExerciseModal(exercise);
  } else if (state.view === "workouts") {
    const template = currentTemplates.find((item) => item.id === row.dataset.id);
    if (template) openTemplateModal(template);
  }
});

function populateExerciseSelect() {
  setForm.exercise_id.innerHTML = currentExercises
    .map((exercise) => `<option value="${exercise.id}">${escapeHtml(exercise.name)}</option>`)
    .join("");
}

function openSetModal(exerciseId) {
  if (currentExercises.length === 0) {
    openExerciseModal(null);
    return;
  }
  setForm.reset();
  populateExerciseSelect();
  if (exerciseId) setForm.exercise_id.value = exerciseId;
  setForm.performed_on.value = localDateStr(new Date());
  showModal(setForm);
}

setForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const payload = {
    exercise_id: setForm.exercise_id.value,
    performed_on: setForm.performed_on.value,
    weight: Number(setForm.weight.value) || 0,
    reps: Number(setForm.reps.value) || 0,
    rpe: numOrNull(setForm.rpe.value),
    notes: textOrNull(setForm.notes.value),
  };
  try {
    await apiFetch("/api/workout-sets", { method: "POST", body: JSON.stringify(payload) });
    closeModal();
    if (state.view === "workouts") {
      const summary = await apiFetch("/api/training/summary");
      renderHero(summary);
    }
    await loadView();
  } catch (err) {
    reportError(err);
  }
});

function populateCategoryDatalist() {
  const categories = [...new Set(currentExercises.map((e) => e.category).filter(Boolean))].sort();
  document.getElementById("category-options").innerHTML = categories
    .map((c) => `<option value="${escapeHtml(c)}"></option>`)
    .join("");
}

function openExerciseModal(exercise) {
  populateCategoryDatalist();
  editingExerciseId = exercise?.id ?? null;
  document.getElementById("exercise-modal-title").textContent = editingExerciseId ? "Edit exercise" : "New exercise";
  exerciseForm.name.value = exercise?.name ?? "";
  exerciseForm.category.value = exercise?.category ?? "Strength";
  exerciseForm.notes.value = exercise?.notes ?? "";
  const deleteBtn = document.getElementById("delete-exercise");
  deleteBtn.classList.toggle("hidden", !editingExerciseId);
  deleteBtn.textContent = "Delete";
  deleteBtn.classList.remove("confirm");
  showModal(exerciseForm);
}

exerciseForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const payload = {
    name: exerciseForm.name.value.trim(),
    category: exerciseForm.category.value,
    notes: textOrNull(exerciseForm.notes.value),
  };
  if (!payload.name) return;

  try {
    if (editingExerciseId) {
      await apiFetch(`/api/exercises/${editingExerciseId}`, { method: "PUT", body: JSON.stringify(payload) });
    } else {
      await apiFetch("/api/exercises", { method: "POST", body: JSON.stringify(payload) });
    }
    closeModal();
    currentExercises = await apiFetch("/api/exercises");
    await loadView();
  } catch (err) {
    reportError(err);
  }
});

document.getElementById("delete-exercise").addEventListener("click", async () => {
  const btn = document.getElementById("delete-exercise");
  if (!editingExerciseId) return;
  if (!btn.classList.contains("confirm")) {
    btn.textContent = "Tap again to delete";
    btn.classList.add("confirm");
    return;
  }
  try {
    await apiFetch(`/api/exercises/${editingExerciseId}`, { method: "DELETE" });
    closeModal();
    await loadExercises();
  } catch (err) {
    reportError(err, "deleting");
  }
});

function templateRow(template) {
  const li = document.createElement("li");
  li.className = "task task-manage";
  li.dataset.id = template.id;
  const names = template.items.map((i) => escapeHtml(i.exercise_name)).join(", ") || "No exercises yet";
  const countWord = template.items.length === 1 ? "exercise" : "exercises";
  li.innerHTML = `
    <div class="body">
      <div class="title">${escapeHtml(template.name)}</div>
      <div class="meta">${template.items.length} ${countWord} &middot; ${names}</div>
    </div>
    <button type="button" class="start-template" data-id="${template.id}">Start</button>
    <span class="chevron" aria-hidden="true">&rsaquo;</span>
  `;
  return li;
}

async function loadTemplates() {
  prListEl.classList.add("hidden");
  volumeTrendEl.classList.add("hidden");
  currentTemplates = await apiFetch("/api/workout-templates");
  emptyEl.textContent = "No workouts yet. Tap + to build one, e.g. 'Leg Day A'.";
  emptyEl.classList.toggle("hidden", currentTemplates.length > 0);
  listEl.innerHTML = "";
  for (const template of currentTemplates) listEl.appendChild(templateRow(template));
}

function renderActiveTemplate() {
  prListEl.classList.add("hidden");
  emptyEl.classList.add("hidden");
  listEl.innerHTML = "";
  const back = document.createElement("li");
  back.className = "task task-manage back-to-workouts";
  back.innerHTML = `<div class="body"><div class="title">&larr; Back to workouts</div></div>`;
  listEl.appendChild(back);
  for (const item of activeTemplate.items) {
    const li = document.createElement("li");
    li.className = "task task-manage start-exercise";
    li.dataset.id = item.exercise_id;
    li.innerHTML = `
      <div class="body"><div class="title">${escapeHtml(item.exercise_name)}</div></div>
      <span class="chevron" aria-hidden="true">&rsaquo;</span>
    `;
    listEl.appendChild(li);
  }
}

function templateItemRow(exerciseId) {
  const row = document.createElement("div");
  row.className = "meal-item-row template-item-row";
  row.innerHTML = `
    <select class="template-item-exercise">
      ${currentExercises.map((ex) => `<option value="${ex.id}" ${ex.id === exerciseId ? "selected" : ""}>${escapeHtml(ex.name)}</option>`).join("")}
    </select>
    <button type="button" class="remove-meal-item">&times;</button>
  `;
  row.querySelector(".remove-meal-item").addEventListener("click", () => row.remove());
  return row;
}

document.getElementById("add-template-item").addEventListener("click", () => {
  if (currentExercises.length === 0) {
    alert("Create an exercise first.");
    return;
  }
  document.getElementById("template-items").appendChild(templateItemRow(currentExercises[0].id));
});

function openTemplateModal(template) {
  editingTemplateId = template?.id ?? null;
  document.getElementById("template-modal-title").textContent = editingTemplateId ? "Edit workout" : "New workout";
  templateForm.name.value = template?.name ?? "";
  const itemsEl = document.getElementById("template-items");
  itemsEl.innerHTML = "";
  for (const item of template?.items ?? []) itemsEl.appendChild(templateItemRow(item.exercise_id));
  const deleteBtn = document.getElementById("delete-template");
  deleteBtn.classList.toggle("hidden", !editingTemplateId);
  deleteBtn.textContent = "Delete";
  deleteBtn.classList.remove("confirm");
  showModal(templateForm);
}

templateForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const items = [...document.querySelectorAll(".template-item-row")].map((row) => ({
    exercise_id: row.querySelector(".template-item-exercise").value,
  }));
  const payload = { name: templateForm.name.value.trim(), items };
  if (!payload.name) return;
  try {
    if (editingTemplateId) {
      await apiFetch(`/api/workout-templates/${editingTemplateId}`, { method: "PUT", body: JSON.stringify(payload) });
    } else {
      await apiFetch("/api/workout-templates", { method: "POST", body: JSON.stringify(payload) });
    }
    closeModal();
    await loadTemplates();
  } catch (err) {
    reportError(err);
  }
});

document.getElementById("delete-template").addEventListener("click", async () => {
  const btn = document.getElementById("delete-template");
  if (!editingTemplateId) return;
  if (!btn.classList.contains("confirm")) {
    btn.textContent = "Tap again to delete";
    btn.classList.add("confirm");
    return;
  }
  try {
    await apiFetch(`/api/workout-templates/${editingTemplateId}`, { method: "DELETE" });
    closeModal();
    await loadTemplates();
  } catch (err) {
    reportError(err, "deleting");
  }
});

try {
  currentExercises = await apiFetch("/api/exercises");
  await loadToday();
} catch (err) {
  console.error(err);
  emptyEl.textContent = "Couldn't load your training data. Check your connection and reload the page.";
  emptyEl.classList.remove("hidden");
}
