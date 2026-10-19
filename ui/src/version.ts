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
      "Folder-preserving imports and drag-and-drop (MP3, M4A, AAC, FLAC, Ogg, Opus, WAV) with automatic folder cover art and album/artist/track inference.",
      "Custom import & metadata editor: import or edit songs and albums with custom titles, artists, album names, versions/editions (Deluxe, Acoustic, Live, Remastered), release years, and cover artwork.",
      "Persistent YouTube song and playlist download queues powered by bundled yt-dlp, Deno, and static LGPL FFmpeg.",
      "Subsonic / OpenSubsonic mobile compatibility (Amperfy, Symfonium) over local Wi-Fi with OS Keychain secret encryption.",
      "Refined button spacing, modal actions, responsive layouts, version badge, and in-app What’s new release notes.",
    ],
  },
];

