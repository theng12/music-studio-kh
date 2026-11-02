import React from "react";
import { APP_VERSION, WHATS_NEW } from "./version";

export default function WhatsNew({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  if (!open) return null;
  return (
    <div
      className="modal-backdrop"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <section
        className="settings-card whats-new-dialog"
        id="whats-new-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="whats-new-title"
      >
        <button
          type="button"
          className="close-dialog"
          aria-label="Close what's new"
          onClick={onClose}
        >
          ×
        </button>
        <span className="eyebrow">RELEASE NOTES · V{APP_VERSION}</span>
        <h2 id="whats-new-title">What’s new in Music Studio KH</h2>
        <div className="whats-new-list">
          {WHATS_NEW.map((entry) => (
            <article key={entry.version} className="whats-new-entry">
              <div className="whats-new-header">
                <span className="version-badge">v{entry.version}</span>
                <span className="whats-new-date">{entry.date}</span>
              </div>
              <h3>{entry.title}</h3>
              <ul>
                {entry.items.map((item, i) => (
                  <li key={i}>{item}</li>
                ))}
              </ul>
            </article>
          ))}
        </div>
        <div className="action-row">
          <button type="button" className="primary" onClick={onClose}>
            Got it
          </button>
        </div>
      </section>
    </div>
  );
}
