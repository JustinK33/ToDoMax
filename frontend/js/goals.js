import { apiFetch } from "./api.js";
import { escapeHtml } from "./dom-utils.js";

const listEl = document.getElementById("goal-list");
const emptyEl = document.getElementById("empty-state");
const modalBackdrop = document.getElementById("modal-backdrop");
const form = document.getElementById("goal-form");
const deleteBtn = document.getElementById("delete-goal");
const modalTitle = document.getElementById("modal-title");

const TIMEFRAME_LABELS = { week: "This Week", month: "This Month", year: "This Year", lifetime: "Lifetime" };
const TIMEFRAME_ORDER = ["week", "month", "year", "lifetime"];

let editingId = null;
let currentGoals = [];

function reportError(err, action = "saving") {
  console.error(err);
  alert(`Something went wrong ${action} that. Check your connection and try again.`);
}

function goalRow(g) {
  const li = document.createElement("li");
  li.className = "task" + (g.completed ? " done" : "");

  const meta = [g.target_date, g.notes].filter(Boolean);
  li.innerHTML = `
    <input type="checkbox" ${g.completed ? "checked" : ""} data-id="${g.id}" class="toggle" />
    <div class="body" data-id="${g.id}">
      <div class="title">${escapeHtml(g.title)}</div>
      ${meta.length ? `<div class="meta">${meta.map(escapeHtml).join(" · ")}</div>` : ""}
    </div>
  `;
  return li;
}

function renderGoals(goals) {
  listEl.innerHTML = "";
  emptyEl.textContent = "No goals yet. Goals are optional - tap + if you want to track one.";
  emptyEl.classList.toggle("hidden", goals.length > 0);
  if (goals.length === 0) return;

  const groups = new Map();
  for (const g of goals) {
    if (!groups.has(g.timeframe)) groups.set(g.timeframe, []);
    groups.get(g.timeframe).push(g);
  }

  for (const timeframe of TIMEFRAME_ORDER) {
    const groupGoals = groups.get(timeframe);
    if (!groupGoals) continue;
    const done = groupGoals.filter((g) => g.completed).length;
    const group = document.createElement("li");
    group.className = "task-group";
    const header = document.createElement("div");
    header.className = "task-group-header";
    header.innerHTML = `<span class="cat-name">${TIMEFRAME_LABELS[timeframe]}</span><span class="cat-count">${done}/${groupGoals.length}</span>`;
    const sublist = document.createElement("ul");
    sublist.className = "task-group-list";
    for (const g of groupGoals) sublist.appendChild(goalRow(g));
    group.append(header, sublist);
    listEl.appendChild(group);
  }
}

async function loadGoals() {
  currentGoals = await apiFetch("/api/goals");
  renderGoals(currentGoals);
}

function resetDeleteConfirm() {
  deleteBtn.textContent = "Delete";
  deleteBtn.classList.remove("confirm");
}

function openModal(goal) {
  editingId = goal?.id ?? null;
  modalTitle.textContent = editingId ? "Edit goal" : "New goal";
  form.title.value = goal?.title ?? "";
  form.notes.value = goal?.notes ?? "";
  form.timeframe.value = goal?.timeframe ?? "week";
  form.target_date.value = goal?.target_date ?? "";
  deleteBtn.classList.toggle("hidden", !editingId);
  resetDeleteConfirm();
  modalBackdrop.classList.remove("hidden");
}

function closeModal() {
  modalBackdrop.classList.add("hidden");
  editingId = null;
  form.reset();
  resetDeleteConfirm();
}

document.getElementById("new-goal").addEventListener("click", () => openModal(null));
document.getElementById("cancel-modal").addEventListener("click", closeModal);
modalBackdrop.addEventListener("click", (e) => {
  if (e.target === modalBackdrop) closeModal();
});

listEl.addEventListener("click", async (e) => {
  const checkbox = e.target.closest(".toggle");
  if (checkbox) {
    const id = checkbox.dataset.id;
    const action = checkbox.checked ? "complete" : "uncomplete";
    try {
      await apiFetch(`/api/goals/${id}/${action}`, { method: "POST" });
      await loadGoals();
    } catch (err) {
      checkbox.checked = !checkbox.checked;
      reportError(err, "updating");
    }
    return;
  }

  const body = e.target.closest(".body");
  if (body) {
    const goal = currentGoals.find((g) => g.id === body.dataset.id);
    if (goal) openModal(goal);
  }
});

form.addEventListener("submit", async (e) => {
  e.preventDefault();
  const payload = {
    title: form.title.value.trim(),
    notes: form.notes.value.trim() || null,
    timeframe: form.timeframe.value,
    target_date: form.target_date.value || null,
  };
  if (!payload.title) return;

  try {
    if (editingId) {
      await apiFetch(`/api/goals/${editingId}`, { method: "PUT", body: JSON.stringify(payload) });
    } else {
      await apiFetch("/api/goals", { method: "POST", body: JSON.stringify(payload) });
    }
    closeModal();
    await loadGoals();
  } catch (err) {
    reportError(err);
  }
});

deleteBtn.addEventListener("click", async () => {
  if (!editingId) return;
  if (!deleteBtn.classList.contains("confirm")) {
    deleteBtn.textContent = "Tap again to delete";
    deleteBtn.classList.add("confirm");
    return;
  }
  try {
    await apiFetch(`/api/goals/${editingId}`, { method: "DELETE" });
    closeModal();
    await loadGoals();
  } catch (err) {
    reportError(err, "deleting");
  }
});

try {
  await loadGoals();
} catch (err) {
  console.error(err);
  emptyEl.textContent = "Couldn't load your goals. Check your connection and reload the page.";
  emptyEl.classList.remove("hidden");
}
