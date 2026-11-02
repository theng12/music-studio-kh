import assert from "node:assert/strict";
import { formatBytes, formatDuration } from "../ui/src/format.ts";

// Bytes: real zero stays "0B", unknown stays "Unknown", adaptive binary units.
assert.equal(formatBytes(0), "0B");
assert.equal(formatBytes(null), "Unknown");
assert.equal(formatBytes(undefined), "Unknown");
assert.equal(formatBytes(Number.NaN), "Unknown");
assert.equal(formatBytes(512), "512B");
assert.equal(formatBytes(1023), "1023B");
assert.equal(formatBytes(1024), "1 KiB");
assert.equal(formatBytes(1536), "1.5 KiB");
assert.equal(formatBytes(1024 ** 2), "1 MiB");
assert.equal(formatBytes(1024 ** 3 * 2), "2 GiB");

// Duration: non-positive or missing is unknown metadata, never a fake 0:00.
assert.equal(formatDuration(0), "Unknown");
assert.equal(formatDuration(-5), "Unknown");
assert.equal(formatDuration(null), "Unknown");
assert.equal(formatDuration(undefined), "Unknown");
assert.equal(formatDuration(65), "1:05");
assert.equal(formatDuration(59), "0:59");
assert.equal(formatDuration(3600), "1:00:00");
assert.equal(formatDuration(3661), "1:01:01");

console.log("PASS: formatBytes / formatDuration honesty rules");
