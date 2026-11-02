// Shared display formatters. Unknown stays "Unknown"; a real zero stays "0B".
export function formatBytes(value?: number | null): string {
  if (value == null || !Number.isFinite(value)) return "Unknown";
  if (value === 0) return "0B";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let n = value;
  let unit = 0;
  while (n >= 1024 && unit < units.length - 1) {
    n /= 1024;
    unit++;
  }
  if (unit === 0) return Math.round(n) + "B";
  const text = n >= 100 ? n.toFixed(0) : n.toFixed(1);
  return (text.endsWith(".0") ? text.slice(0, -2) : text) + " " + units[unit];
}

export function formatDuration(seconds?: number | null): string {
  // A non-positive duration is missing metadata, not a measured 0:00.
  if (seconds == null || !Number.isFinite(seconds) || seconds <= 0)
    return "Unknown";
  const total = Math.round(seconds);
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const mm = h > 0 ? String(m).padStart(2, "0") : String(m);
  return (h > 0 ? h + ":" : "") + mm + ":" + String(s).padStart(2, "0");
}
