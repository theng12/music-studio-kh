import assert from "node:assert/strict";
import { shuffledQueue } from "../ui/src/playback.ts";
const album = [{ id: "a" }, { id: "b" }, { id: "c" }];
const input = [...album, album[0]];
for (let i = 0; i < 20; i++) {
  const queue = shuffledQueue(input);
  assert.equal(queue.length, 3, "each selected song occurs once");
  assert.deepEqual(
    new Set(queue.map((t) => t.id)),
    new Set(["a", "b", "c"]),
    "shuffle stays within selected scope",
  );
  assert.deepEqual(input, [...album, album[0]], "original queue is unchanged");
}
assert.deepEqual(shuffledQueue([]), []);
assert.deepEqual(shuffledQueue([album[0]]), [album[0]]);
console.log("PASS: shuffle preserves scope, unique songs and source queue");
