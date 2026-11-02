import pkg from "../../package.json";
import changelogRaw from "../../CHANGELOG.md?raw";
import { parseChangelog, ReleaseEntry } from "./changelog-parser";

export const APP_VERSION: string = pkg.version;
export type { ReleaseEntry };
export { parseChangelog };
export const WHATS_NEW: ReleaseEntry[] = parseChangelog(changelogRaw);
