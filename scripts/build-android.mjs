import { spawnSync } from "node:child_process";
import {
  mkdir,
  readFile,
  writeFile,
  copyFile,
  access,
  rename,
} from "node:fs/promises";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { homedir } from "node:os";

export function hostNames(values) {
  if (!values.length)
    throw new Error(
      "Name each permitted HTTP host with --host HOST (no scheme or port)",
    );
  return [
    ...new Set(
      values.map((value) => {
        const host = value.toLowerCase();
        const url = new URL(`http://${host}`);
        if (
          url.hostname !== host ||
          !/^[a-z0-9.-]+$/.test(host) ||
          url.port ||
          url.username ||
          url.password ||
          url.pathname !== "/" ||
          url.search ||
          url.hash ||
          host.endsWith(".") ||
          host.includes("..")
        )
          throw new Error(`Invalid exact host: ${value}`);
        return host;
      }),
    ),
  ];
}

export function networkConfig(hosts) {
  const domains = [
    ...new Set(["tauri.localhost", "ipc.localhost", ...hostNames(hosts)]),
  ];
  return `<?xml version="1.0" encoding="utf-8"?>\n<network-security-config>\n  <base-config cleartextTrafficPermitted="false" />\n  <domain-config cleartextTrafficPermitted="true">\n${domains.map((host) => `    <domain includeSubdomains="false">${host}</domain>`).join("\n")}\n  </domain-config>\n</network-security-config>\n`;
}

async function main() {
  const args = process.argv.slice(2),
    values = [];
  let targets = ["aarch64", "armv7"];
  for (let i = 0; i < args.length; i++) {
    if (args[i] === "--host" && args[i + 1]) values.push(args[++i]);
    else if (args[i] === "--target" && args[i + 1])
      targets = args[++i].split(",");
    else throw new Error(`Unknown or incomplete option: ${args[i]}`);
  }
  if (
    !targets.length ||
    targets.some(
      (target) => !["aarch64", "armv7", "x86_64", "i686"].includes(target),
    )
  )
    throw new Error("Invalid Android target");
  const hosts = hostNames(values);
  const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
  process.chdir(root);
  const env = {
    ...process.env,
    MUSIC_ANDROID_HTTP_HOSTS: JSON.stringify(hosts),
  };
  const run = (command, commandArgs) => {
    const result = spawnSync(command, commandArgs, { env, stdio: "inherit" });
    if (result.error) throw result.error;
    if (result.status !== 0)
      throw new Error(`${command} exited ${result.status ?? result.signal}`);
  };
  try {
    await access("src-tauri/gen/android/app/src/main/AndroidManifest.xml");
  } catch {
    run("npx", ["tauri", "android", "init", "--ci"]);
  }
  const main = "src-tauri/gen/android/app/src/main";
  await mkdir(`${main}/res/xml`, { recursive: true });
  await writeFile(
    `${main}/res/xml/music_network_security.xml`,
    networkConfig(hosts),
  );
  const manifestPath = `${main}/AndroidManifest.xml`;
  let manifest = await readFile(manifestPath, "utf8");
  manifest = manifest
    .replace(/\sandroid:usesCleartextTraffic="[^"]*"/g, "")
    .replace(/\sandroid:networkSecurityConfig="[^"]*"/g, "");
  manifest = manifest.replace(
    "<application",
    '<application android:usesCleartextTraffic="false" android:networkSecurityConfig="@xml/music_network_security"',
  );
  if (!manifest.includes("android.hardware.touchscreen"))
    manifest = manifest.replace(
      "<application",
      '<uses-feature android:name="android.hardware.touchscreen" android:required="false" />\n    <application',
    );
  if (!manifest.includes("android.software.leanback"))
    manifest = manifest.replace(
      "<application",
      '<uses-feature android:name="android.software.leanback" android:required="false" />\n    <application',
    );
  if (!manifest.includes("android.intent.category.LEANBACK_LAUNCHER"))
    manifest = manifest.replace(
      '<category android:name="android.intent.category.LAUNCHER"',
      '<category android:name="android.intent.category.LEANBACK_LAUNCHER" />\n                <category android:name="android.intent.category.LAUNCHER"',
    );
  await writeFile(manifestPath, manifest);
  await copyFile(
    "src-tauri/android/MainActivity.kt",
    `${main}/java/app/tuneharbor/android/MainActivity.kt`,
  );
  // Generated mobile projects are ignored; recreate these changes on every build.
  run("npx", [
    "tauri",
    "android",
    "build",
    "--debug",
    "--apk",
    "--ci",
    "--target",
    ...targets,
  ]);
  const pkg = JSON.parse(await readFile("package.json", "utf8"));
  const output = resolve(
    homedir(),
    "Desktop/All YouTube Stuff/Music Studio KH/Android",
  );
  await mkdir(output, { recursive: true });
  const apk = `${output}/Music Studio KH-${pkg.version}-debug.apk`;
  const retired = resolve(
    homedir(),
    "Developer/_trash/tuneharbor/android",
    new Date().toISOString().replaceAll(":", "-"),
  );
  for (const file of [apk, `${apk}.json`]) {
    try {
      await access(file);
    } catch (error) {
      if (error.code === "ENOENT") continue;
      throw error;
    }
    await mkdir(retired, { recursive: true });
    await rename(file, resolve(retired, file.slice(file.lastIndexOf("/") + 1)));
    console.log(`Previous debug output kept in ${retired}`);
  }
  await copyFile(
    "src-tauri/gen/android/app/build/outputs/apk/universal/debug/app-universal-debug.apk",
    apk,
  );
  await writeFile(
    `${apk}.json`,
    JSON.stringify(
      {
        version: pkg.version,
        targets,
        httpHosts: hosts,
        build: "debug",
        physicalHardwareTested: false,
      },
      null,
      2,
    ) + "\n",
  );
  console.log(`APK: ${apk}`);
}

if (
  process.argv[1] &&
  resolve(process.argv[1]) === fileURLToPath(import.meta.url)
)
  main().catch((error) => {
    console.error(error.message);
    process.exitCode = 1;
  });
