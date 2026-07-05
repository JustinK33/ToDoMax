import { apiFetch } from "./api.js";
import { escapeHtml } from "./dom-utils.js";

const listEl = document.getElementById("item-list");
const emptyEl = document.getElementById("empty-state");
const modalBackdrop = document.getElementById("modal-backdrop");
const viewTabs = document.getElementById("view-tabs");
const heroDateEl = document.getElementById("hero-date");
const heroStatsEl = document.getElementById("hero-stats");

const logForm = document.getElementById("log-form");
const foodForm = document.getElementById("food-form");
const mealForm = document.getElementById("meal-form");
const targetForm = document.getElementById("target-form");
const allForms = [logForm, foodForm, mealForm, targetForm];

const state = { view: "today" };
let currentFoods = [];
let currentMeals = [];
let currentSummary = null;
let editingFoodId = null;
let editingMealId = null;

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

// --- Today / log view ---

function renderHero(summary) {
  heroDateEl.textContent = new Date().toLocaleDateString(undefined, {
    weekday: "long",
    month: "long",
    day: "numeric",
  });
  const t = summary.target ?? {};
  const totals = summary.totals ?? { calories: 0, protein_g: 0, carbs_g: 0, fat_g: 0 };
  const parts = [];
  if (t.calories != null) parts.push(`${Math.round(totals.calories)}/${t.calories} cal`);
  else parts.push(`${Math.round(totals.calories)} cal`);
  if (t.protein_g != null) parts.push(`${round1(totals.protein_g)}/${t.protein_g}g protein`);
  else if (totals.protein_g) parts.push(`${round1(totals.protein_g)}g protein`);

  const hitProtein = t.protein_g != null && totals.protein_g >= t.protein_g;
  const pill = hitProtein ? `<span class="pill pill-accent">Protein goal hit</span>` : "";
  heroStatsEl.innerHTML = `<span>${parts.join(" · ")}</span>${pill}`;
}

function entryRow(e) {
  const li = document.createElement("li");
  li.className = "task";
  li.innerHTML = `
    <div class="body">
      <div class="title">${escapeHtml(e.name)}</div>
      <div class="meta">${e.servings}&times; · ${Math.round(e.calories)} cal · ${round1(e.protein_g)}g protein</div>
    </div>
    <button type="button" class="delete-entry" data-id="${e.id}" aria-label="Remove">&times;</button>
  `;
  return li;
}

async function loadToday() {
  currentSummary = await apiFetch("/api/nutrition/day");
  renderHero(currentSummary);
  const entries = currentSummary.entries ?? [];
  emptyEl.textContent = "Nothing logged today yet. Tap + to log a food or meal.";
  emptyEl.classList.toggle("hidden", entries.length > 0);
  listEl.innerHTML = "";
  for (const e of entries) listEl.appendChild(entryRow(e));
}

// --- Foods view ---

function foodRow(f) {
  const li = document.createElement("li");
  li.className = "task task-manage";
  li.dataset.id = f.id;
  li.innerHTML = `
    <div class="body">
      <div class="title">${escapeHtml(f.name)}</div>
      <div class="meta">per ${escapeHtml(f.serving_label)} · ${Math.round(f.calories)} cal · ${round1(f.protein_g)}g protein · ${round1(f.carbs_g)}g carbs · ${round1(f.fat_g)}g fat</div>
    </div>
    <span class="chevron" aria-hidden="true">&rsaquo;</span>
  `;
  return li;
}

async function loadFoods() {
  currentFoods = await apiFetch("/api/foods");
  emptyEl.textContent = "No foods yet. Tap + to create one.";
  emptyEl.classList.toggle("hidden", currentFoods.length > 0);
  listEl.innerHTML = "";
  for (const f of currentFoods) listEl.appendChild(foodRow(f));
}

// --- Meals view ---

function mealRow(m) {
  const li = document.createElement("li");
  li.className = "task task-manage";
  li.dataset.id = m.id;
  li.innerHTML = `
    <div class="body">
      <div class="title">${escapeHtml(m.name)}</div>
      <div class="meta">${Math.round(m.total_calories)} cal · ${round1(m.total_protein_g)}g protein · ${round1(m.total_carbs_g)}g carbs · ${round1(m.total_fat_g)}g fat</div>
    </div>
    <span class="chevron" aria-hidden="true">&rsaquo;</span>
  `;
  return li;
}

async function loadMeals() {
  currentMeals = await apiFetch("/api/meals");
  emptyEl.textContent = "No meals yet. Tap + to combine some foods into one.";
  emptyEl.classList.toggle("hidden", currentMeals.length > 0);
  listEl.innerHTML = "";
  for (const m of currentMeals) listEl.appendChild(mealRow(m));
}

async function loadView() {
  try {
    if (state.view === "today") await loadToday();
    else if (state.view === "foods") await loadFoods();
    else await loadMeals();
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
  for (const b of viewTabs.querySelectorAll("button")) b.classList.toggle("active", b === btn);
  await loadView();
});

// --- Modal plumbing ---

function showModal(form) {
  for (const f of allForms) f.classList.toggle("hidden", f !== form);
  modalBackdrop.classList.remove("hidden");
}

function closeModal() {
  modalBackdrop.classList.add("hidden");
  for (const f of allForms) f.reset();
  editingFoodId = null;
  editingMealId = null;
}

document.querySelectorAll(".cancel-modal").forEach((btn) => btn.addEventListener("click", closeModal));
modalBackdrop.addEventListener("click", (e) => {
  if (e.target === modalBackdrop) closeModal();
});

document.getElementById("new-item").addEventListener("click", () => {
  if (state.view === "today") openLogModal();
  else if (state.view === "foods") openFoodModal(null);
  else openMealModal(null);
});

listEl.addEventListener("click", async (e) => {
  const del = e.target.closest(".delete-entry");
  if (del) {
    try {
      await apiFetch(`/api/nutrition/log/${del.dataset.id}`, { method: "DELETE" });
      await loadToday();
    } catch (err) {
      reportError(err, "deleting");
    }
    return;
  }

  const row = e.target.closest(".task-manage");
  if (row) {
    if (state.view === "foods") {
      const food = currentFoods.find((f) => f.id === row.dataset.id);
      if (food) openFoodModal(food);
    } else if (state.view === "meals") {
      const meal = currentMeals.find((m) => m.id === row.dataset.id);
      if (meal) openMealModal(meal);
    }
  }
});

// --- Log modal ---

function openLogModal() {
  logForm.reset();
  logForm.log_date.value = localDateStr(new Date());
  setLogSource("food");
  showModal(logForm);
}

function setLogSource(source) {
  for (const btn of logForm.querySelectorAll(".source-toggle")) {
    btn.classList.toggle("active", btn.dataset.source === source);
  }
  const select = document.getElementById("log-source-select");
  const options = source === "food" ? currentFoods : currentMeals;
  select.innerHTML = options.map((o) => `<option value="${o.id}">${escapeHtml(o.name)}</option>`).join("");
  select.dataset.source = source;
}

logForm.querySelectorAll(".source-toggle").forEach((btn) => {
  btn.addEventListener("click", () => setLogSource(btn.dataset.source));
});

logForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const select = document.getElementById("log-source-select");
  const id = select.value;
  if (!id) {
    alert(state.view === "today" && select.dataset.source === "food" ? "Create a food first." : "Create a meal first.");
    return;
  }
  const payload = {
    log_date: logForm.log_date.value,
    servings: Number(logForm.servings.value),
    food_id: select.dataset.source === "food" ? id : null,
    meal_id: select.dataset.source === "meal" ? id : null,
  };
  try {
    await apiFetch("/api/nutrition/log", { method: "POST", body: JSON.stringify(payload) });
    closeModal();
    await loadToday();
  } catch (err) {
    reportError(err);
  }
});

// --- Food modal ---

function openFoodModal(food) {
  editingFoodId = food?.id ?? null;
  document.getElementById("food-modal-title").textContent = editingFoodId ? "Edit food" : "New food";
  foodForm.name.value = food?.name ?? "";
  foodForm.serving_label.value = food?.serving_label ?? "";
  foodForm.calories.value = food?.calories ?? 0;
  foodForm.protein_g.value = food?.protein_g ?? 0;
  foodForm.carbs_g.value = food?.carbs_g ?? 0;
  foodForm.fat_g.value = food?.fat_g ?? 0;
  const deleteBtn = document.getElementById("delete-food");
  deleteBtn.classList.toggle("hidden", !editingFoodId);
  deleteBtn.textContent = "Delete";
  deleteBtn.classList.remove("confirm");
  showModal(foodForm);
}

foodForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const payload = {
    name: foodForm.name.value.trim(),
    serving_label: foodForm.serving_label.value.trim() || "serving",
    calories: Number(foodForm.calories.value) || 0,
    protein_g: Number(foodForm.protein_g.value) || 0,
    carbs_g: Number(foodForm.carbs_g.value) || 0,
    fat_g: Number(foodForm.fat_g.value) || 0,
  };
  if (!payload.name) return;
  try {
    if (editingFoodId) {
      await apiFetch(`/api/foods/${editingFoodId}`, { method: "PUT", body: JSON.stringify(payload) });
    } else {
      await apiFetch("/api/foods", { method: "POST", body: JSON.stringify(payload) });
    }
    closeModal();
    await loadFoods();
  } catch (err) {
    reportError(err);
  }
});

document.getElementById("delete-food").addEventListener("click", async () => {
  const btn = document.getElementById("delete-food");
  if (!editingFoodId) return;
  if (!btn.classList.contains("confirm")) {
    btn.textContent = "Tap again to delete";
    btn.classList.add("confirm");
    return;
  }
  try {
    await apiFetch(`/api/foods/${editingFoodId}`, { method: "DELETE" });
    closeModal();
    await loadFoods();
  } catch (err) {
    reportError(err, "deleting");
  }
});

// --- Meal modal ---

function mealItemRow(foodId, servings) {
  const row = document.createElement("div");
  row.className = "meal-item-row";
  row.innerHTML = `
    <select class="meal-item-food">
      ${currentFoods.map((f) => `<option value="${f.id}" ${f.id === foodId ? "selected" : ""}>${escapeHtml(f.name)}</option>`).join("")}
    </select>
    <input class="meal-item-servings" type="number" min="0.1" step="0.1" value="${servings ?? 1}" />
    <button type="button" class="remove-meal-item">&times;</button>
  `;
  row.querySelector(".remove-meal-item").addEventListener("click", () => row.remove());
  return row;
}

document.getElementById("add-meal-item").addEventListener("click", () => {
  if (currentFoods.length === 0) {
    alert("Create a food first.");
    return;
  }
  document.getElementById("meal-items").appendChild(mealItemRow(currentFoods[0].id, 1));
});

function openMealModal(meal) {
  editingMealId = meal?.id ?? null;
  document.getElementById("meal-modal-title").textContent = editingMealId ? "Edit meal" : "New meal";
  mealForm.name.value = meal?.name ?? "";
  const itemsEl = document.getElementById("meal-items");
  itemsEl.innerHTML = "";
  for (const item of meal?.items ?? []) itemsEl.appendChild(mealItemRow(item.food_id, item.servings));
  const deleteBtn = document.getElementById("delete-meal");
  deleteBtn.classList.toggle("hidden", !editingMealId);
  deleteBtn.textContent = "Delete";
  deleteBtn.classList.remove("confirm");
  showModal(mealForm);
}

mealForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const items = [...document.querySelectorAll(".meal-item-row")].map((row) => ({
    food_id: row.querySelector(".meal-item-food").value,
    servings: Number(row.querySelector(".meal-item-servings").value) || 1,
  }));
  const payload = { name: mealForm.name.value.trim(), items };
  if (!payload.name) return;
  try {
    if (editingMealId) {
      await apiFetch(`/api/meals/${editingMealId}`, { method: "PUT", body: JSON.stringify(payload) });
    } else {
      await apiFetch("/api/meals", { method: "POST", body: JSON.stringify(payload) });
    }
    closeModal();
    await loadMeals();
  } catch (err) {
    reportError(err);
  }
});

document.getElementById("delete-meal").addEventListener("click", async () => {
  const btn = document.getElementById("delete-meal");
  if (!editingMealId) return;
  if (!btn.classList.contains("confirm")) {
    btn.textContent = "Tap again to delete";
    btn.classList.add("confirm");
    return;
  }
  try {
    await apiFetch(`/api/meals/${editingMealId}`, { method: "DELETE" });
    closeModal();
    await loadMeals();
  } catch (err) {
    reportError(err, "deleting");
  }
});

// --- Target modal ---

document.getElementById("set-target").addEventListener("click", async () => {
  try {
    const target = await apiFetch("/api/nutrition/target");
    targetForm.calories.value = target.calories ?? "";
    targetForm.protein_g.value = target.protein_g ?? "";
    targetForm.carbs_g.value = target.carbs_g ?? "";
    targetForm.fat_g.value = target.fat_g ?? "";
    showModal(targetForm);
  } catch (err) {
    reportError(err, "loading");
  }
});

function numOrNull(v) {
  return v === "" ? null : Number(v);
}

targetForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const payload = {
    calories: numOrNull(targetForm.calories.value),
    protein_g: numOrNull(targetForm.protein_g.value),
    carbs_g: numOrNull(targetForm.carbs_g.value),
    fat_g: numOrNull(targetForm.fat_g.value),
  };
  try {
    await apiFetch("/api/nutrition/target", { method: "PUT", body: JSON.stringify(payload) });
    closeModal();
    await loadToday();
  } catch (err) {
    reportError(err);
  }
});

// --- Bootstrap ---

try {
  currentFoods = await apiFetch("/api/foods");
  currentMeals = await apiFetch("/api/meals");
  await loadToday();
} catch (err) {
  console.error(err);
  emptyEl.textContent = "Couldn't load your nutrition data. Check your connection and reload the page.";
  emptyEl.classList.remove("hidden");
}
