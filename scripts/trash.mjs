// One trash (owner 2026-11-18): ~/Desktop/_trash/music-studio-kh, never the retired Developer
// root. The folder is made at use, never at import, and a path that is not a real directory
// (for example a symlink) stops the run instead of being written through.
import { homedir } from "node:os";
import { join } from "node:path";
import { lstat, mkdir } from "node:fs/promises";

export async function trashPath(...parts) {
  const root = join(homedir(), "Desktop", "_trash", "music-studio-kh");
  try {
    const info = await lstat(root);
    if (!info.isDirectory()) {
      throw new Error(`refusing to write through ${root}: it is not a directory`);
    }
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
  }
  const target = join(root, ...parts);
  await mkdir(target, { recursive: true });
  return target;
}
