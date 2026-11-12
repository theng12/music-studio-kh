import React, { useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import Icon from "./Icon";

export default function Support({
  notice,
}: {
  notice: (s: string) => void;
}) {
  const [opening, setOpening] = useState(false);
  const donate = async () => {
    setOpening(true);
    try {
      // The app decides the destination; the browser only shows the shared page.
      await invoke("open_support_page");
    } catch (e) {
      notice(String(e));
    } finally {
      setOpening(false);
    }
  };
  return (
    <section className="support-page" id="support-page">
      <h2>Support this app</h2>
      <p className="support-intro">
        Music Studio KH is free. Every feature is unlocked — no account, no ads,
        nothing held back.
      </p>
      <div className="settings-card support-card">
        <span className="support-heart">
          <Icon name="heart" size={26} />
        </span>
        <h3>If it’s useful to you</h3>
        <p>
          A one-time donation helps keep it going. It is completely optional and
          changes nothing about the app.
        </p>
        <button
          className="primary"
          type="button"
          disabled={opening}
          onClick={donate}
        >
          {opening ? "Opening..." : "Donate once"}
        </button>
        <small>
          Opens nowpayments.io/donation/studiokh in your browser. Optional, with
          no reminders.
        </small>
      </div>
    </section>
  );
}
