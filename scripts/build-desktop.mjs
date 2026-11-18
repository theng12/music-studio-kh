// The release build of the desktop bundle. Rust bakes its absolute source paths
// into the binary unless they are remapped, so every build maps them away;
// scripts/build-mac.sh then scans the shipped DMG and fails if any come back.
import { execFileSync } from "node:child_process";
import { join } from "node:path";
import { homedir } from "node:os";

const home = homedir();
const rustflags = [
  process.env.RUSTFLAGS ?? "",
  // rustc lets the last matching prefix win, so the registry rule comes last:
  // it must also remove the ".cargo/registry" token, not just the owner name.
  `--remap-path-prefix=${home}=/home/owner`,
  `--remap-path-prefix=${home}/.cargo/registry=/rust/registry`,
].join(" ").trim();

execFileSync(join("node_modules", ".bin", "tauri"), ["build", ...process.argv.slice(2)], {
  stdio: "inherit",
  env: { ...process.env, RUSTFLAGS: rustflags },
});
