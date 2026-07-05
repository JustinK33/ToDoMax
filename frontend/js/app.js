import { apiFetch } from "./api.js";

const listEl = document.getElementById("task-list");
const emptyEl = document.getElementById("empty-state");
const modalBackdrop = document.getElementById("modal-backdrop");
const form = document.getElementById("task-form");
const deleteBtn = document.getElementById("delete-task");
const modalTitle = document.getElementById("modal-title");

let editingId = null;

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
    li.innerHTML = `
      <input type="checkbox" ${t.completed ? "checked" : ""} data-id="${t.id}" class="toggle" />
      <div class="body" data-id="${t.id}">
        <div class="title">${escapeHtml(t.title)}</div>
        ${due || t.category ? `<div class="meta">${[due, t.category].filter(Boolean).map(escapeHtml).join(" · ")}</div>` : ""}
      </div>
    `;
    listEl.appendChild(li);
  }
}

async function loadTasks() {
  const tasks = await apiFetch("/api/tasks");
  renderTasks(tasks);
}

function openModal(task) {
  editingId = task?.id ?? null;
  modalTitle.textContent = editingId ? "Edit task" : "New task";
  form.title.value = task?.title ?? "";
  form.notes.value = task?.notes ?? "";
  form.category.value = task?.category ?? "";
  form.due_date.value = task?.due_date ?? "";
  form.due_time.value = task?.due_time?.slice(0, 5) ?? "";
  deleteBtn.classList.toggle("hidden", !editingId);
  modalBackdrop.classList.remove("hidden");
}

function closeModal() {
  modalBackdrop.classList.add("hidden");
  editingId = null;
  form.reset();
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
    await apiFetch(`/api/tasks/${id}/${action}`, { method: "POST" });
    await loadTasks();
    return;
  }

  const body = e.target.closest(".body");
  if (body) {
    const tasks = await apiFetch("/api/tasks");
    const task = tasks.find((t) => t.id === body.dataset.id);
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
  };
  if (!payload.title) return;

  if (editingId) {
    await apiFetch(`/api/tasks/${editingId}`, { method: "PUT", body: JSON.stringify(payload) });
  } else {
    await apiFetch("/api/tasks", { method: "POST", body: JSON.stringify(payload) });
  }
  closeModal();
  await loadTasks();
});

deleteBtn.addEventListener("click", async () => {
  if (!editingId) return;
  await apiFetch(`/api/tasks/${editingId}`, { method: "DELETE" });
  closeModal();
  await loadTasks();
});

await loadTasks();
