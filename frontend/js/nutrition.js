import { apiFetch } from "./api.js";
import { escapeHtml } from "./dom-utils.js";
import { sparkline } from "./sparkline.js";

const listEl = document.getElementById("item-list");
const emptyEl = document.getElementById("empty-state");
const modalBackdrop = document.getElementById("modal-backdrop");
const viewTabs = document.getElementById("view-tabs");
const heroDateEl = document.getElementById("hero-date");
const heroStatsEl = document.getElementById("hero-stats");
const calorieTrendEl = document.getElementById("calorie-trend");

const logForm = document.getElementById("log-form");
const foodForm = document.getElementById("food-form");
const mealForm = document.getElementById("meal-form");
const targetForm = document.getElementById("target-form");
const allForms = [logForm, foodForm, mealForm, targetForm];

const state = { view: "today", viewedDate: localDateStr(new Date()) };
let currentFoods = [];
let currentMeals = [];
let currentSummary = null;
let editingFoodId = null;
let editingMealId = null;
let editingEntryId = null;

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
  const viewed = new Date(`${state.viewedDate}T00:00:00`);
  heroDateEl.textContent = viewed.toLocaleDateString(undefined, {
    weekday: "long",
    month: "long",
    day: "numeric",
  });
  document.getElementById("next-day").disabled = state.viewedDate >= localDateStr(new Date());

  const t = summary.target ?? {};
  const totals = summary.totals ?? { calories: 0, protein_g: 0, carbs_g: 0, fat_g: 0 };
  const parts = [];
  if (t.calories != null) parts.push(`${Math.round(totals.calories)}/${t.calories} cal`);
  else parts.push(`${Math.round(totals.calories)} cal`);
  if (t.protein_g != null) parts.push(`${round1(totals.protein_g)}/${t.protein_g}g protein`);
  else if (totals.protein_g) parts.push(`${round1(totals.protein_g)}g protein`);
  if (t.carbs_g != null) parts.push(`${round1(totals.carbs_g)}/${t.carbs_g}g carbs`);
  else if (totals.carbs_g) parts.push(`${round1(totals.carbs_g)}g carbs`);
  if (t.fat_g != null) parts.push(`${round1(totals.fat_g)}/${t.fat_g}g fat`);
  else if (totals.fat_g) parts.push(`${round1(totals.fat_g)}g fat`);

  const hitProtein = t.protein_g != null && totals.protein_g >= t.protein_g;
  const pills = [];
  if (hitProtein) pills.push(`<span class="pill pill-accent">Protein goal hit</span>`);
  if (summary.streak > 0) pills.push(`<span class="pill pill-accent">&#128293; ${summary.streak}d streak</span>`);
  heroStatsEl.innerHTML = `<span>${parts.join(" · ")}</span>${pills.join("")}`;
}

function entryRow(e) {
  const li = document.createElement("li");
  li.className = "task task-manage";
  li.dataset.id = e.id;
  li.innerHTML = `
    <div class="body">
      <div class="title">${escapeHtml(e.name)}</div>
      <div class="meta">${e.servings}&times; · ${Math.round(e.calories)} cal · ${round1(e.protein_g)}g protein · ${round1(e.carbs_g)}g carbs · ${round1(e.fat_g)}g fat</div>
    </div>
    <span class="chevron" aria-hidden="true">&rsaquo;</span>
  `;
  return li;
}

async function loadToday() {
  currentSummary = await apiFetch(`/api/nutrition/day?date=${state.viewedDate}`);
  renderHero(currentSummary);
  const entries = currentSummary.entries ?? [];
  emptyEl.textContent = "Nothing logged today yet. Tap + to log a food or meal.";
  emptyEl.classList.toggle("hidden", entries.length > 0);
  listEl.innerHTML = "";
  for (const e of entries) listEl.appendChild(entryRow(e));

  const history = await apiFetch("/api/nutrition/history?days=14");
  calorieTrendEl.innerHTML = sparkline(history.map((h) => ({ date: h.log_date, value: h.calories })));
  calorieTrendEl.classList.remove("hidden");
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
  calorieTrendEl.classList.add("hidden");
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
  calorieTrendEl.classList.add("hidden");
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

function shiftViewedDate(days) {
  const d = new Date(`${state.viewedDate}T00:00:00`);
  d.setDate(d.getDate() + days);
  state.viewedDate = localDateStr(d);
}

document.getElementById("prev-day").addEventListener("click", async () => {
  shiftViewedDate(-1);
  await loadToday();
});

document.getElementById("next-day").addEventListener("click", async () => {
  shiftViewedDate(1);
  await loadToday();
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
  editingEntryId = null;
}

document.querySelectorAll(".cancel-modal").forEach((btn) => btn.addEventListener("click", closeModal));
modalBackdrop.addEventListener("click", (e) => {
  if (e.target === modalBackdrop) closeModal();
});

document.getElementById("new-item").addEventListener("click", () => {
  if (state.view === "today") openLogModal(null);
  else if (state.view === "foods") openFoodModal(null);
  else openMealModal(null);
});

listEl.addEventListener("click", async (e) => {
  const row = e.target.closest(".task-manage");
  if (row) {
    if (state.view === "today") {
      const entry = (currentSummary?.entries ?? []).find((entry) => entry.id === row.dataset.id);
      if (entry) openLogModal(entry);
    } else if (state.view === "foods") {
      const food = currentFoods.find((f) => f.id === row.dataset.id);
      if (food) openFoodModal(food);
    } else if (state.view === "meals") {
      const meal = currentMeals.find((m) => m.id === row.dataset.id);
      if (meal) openMealModal(meal);
    }
  }
});

// --- Log modal ---

function openLogModal(entry) {
  editingEntryId = entry?.id ?? null;
  document.getElementById("log-modal-title").textContent = editingEntryId ? "Edit log entry" : "Log food";
  logForm.reset();
  logForm.log_date.value = entry?.log_date ?? state.viewedDate;
  logForm.servings.value = entry?.servings ?? 1;
  setLogSource(entry?.meal_id ? "meal" : "food", entry?.name);
  const deleteBtn = document.getElementById("delete-entry");
  deleteBtn.classList.toggle("hidden", !editingEntryId);
  deleteBtn.textContent = "Delete";
  deleteBtn.classList.remove("confirm");
  showModal(logForm);
}

function setLogSource(source, selectedName) {
  for (const btn of logForm.querySelectorAll(".source-toggle")) {
    btn.classList.toggle("active", btn.dataset.source === source);
  }
  const input = document.getElementById("log-source-input");
  const options = source === "food" ? currentFoods : currentMeals;
  document.getElementById("log-source-options").innerHTML = options
    .map((o) => `<option value="${escapeHtml(o.name)}"></option>`)
    .join("");
  input.value = selectedName ?? "";
  input.dataset.source = source;
}

logForm.querySelectorAll(".source-toggle").forEach((btn) => {
  btn.addEventListener("click", () => setLogSource(btn.dataset.source));
});

logForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const input = document.getElementById("log-source-input");
  const source = input.dataset.source;
  const list = source === "food" ? currentFoods : currentMeals;
  const match = list.find((o) => o.name === input.value.trim());
  if (!match) {
    alert(source === "food" ? "Pick a food from the list (or create one first)." : "Pick a meal from the list (or create one first).");
    return;
  }
  const payload = {
    log_date: logForm.log_date.value,
    servings: Number(logForm.servings.value),
    food_id: source === "food" ? match.id : null,
    meal_id: source === "meal" ? match.id : null,
  };
  try {
    if (editingEntryId) {
      await apiFetch(`/api/nutrition/log/${editingEntryId}`, { method: "PUT", body: JSON.stringify(payload) });
    } else {
      await apiFetch("/api/nutrition/log", { method: "POST", body: JSON.stringify(payload) });
    }
    closeModal();
    await loadToday();
  } catch (err) {
    reportError(err);
  }
});

document.getElementById("delete-entry").addEventListener("click", async () => {
  const btn = document.getElementById("delete-entry");
  if (!editingEntryId) return;
  if (!btn.classList.contains("confirm")) {
    btn.textContent = "Tap again to delete";
    btn.classList.add("confirm");
    return;
  }
  try {
    await apiFetch(`/api/nutrition/log/${editingEntryId}`, { method: "DELETE" });
    closeModal();
    await loadToday();
  } catch (err) {
    reportError(err, "deleting");
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

function populateMealItemFoodsDatalist() {
  document.getElementById("meal-item-foods").innerHTML = currentFoods
    .map((f) => `<option value="${escapeHtml(f.name)}"></option>`)
    .join("");
}

function mealItemRow(foodId, servings) {
  const food = currentFoods.find((f) => f.id === foodId);
  const row = document.createElement("div");
  row.className = "meal-item-row";
  row.innerHTML = `
    <input class="meal-item-food" list="meal-item-foods" autocomplete="off" placeholder="Type to search..." value="${escapeHtml(food?.name ?? "")}" />
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
  document.getElementById("meal-items").appendChild(mealItemRow(null, 1));
});

function openMealModal(meal) {
  populateMealItemFoodsDatalist();
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
  const items = [...document.querySelectorAll(".meal-item-row")]
    .map((row) => {
      const name = row.querySelector(".meal-item-food").value.trim();
      const food = currentFoods.find((f) => f.name === name);
      if (!food) return null;
      return { food_id: food.id, servings: Number(row.querySelector(".meal-item-servings").value) || 1 };
    })
    .filter(Boolean);
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
