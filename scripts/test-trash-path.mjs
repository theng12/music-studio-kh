// The one trash: created at use under ~/Desktop/_trash/music-studio-kh, and a path that is
// not a real directory is refused instead of written through.
import assert from "node:assert/strict";
import { mkdtemp, mkdir, readdir, rm, symlink } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { trashPath } from "./trash.mjs";

const home = await mkdtemp(join(tmpdir(), "trash-path-"));
const realHome = process.env.HOME;
try {
  process.env.HOME = home;
  const created = await trashPath("test-fixtures");
  assert.equal(created, join(home, "Desktop", "_trash", "music-studio-kh", "test-fixtures"));
  assert.deepEqual(await readdir(created), [], "the folder is made empty at use");

  const linked = await mkdtemp(join(tmpdir(), "trash-link-"));
  const elsewhere = join(linked, "elsewhere");
  await mkdir(join(linked, "Desktop", "_trash"), { recursive: true });
  await mkdir(elsewhere);
  await symlink(elsewhere, join(linked, "Desktop", "_trash", "music-studio-kh"));
  process.env.HOME = linked;
  await assert.rejects(trashPath("test-fixtures"), /refusing to write through .* not a directory/);
  assert.deepEqual(await readdir(elsewhere), [], "nothing was written through the symlink");
  await rm(linked, { recursive: true, force: true });
} finally {
  process.env.HOME = realHome;
  await rm(home, { recursive: true, force: true });
}
console.log("PASS: trash is created at use under the Desktop, and a symlinked root is refused");
