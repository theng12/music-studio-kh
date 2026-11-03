import assert from "node:assert/strict";
import { hostNames, networkConfig } from "./build-android.mjs";
assert.deepEqual(hostNames(["10.0.2.2", "MAC.example", "mac.example"]), [
  "10.0.2.2",
  "mac.example",
]);
for (const value of [
  "*.example",
  "example:4533",
  "https://example",
  "person@example",
  "example/path",
  "example?x=1",
  "example#x",
  "example..com",
  "example.",
  "<evil>",
])
  assert.throws(() => hostNames([value]), value);
assert.throws(() => hostNames([]));
const xml = networkConfig(["10.0.2.2", "mac.example"]);
assert.match(xml, /base-config cleartextTrafficPermitted="false"/);
assert.match(xml, /<domain includeSubdomains="false">mac.example<\/domain>/);
assert.match(xml, /<domain includeSubdomains="false">10.0.2.2<\/domain>/);
assert(!xml.includes('includeSubdomains="true"'));
assert(!xml.includes('<base-config cleartextTrafficPermitted="true"'));
console.log(
  "PASS: exact-host input validation and restricted Android HTTP configuration",
);
