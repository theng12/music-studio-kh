import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { parseChangelog } from "../ui/src/changelog-parser.ts";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.resolve(__dirname, "..");

// 1. Verify parsing the real repository CHANGELOG.md
const changelogPath = path.join(rootDir, "CHANGELOG.md");
assert(fs.existsSync(changelogPath), "CHANGELOG.md must exist in repo root");
const changelogRaw = fs.readFileSync(changelogPath, "utf-8");

const pkgPath = path.join(rootDir, "package.json");
const pkg = JSON.parse(fs.readFileSync(pkgPath, "utf-8"));

const entries = parseChangelog(changelogRaw);
assert(Array.isArray(entries), "parseChangelog must return an array");
assert(entries.length >= 3, "retained release history must be present");
assert.equal(new Set(entries.map(entry => entry.version)).size, entries.length, "release versions must be unique");

// Verify latest version matches package.json
assert.equal(
  entries[0].version,
  pkg.version,
  "top changelog version must match package.json version",
);

const topSection = changelogRaw.split(/^## /m)[1].trim().split("\n").filter(line => line.trim());
assert.equal(topSection[0], `${pkg.version} — ${entries[0].date}`, "latest heading uses the release standard");
assert(topSection.slice(1).every(line => /^- \S/.test(line)), "only plain bullets follow the latest heading");
assert(/^\d{4}-\d{2}-\d{2}$/.test(entries[0].date), "latest release date must be YYYY-MM-DD");
assert.equal(entries[0].title, "", "latest release must have no subheading (title is empty)");
assert(
  entries[0].items.length >= 1 && entries[0].items.length <= 6,
  "latest release must have 1 to 6 plain bullets directly under the heading",
);

// Release history survives future version bumps.
for (const [version, date, itemCount] of [
  ["0.2.1", "November 2026", 3],
  ["0.2.0", "October 2026", 6],
  ["0.1.0", "October 2026", 7],
]) {
  const release = entries.find(entry => entry.version === version);
  assert(release, version + " notes retained");
  assert.equal(release.date, date);
  assert.equal(release.items.length, itemCount, version + " bullets retained");
}

// 2. Edge case testing for parser robustness
assert.deepEqual(parseChangelog(""), [], "empty markdown yields empty array");
assert.deepEqual(
  parseChangelog("# Title\n\nNo versions here"),
  [],
  "no version sections yields empty array",
);

const sample = `
## [1.0.0] - January 2027

### Initial major release

- First item
- Second item
`;
const parsedSample = parseChangelog(sample);
assert.equal(parsedSample.length, 1);
assert.equal(parsedSample[0].version, "1.0.0");
assert.equal(parsedSample[0].date, "January 2027");
assert.equal(parsedSample[0].title, "Initial major release");
assert.deepEqual(parsedSample[0].items, ["First item", "Second item"]);

console.log(
  "PASS: changelog parsing, retained history and edge cases (" + entries.length +
    " releases; latest " + entries[0].version + " matches package.json).",
);
