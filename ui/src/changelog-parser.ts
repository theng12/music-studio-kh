export interface ReleaseEntry {
  version: string;
  date: string;
  title: string;
  items: string[];
}

export function parseChangelog(markdown: string): ReleaseEntry[] {
  const entries: ReleaseEntry[] = [];
  const sections = markdown.split(/^## /m).slice(1);
  for (const section of sections) {
    const lines = section.trim().split("\n");
    const headerLine = lines[0] || "";
    const match = headerLine.match(
      /\[?([0-9]+\.[0-9]+\.[0-9]+)\]?\s*[-—–]\s*(.+)/,
    );
    if (!match) continue;
    const version = match[1];
    const date = match[2].trim();
    let title = "";
    const items: string[] = [];
    for (let i = 1; i < lines.length; i++) {
      const line = lines[i].trim();
      if (line.startsWith("### ")) {
        title = line.slice(4).trim();
      } else if (line.startsWith("- ")) {
        items.push(line.slice(2).trim());
      }
    }
    entries.push({ version, date, title, items });
  }
  return entries;
}
