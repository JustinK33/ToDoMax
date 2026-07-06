// Cross-domain "today at a glance" + weekly review, layered onto the Tasks
// page without touching app.js or any element it owns - this module only
// reads/writes its own dashboard-today/weekly-review elements.
import { apiFetch } from "./api.js";

const dashboardEl = document.getElementById("dashboard-today");
const weeklyReviewEl = document.getElementById("weekly-review");

function card(label, value) {
  return `<div class="dashboard-card"><div class="label">${label}</div><div class="value">${value}</div></div>`;
}

async function loadDashboard() {
  const [nutrition, training, habits, goals] = await Promise.allSettled([
    apiFetch("/api/nutrition/day"),
    apiFetch("/api/training/summary"),
    apiFetch("/api/tasks/habits"),
    apiFetch("/api/goals"),
  ]);

  const cards = [];

  if (nutrition.status === "fulfilled") {
    const entries = nutrition.value.entries ?? [];
    cards.push(card("Nutrition", entries.length ? `${Math.round(nutrition.value.totals.calories)} cal` : "Not logged yet"));
  }

  if (training.status === "fulfilled") {
    const sets = training.value.today_sets ?? [];
    cards.push(card("Training", sets.length ? `${sets.length} sets today` : "Rest day"));
  }

  let habitList = [];
  if (habits.status === "fulfilled") {
    habitList = habits.value;
    const done = habitList.filter((h) => h.completed_today).length;
    cards.push(card("Habits", habitList.length ? `${done}/${habitList.length} done` : "No habits yet"));
  }

  if (goals.status === "fulfilled") {
    const incomplete = goals.value.filter((g) => !g.completed).length;
    cards.push(card("Goals", goals.value.length ? `${incomplete} in progress` : "No goals yet"));
  }

  dashboardEl.innerHTML = cards.join("");

  const reviewParts = [];
  const weekSummary = await apiFetch("/api/summary/week").catch(() => null);
  if (weekSummary) {
    reviewParts.push(`<div class="total">${weekSummary.completed}/${weekSummary.expected} tasks done this week</div>`);
  }
  if (training.status === "fulfilled") {
    reviewParts.push(`<div>${training.value.week_sets} sets &middot; ${Math.round(training.value.week_volume)} lb in the last 7 days</div>`);
  }
  const nutritionHistory = await apiFetch("/api/nutrition/history?days=7").catch(() => null);
  if (nutritionHistory && nutritionHistory.length) {
    const avg = nutritionHistory.reduce((sum, d) => sum + d.calories, 0) / nutritionHistory.length;
    reviewParts.push(`<div>avg ${Math.round(avg)} cal/day this week</div>`);
  }
  if (habitList.length) {
    const longest = Math.max(...habitList.map((h) => h.streak), 0);
    reviewParts.push(`<div>Longest habit streak: ${longest} days</div>`);
  }

  if (reviewParts.length) {
    weeklyReviewEl.innerHTML = reviewParts.join("");
    weeklyReviewEl.classList.remove("hidden");
  }
}

loadDashboard().catch((err) => console.error("dashboard: failed to load", err));
