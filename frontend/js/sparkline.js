// Minimal inline-SVG line chart, no dependency. Renders an HTML string for
// direct innerHTML assignment from ascending {date, value} points.
export function sparkline(points, { width = 280, height = 64 } = {}) {
  if (points.length < 2) {
    return `<p class="empty-state" style="padding:0.75rem">Not enough data yet.</p>`;
  }

  const values = points.map((p) => p.value);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = max - min || 1;
  const stepX = width / (points.length - 1);

  const coords = points.map((p, i) => {
    const x = i * stepX;
    const y = height - ((p.value - min) / range) * (height - 8) - 4;
    return `${x.toFixed(1)},${y.toFixed(1)}`;
  });
  const last = coords.at(-1).split(",");

  return `
    <svg viewBox="0 0 ${width} ${height}" width="100%" height="${height}" preserveAspectRatio="none">
      <polyline points="${coords.join(" ")}" fill="none" stroke="var(--accent)" stroke-width="2" />
      <circle cx="${last[0]}" cy="${last[1]}" r="3" fill="var(--accent)" />
    </svg>`;
}
