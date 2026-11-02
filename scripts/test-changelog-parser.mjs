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
assert.equal(entries.length, 3, "expected 3 releases (0.2.1, 0.2.0, 0.1.0)");

// Verify latest version matches package.json
assert.equal(
  entries[0].version,
  pkg.version,
  "top changelog version must match package.json version",
);
assert.equal(entries[0].version, "0.2.1");
assert.equal(entries[0].date, "November 2026");
assert(
  entries[0].title.includes("Music Studio KH"),
  "0.2.1 title mentions Music Studio KH",
);
assert.equal(entries[0].items.length, 3, "0.2.1 has 3 bullet items");

// Verify 0.2.0
assert.equal(entries[1].version, "0.2.0");
assert.equal(entries[1].date, "October 2026");
assert.equal(entries[1].items.length, 6, "0.2.0 has 6 bullet items");

// Verify 0.1.0
assert.equal(entries[2].version, "0.1.0");
assert.equal(entries[2].date, "October 2026");
assert.equal(entries[2].items.length, 7, "0.1.0 has 7 bullet items");

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
  "PASS: test-changelog-parser (parsed 3 releases from CHANGELOG.md; top version " +
    entries[0].version +
    " matches package.json).",
);
