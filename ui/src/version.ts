export const APP_VERSION = "0.1.0";

export interface ReleaseEntry {
  version: string;
  date: string;
  title: string;
  items: string[];
}

export const WHATS_NEW: ReleaseEntry[] = [
  {
    version: "0.1.0",
    date: "October 2026",
    title: "Your music harbor, ready at home",
    items: [
      "Multiple independent server profiles with separate or shared music folders, ports, and logs.",
      "Drag-and-drop audio imports (MP3, M4A, AAC, FLAC, Ogg, Opus, WAV) with safe collision numbering and exFAT/FAT support.",
      "Persistent YouTube song and playlist download queues powered by bundled yt-dlp, Deno, and static LGPL FFmpeg.",
      "Built-in desktop and browser player with custom song and album artwork, instant search, and theme/accent/density settings.",
      "Subsonic / OpenSubsonic mobile compatibility (Amperfy, Symfonium) over local Wi-Fi with OS Keychain secret encryption.",
      "Refined button spacing, modal actions, responsive layouts, version badge, and in-app What’s new release notes.",
    ],
  },
];
