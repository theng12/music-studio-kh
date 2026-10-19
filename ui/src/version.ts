export const APP_VERSION = "0.2.0";

export interface ReleaseEntry {
  version: string;
  date: string;
  title: string;
  items: string[];
}

export const WHATS_NEW: ReleaseEntry[] = [
  {
    version: "0.2.0",
    date: "October 2026",
    title: "Smart duplicate skipping, full player controls & lyrics",
    items: [
      "Smart duplicate-skipping music & folder imports (skips existing songs automatically without creating -2 copies).",
      "Full music player controls: Shuffle (🔀), Repeat (Off / All / One), ±10s seek buttons, playback speed (0.75×–2×), mute toggle, and Up Next queue.",
      "Built-in Lyrics drawer (🎤 Lyrics) with live [mm:ss] LRC synchronized line highlighting, click-to-seek, and inline lyrics editor.",
      "Non-overlapping sidebar layout + compact Mini-Player mode (⧉), Pin on Top (📌), Hide to Menu Bar (🙈), and native macOS Menu Bar & Tray music controller.",
      "Reorganized sidebar usage navigation (Songs, Albums, Artists, Folders with live item counts, Downloads, and Settings containing Appearance & access).",
      "Red close button background playback vs. quit preference with prompt dialog and Settings option (keeps music playing smoothly in background or quits on demand).",
    ],
  },
  {
    version: "0.1.0",
    date: "October 2026",
    title: "Your music harbor, ready at home",
    items: [
      "Multiple independent server profiles with separate or shared music folders, ports, and logs.",
      "Folder-preserving imports and drag-and-drop (MP3, M4A, AAC, FLAC, Ogg, Opus, WAV) with automatic folder cover art and album/artist/track inference.",
      "Custom import & metadata editor: import or edit songs and albums with custom titles, artists, album names, versions/editions (Deluxe, Acoustic, Live, Remastered), release years, and cover artwork.",
      "Persistent YouTube song and playlist download queues powered by bundled yt-dlp, Deno, and static LGPL FFmpeg.",
      "Subsonic / OpenSubsonic mobile compatibility (Amperfy, Symfonium) over local Wi-Fi with OS Keychain secret encryption.",
      "Industry-standard Profile, Preferences & Settings hub (account credentials, streaming quality, playback repeat mode, theme, default library view, and sign-out confirmation dialog).",
      "Refined button spacing, modal actions, responsive layouts, version badge, and in-app What’s new release notes.",
    ],
  },
];

