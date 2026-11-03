export const playerShortcuts = [
  {
    action: "play",
    key: " ",
    command: false,
    label: "Space",
    description: "Play / pause",
  },
  {
    action: "back",
    key: "ArrowLeft",
    command: false,
    label: "←",
    description: "Back 10 seconds",
  },
  {
    action: "forward",
    key: "ArrowRight",
    command: false,
    label: "→",
    description: "Forward 10 seconds",
  },
  {
    action: "previous",
    key: "ArrowLeft",
    command: true,
    label: "⌘← / Ctrl+←",
    description: "Previous song",
  },
  {
    action: "next",
    key: "ArrowRight",
    command: true,
    label: "⌘→ / Ctrl+→",
    description: "Next song",
  },
  {
    action: "volumeUp",
    key: "ArrowUp",
    command: true,
    label: "⌘↑ / Ctrl+↑",
    description: "Volume up",
  },
  {
    action: "volumeDown",
    key: "ArrowDown",
    command: true,
    label: "⌘↓ / Ctrl+↓",
    description: "Volume down",
  },
  {
    action: "mute",
    key: "m",
    command: false,
    label: "M",
    description: "Mute / unmute",
  },
  {
    action: "shuffle",
    key: "s",
    command: false,
    label: "S",
    description: "Shuffle on / off",
  },
  {
    action: "repeat",
    key: "r",
    command: false,
    label: "R",
    description: "Cycle repeat mode",
  },
  {
    action: "lyrics",
    key: "l",
    command: false,
    label: "L",
    description: "Show / hide lyrics",
  },
  {
    action: "queue",
    key: "q",
    command: false,
    label: "Q",
    description: "Show / hide queue",
  },
  {
    action: "fullscreen",
    key: "f",
    command: false,
    label: "F",
    description: "Enter / exit fullscreen",
  },
] as const;

export const shortcutTitle = (action: string, title: string) =>
  `${title} (${playerShortcuts.find((shortcut) => shortcut.action === action)?.label})`;

export function playerKeyAction(event: KeyboardEvent) {
  const target = event.target;
  if (
    event.defaultPrevented ||
    event.isComposing ||
    event.altKey ||
    event.shiftKey ||
    (target instanceof HTMLElement &&
      (target.isContentEditable ||
        target.closest(
          'input:not([type="range"]), textarea, select, [role="textbox"], [role="combobox"]',
        ))) ||
    document.querySelector('[aria-modal="true"]')
  )
    return null;
  const shortcut = playerShortcuts.find(
    (item) =>
      item.key.toLowerCase() === event.key.toLowerCase() &&
      item.command === (event.metaKey || event.ctrlKey),
  );
  return shortcut?.action ?? null;
}
