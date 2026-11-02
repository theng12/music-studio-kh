import React from "react";

const paths: Record<string, React.ReactNode> = {
  music: (
    <>
      <path d="M9 18V5l12-2v13M9 8l12-2" />
      <ellipse cx="6" cy="18" rx="3" ry="3" />
      <ellipse cx="18" cy="16" rx="3" ry="3" />
    </>
  ),
  album: (
    <>
      <circle cx="12" cy="12" r="9" />
      <circle cx="12" cy="12" r="2" />
      <path d="M7 8a6 6 0 0 1 4-2m2 12a6 6 0 0 0 4-2" />
    </>
  ),
  artist: (
    <>
      <circle cx="12" cy="8" r="4" />
      <path d="M4 21v-2a8 8 0 0 1 16 0v2" />
    </>
  ),
  folder: (
    <path d="M3 6a2 2 0 0 1 2-2h5l2 3h7a2 2 0 0 1 2 2v10a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1Z" />
  ),
  download: (
    <>
      <path d="M12 3v12m-5-5 5 5 5-5M4 16v4h16v-4" />
    </>
  ),
  settings: (
    <>
      <path d="m10 3-1 3-3 1-3 3 2 2-1 4 3 2 3-1 2 4 3-1 1-3 4-1 1-3-3-2 1-4-3-2-3 1Z" />
      <circle cx="12" cy="12" r="3" />
    </>
  ),
  play: <path d="m8 4 12 8-12 8Z" fill="currentColor" stroke="none" />,
  pause: (
    <>
      <path d="M6 4h4v16H6zm8 0h4v16h-4z" fill="currentColor" stroke="none" />
    </>
  ),
  next: (
    <>
      <path d="m5 5 10 7-10 7Z" fill="currentColor" stroke="none" />
      <path d="M19 5v14" />
    </>
  ),
  previous: (
    <>
      <path d="m19 5-10 7 10 7Z" fill="currentColor" stroke="none" />
      <path d="M5 5v14" />
    </>
  ),
  shuffle: (
    <>
      <path d="M3 6h3c5 0 7 12 12 12h3m-4-4 4 4-4 4M3 18h3c2 0 3-2 5-5m3-4c1-2 2-3 4-3h3m-4-4 4 4-4 4" />
    </>
  ),
  repeat: (
    <>
      <path d="m17 2 4 4-4 4M3 11V8a2 2 0 0 1 2-2h16M7 22l-4-4 4-4m14-1v3a2 2 0 0 1-2 2H3" />
    </>
  ),
  heart: (
    <path d="M20.8 4.6a5.5 5.5 0 0 0-7.8 0L12 5.7l-1.1-1.1a5.5 5.5 0 0 0-7.8 7.8L12 21l8.8-8.6a5.5 5.5 0 0 0 0-7.8Z" />
  ),
  volume: (
    <>
      <path d="m11 4-6 5H2v6h3l6 5Z" />
      <path d="M15 8a6 6 0 0 1 0 8m3-11a10 10 0 0 1 0 14" />
    </>
  ),
  mute: (
    <>
      <path d="m11 4-6 5H2v6h3l6 5Z" />
      <path d="m17 9 5 6m0-6-5 6" />
    </>
  ),
  search: (
    <>
      <circle cx="10.5" cy="10.5" r="7.5" />
      <path d="m16 16 5 5" />
    </>
  ),
  plus: <path d="M12 4v16M4 12h16" />,
  close: <path d="m6 6 12 12M6 18 18 6" />,
  refresh: (
    <>
      <path d="M20 8a8 8 0 1 0 0 8M20 3v5h-5" />
    </>
  ),
  chevron: <path d="m9 5 7 7-7 7" />,
  grid: <path d="M3 3h7v7H3zm11 0h7v7h-7zM3 14h7v7H3zm11 0h7v7h-7z" />,
  list: <path d="M8 5h13M8 12h13M8 19h13M3 5h1m-1 7h1m-1 7h1" />,
  edit: (
    <>
      <path d="m15 4 5 5M4 20l5-1L21 7l-5-5L4 14Z" />
    </>
  ),
  lyrics: <path d="M4 4h16v13H8l-4 4ZM8 8h8M8 12h6" />,
  queue: <path d="M3 5h14M3 10h14M3 15h8m5-1 5 4-5 4Z" />,
  info: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 11v6m0-10v1" />
    </>
  ),
  clock: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 6v6l4 2" />
    </>
  ),
  upload: <path d="M12 16V3m-5 5 5-5 5 5M4 16v4h16v-4" />,
  check: <path d="m4 12 5 5L20 6" />,
  back: <path d="m11 4-8 8 8 8M3 12h18" />,
  expand: <path d="M8 3H3v5m13-5h5v5M3 16v5h5m13-5v5h-5" />,
  pin: <path d="m9 3 12 12-4 1-2 5-5-7-4-1 1-4ZM3 21l6-6" />,
};

export default function Icon({
  name,
  size = 20,
  className = "",
}: {
  name: string;
  size?: number;
  className?: string;
}) {
  return (
    <svg
      className={"icon " + className}
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      {paths[name] || paths.music}
    </svg>
  );
}
