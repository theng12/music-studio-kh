import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { invoke } from "@tauri-apps/api/core";
import Icon from "./Icon";
import WhatsNew from "./WhatsNew";
import { APP_VERSION } from "./version";
import "./style.css";
import "./android.css";

type ClientConfig = { httpHosts: string[] };

function Android() {
  const [config, setConfig] = useState<ClientConfig | null>(null);
  const [server, setServer] = useState(
    localStorage.getItem("music-server") || "",
  );
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [notes, setNotes] = useState(false);
  useEffect(() => {
    invoke<ClientConfig>("client_config")
      .then(setConfig)
      .catch(() =>
        setError(
          "Could not load Android connection settings. Close and reopen the app.",
        ),
      );
  }, []);
  return (
    <main className="android-connect">
      <div className="brand">
        <span className="brand-mark">
          <Icon name="music" size={22} />
        </span>
        Music Studio KH
      </div>
      <div className="action-row">
        <span className="version-badge">Android · v{APP_VERSION}</span>
        <button className="subtle" onClick={() => setNotes(true)}>
          What’s new
        </button>
      </div>
      <h1>Listen from your Mac</h1>
      <p>
        Keep Music Studio KH running on your Mac with local-network access
        enabled. Enter the library address shown there, then sign in with that
        library’s username and password.
      </p>
      <form
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          setError("");
          try {
            const address = await invoke<string>("connect", {
              server: server.trim(),
            });
            localStorage.setItem("music-server", address);
            location.assign(address);
          } catch (e) {
            setError(String(e));
            setBusy(false);
          }
        }}
      >
        <label htmlFor="server">Mac library address</label>
        <input
          id="server"
          type="url"
          inputMode="url"
          autoCapitalize="none"
          autoCorrect="off"
          spellCheck={false}
          required
          placeholder="http://Mac-address:port"
          value={server}
          onChange={(e) => setServer(e.target.value)}
        />
        <button className="primary" disabled={busy || !config}>
          {busy ? "Connecting…" : "Connect"}
        </button>
      </form>
      {error && (
        <p role="alert" className="android-error">
          {error}
        </p>
      )}
      <p>
        Music and downloads stay on the Mac. You need a working network
        connection to listen.
      </p>
      <small>
        HTTP hosts allowed in this APK:{" "}
        {config
          ? config.httpHosts.join(", ") || "none"
          : error
            ? "Unavailable"
            : "Loading…"}
        . If your Mac’s address changes, rebuild for its new host. HTTPS uses
        normal certificate checks.
      </small>
      <small>
        Touch to select, or use arrow keys and OK on a remote. Back returns from
        the library to this screen. Background and screen-off playback have not
        been qualified.
      </small>
      <WhatsNew open={notes} onClose={() => setNotes(false)} />
    </main>
  );
}

createRoot(document.getElementById("root")!).render(<Android />);
