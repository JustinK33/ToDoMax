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

let editingId = null;
let currentTasks = [];
const state = { view: "today", category: "" };

function escapeHtml(s) {
  const div = document.createElement("div");
  div.textContent = s;
  return div.innerHTML;
}

function renderTasks(tasks) {
  listEl.innerHTML = "";
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

async function loadTasks() {
  const params = new URLSearchParams({ view: state.view });
  if (state.category) params.set("category", state.category);
  currentTasks = await apiFetch(`/api/tasks?${params}`);
  renderTasks(currentTasks);
}

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

viewTabs.addEventListener("click", async (e) => {
  const btn = e.target.closest("button[data-view]");
  if (!btn) return;
  state.view = btn.dataset.view;
  for (const b of viewTabs.querySelectorAll("button")) {
    b.classList.toggle("active", b === btn);
  }
  await loadTasks();
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
  deleteBtn.classList.toggle("hidden", !editingId);
  modalBackdrop.classList.remove("hidden");
}

function closeModal() {
  modalBackdrop.classList.add("hidden");
  editingId = null;
  form.reset();
  setSelectedDays([]);
  updateRecurrenceDaysVisibility();
}

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
  };
  if (!payload.title) return;
  if (payload.recurrence_type === "weekly" && payload.recurrence_days.length === 0) {
    alert("Pick at least one day for a weekly repeat.");
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
});

deleteBtn.addEventListener("click", async () => {
  if (!editingId) return;
  await apiFetch(`/api/tasks/${editingId}`, { method: "DELETE" });
  closeModal();
  await loadTasks();
  await refreshCategories();
});

await loadTasks();
await refreshCategories();
