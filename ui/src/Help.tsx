import React from "react";
import { playerShortcuts } from "./playerShortcuts";

export default function Help() {
  return (
    <section id="help-page" className="settings-card">
      <h2>Keyboard shortcuts</h2>
      <p className="muted">
        Use these while listening. Shortcuts pause while you type in a text
        field or use a dialog.
      </p>
      <dl className="keyboard-shortcuts">
        {playerShortcuts.map(({ action, label, description }) => (
          <div key={action}>
            <dt>{description}</dt>
            <dd>
              <kbd>{label}</kbd>
            </dd>
          </div>
        ))}
      </dl>
      <p className="muted">
        Use ⌘ on Mac or Ctrl on Windows and Linux. Your keyboard’s media
        play/pause, next and previous keys also work when the operating system
        sends them to Music Studio KH.
      </p>
      <p className="muted">
        Click speed or volume in the player for more controls. The collapse
        button opens mini player and hide-to-menu-bar options.
      </p>
    </section>
  );
}
