import React, { useEffect, useState } from "react";
import { open } from "@tauri-apps/plugin-dialog";
import { enable, disable } from "@tauri-apps/plugin-autostart";
import { invoke } from "@tauri-apps/api/core";
import { getCurrentWebviewWindow } from "@tauri-apps/api/webviewWindow";
import { Profile, rpc } from "./api";
export default function Profiles({ notice }: { notice: (s: string) => void }) {
  const [profiles, setProfiles] = useState<Profile[]>([]),
    [creating, setCreating] = useState(false),
    [busy, setBusy] = useState(false),
    [editing, setEditing] = useState<Profile | null>(null);
  const [mobilePassword, setMobilePassword] = useState("");
  const [name, setName] = useState("Personal"),
    [library, setLibrary] = useState(""),
    [username, setUsername] = useState("owner"),
    [password, setPassword] = useState(""),
    [lan, setLAN] = useState(false);
  const refresh = async () => setProfiles(await rpc("profiles"));
  useEffect(() => {
    rpc("doctor").catch((e) => notice(String(e)));
    refresh().catch((e) => notice(String(e)));
    const timer = setInterval(() => refresh().catch(() => {}), 5000);
    return () => clearInterval(timer);
  }, []);
  const importing = profiles.some((p) => p.importing);
  useEffect(() => {
    if (!importing) return;
    const timer = setInterval(() => refresh().catch(() => {}), 1000);
    return () => clearInterval(timer);
  }, [importing]);
  useEffect(() => {
    let disposed = false,
      unlisten: (() => void) | undefined;
    getCurrentWebviewWindow()
      .onDragDropEvent(async (e) => {
        if (e.payload.type !== "drop") return;
        const position = e.payload.position;
        const card = document
          .elementFromPoint(
            position.x / devicePixelRatio,
            position.y / devicePixelRatio,
          )
          ?.closest<HTMLElement>("[data-profile]");
        const id = card?.dataset.profile;
        if (!id) {
          notice(
            "Drop music files or folders onto the profile you want to import into.",
          );
          return;
        }
        try {
          await rpc("start", { id });
          await rpc("import", { id, paths: e.payload.paths });
          notice(
            "Importing music. Songs appear in the library as they are copied.",
          );
          await refresh();
        } catch (error) {
          notice(String(error));
        }
      })
      .then((fn) => {
        if (disposed) fn();
        else unlisten = fn;
      });
    return () => {
      disposed = true;
      unlisten?.();
    };
  }, []);
  const listen = async (id: string) => {
    setBusy(true);
    try {
      await rpc("doctor");
      const opened = await rpc("start", { id });
      await invoke("open_profile", { opened });
      await refresh();
    } catch (e) {
      notice(String(e));
    } finally {
      setBusy(false);
    }
  };
  const choose = async () => {
    try {
      const path = await open({
        directory: true,
        multiple: false,
        title: "Choose your music folder",
      });
      if (typeof path === "string") setLibrary(path);
    } catch (e) {
      notice(String(e));
    }
  };
  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      await rpc("doctor");
      const p: Profile = await rpc("create", {
        name,
        library,
        username,
        password,
        lan,
        port: 0,
      });
      setPassword("");
      setCreating(false);
      await refresh();
      await listen(p.id);
    } catch (e) {
      notice(String(e));
    } finally {
      setBusy(false);
    }
  };
  const stop = async (p: Profile) => {
    try {
      await rpc("stop", { id: p.id });
      await refresh();
    } catch (e) {
      notice(String(e));
    }
  };
  const remove = async (p: Profile) => {
    if (
      !confirm(
        "Remove " +
          p.name +
          " and its private settings? Your music files will remain.",
      )
    )
      return;
    try {
      await rpc("remove", { id: p.id, confirmed: true });
      const rows: Profile[] = await rpc("profiles");
      setProfiles(rows);
      // Launch at sign-in only while some remaining profile asks for it.
      if (!rows.some((r) => r.autoStart)) await disable().catch(() => {});
    } catch (e) {
      notice(String(e));
    }
  };
  const relocate = async (p: Profile) => {
    try {
      const path = await open({
        directory: true,
        multiple: false,
        title: "Choose a replacement music folder",
      });
      if (typeof path !== "string" || path === p.library) return;
      if (
        !confirm(
          "Use " +
            path +
            " for " +
            p.name +
            "? The profile restarts and indexes the new folder. Files in the old folder stay where they are.",
        )
      )
        return;
      setBusy(true);
      await rpc("relocate", { id: p.id, library: path });
      setEditing(null);
      await refresh();
      notice("Music folder changed. Press Listen to open the library.");
    } catch (e) {
      notice(String(e));
    } finally {
      setBusy(false);
    }
  };
  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editing) return;
    setBusy(true);
    try {
      if (mobilePassword) {
        await rpc("enable_mobile", {
          id: editing.id,
          password: mobilePassword,
        });
        setMobilePassword("");
      }
      await rpc("rename", { id: editing.id, name: editing.name });
      await rpc("configure", {
        id: editing.id,
        port: editing.port,
        lan: editing.lan,
        autoStart: editing.autoStart,
      });
      const rows: Profile[] = await rpc("profiles");
      try {
        if (rows.some((p) => p.autoStart)) await enable();
        else await disable();
      } catch (e) {
        const previous = profiles.find((p) => p.id === editing.id);
        if (previous)
          await rpc("configure", {
            id: previous.id,
            port: editing.port,
            lan: editing.lan,
            autoStart: previous.autoStart,
          });
        throw e;
      }
      setProfiles(rows);
      setEditing(null);
      notice("Profile settings saved. Press Listen to open it.");
    } catch (e) {
      notice(String(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <main className="profile-manager">
      <div className="brand">
        <span className="brand-mark">♪</span>TuneHarbor
      </div>
      <header className="manager-heading">
        <div>
          <span className="eyebrow">YOUR MUSIC, AT HOME</span>
          <h1>A harbor for every collection.</h1>
          <p>Start a library, share one, or keep a little space of your own.</p>
        </div>
        <button className="primary" onClick={() => setCreating(true)}>
          ＋ New profile
        </button>
      </header>
      {!profiles.length && !creating && (
        <div className="hero manager-welcome">
          <div>
            <h2>Let’s make room for your music.</h2>
            <p>
              Choose a folder and a simple password. TuneHarbor handles the
              server for you.
            </p>
            <button className="primary" onClick={() => setCreating(true)}>
              Create your first profile ↗
            </button>
          </div>
          <div className="hero-art">
            <div className="small-record" />
            <span className="hero-note">♫</span>
          </div>
        </div>
      )}
      <div className="profile-grid">
        {profiles.map((p) => (
          <article
            data-profile={p.id}
            key={p.id}
            className="settings-card profile-card"
          >
            <div className="profile-card-top">
              <span className="profile-avatar">
                {p.name.slice(0, 2).toUpperCase()}
              </span>
              <span
                className={"state-pill " + (p.running ? "completed" : "queued")}
              >
                {p.running ? "Running" : "Stopped"}
              </span>
            </div>
            <h2>{p.name}</h2>
            <p className="profile-folder">{p.library}</p>
            <dl>
              <dt>Username</dt>
              <dd>{p.username}</dd>
              <dt>Port</dt>
              <dd>{p.port}</dd>
              <dt>Phone access</dt>
              <dd>
                {!p.lan
                  ? "Off"
                  : p.mobileReady
                    ? "Same Wi-Fi"
                    : "Same Wi-Fi · phone app sign-in needs setup in ⚙︎"}
              </dd>
            </dl>
            {p.error && <p className="error">{p.error}</p>}
            {(p.importing || p.imported > 0 || p.importError) && (
              <p
                className={p.importError ? "error" : "profile-import"}
                role="status"
              >
                {p.importing ? "Importing… " : "Import finished: "}
                {p.imported} {p.imported === 1 ? "song" : "songs"} added
                {p.importSkipped > 0 &&
                  `, ${p.importSkipped} skipped (unreadable or not audio)`}
                {p.importError && ` — ${p.importError}`}
              </p>
            )}
            <div className="profile-actions">
              <button
                className="primary"
                disabled={busy}
                onClick={() => listen(p.id)}
              >
                ▶ Listen
              </button>
              {p.running && (
                <button className="secondary" onClick={() => stop(p)}>
                  Stop
                </button>
              )}
              <button
                className="icon-button"
                aria-label={"Settings for " + p.name}
                onClick={() => setEditing({ ...p })}
              >
                ⚙︎
              </button>
              <button className="subtle" onClick={() => remove(p)}>
                Remove
              </button>
            </div>
          </article>
        ))}
      </div>
      {creating && (
        <div className="modal-backdrop">
          <form className="settings-card profile-form" onSubmit={create}>
            <button
              type="button"
              className="close-dialog"
              aria-label="Close"
              onClick={() => setCreating(false)}
            >
              ×
            </button>
            <span className="eyebrow">A LITTLE SPACE OF YOUR OWN</span>
            <h2>Create a music profile</h2>
            <label>
              Profile name
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                maxLength={80}
              />
            </label>
            <label>
              Music folder
              <div className="folder-input">
                <input
                  value={library}
                  readOnly
                  placeholder="Choose a folder"
                  required
                />
                <button className="secondary" type="button" onClick={choose}>
                  Browse…
                </button>
              </div>
            </label>
            {profiles.length > 0 && (
              <label>
                Or share an existing library
                <select value="" onChange={(e) => setLibrary(e.target.value)}>
                  <option value="">Choose a profile’s music folder…</option>
                  {profiles.map((p) => (
                    <option key={p.id} value={p.library}>
                      {p.name}
                    </option>
                  ))}
                </select>
              </label>
            )}
            <div className="form-columns">
              <label>
                Username
                <input
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  autoComplete="username"
                  required
                />
              </label>
              <label>
                Password
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  minLength={6}
                  autoComplete="new-password"
                  required
                  placeholder="At least 6 characters"
                />
              </label>
            </div>
            <small>
              Numbers are fine. Username capitalization doesn’t matter.
            </small>
            <label className="checkbox">
              <input
                type="checkbox"
                checked={lan}
                onChange={(e) => setLAN(e.target.checked)}
              />
              Allow listening from my phone on the same Wi-Fi
            </label>
            {lan && (
              <small>
                Local-network listening uses HTTP. Enable it on a trusted
                network.
              </small>
            )}
            <button
              className="primary"
              type="submit"
              disabled={busy || !library}
            >
              {busy ? "Creating…" : "Create & listen ↗"}
            </button>
          </form>
        </div>
      )}
      {editing && (
        <div className="modal-backdrop">
          <form className="settings-card profile-form" onSubmit={save}>
            <button
              type="button"
              className="close-dialog"
              aria-label="Close"
              onClick={() => setEditing(null)}
            >
              ×
            </button>
            <span className="eyebrow">YOUR PROFILE</span>
            <h2>Profile settings</h2>
            <label>
              Name
              <input
                value={editing.name}
                onChange={(e) =>
                  setEditing({ ...editing, name: e.target.value })
                }
                required
              />
            </label>
            <label>
              Music folder
              <div className="folder-input">
                <input value={editing.library} readOnly />
                <button
                  className="secondary"
                  type="button"
                  onClick={() => relocate(editing)}
                >
                  Change…
                </button>
              </div>
            </label>
            <label>
              Listening port
              <input
                type="number"
                min="1024"
                max="65535"
                value={editing.port}
                onChange={(e) =>
                  setEditing({ ...editing, port: Number(e.target.value) })
                }
                required
              />
            </label>
            <label className="checkbox">
              <input
                type="checkbox"
                checked={editing.lan}
                onChange={(e) =>
                  setEditing({ ...editing, lan: e.target.checked })
                }
              />
              Allow local-network listening
            </label>
            <label className="checkbox">
              <input
                type="checkbox"
                checked={editing.autoStart}
                onChange={(e) =>
                  setEditing({ ...editing, autoStart: e.target.checked })
                }
              />
              Start this profile when I sign in to my computer
            </label>
            {!editing.mobileReady && (
              <label>
                Enable phone app sign-in
                <input
                  type="password"
                  value={mobilePassword}
                  onChange={(e) => setMobilePassword(e.target.value)}
                  placeholder="Enter the current profile password"
                  autoComplete="current-password"
                />
                <small>
                  Phone apps such as Amperfy need this profile's password saved,
                  encrypted, in your computer's keychain. If saving failed
                  before, unlock the keychain and enter the password here.
                </small>
              </label>
            )}
            <small>
              Changes stop this profile so its access settings can take effect.
              Other profiles keep running.
            </small>
            <button className="primary" disabled={busy} type="submit">
              Save profile
            </button>
          </form>
        </div>
      )}
      <p className="manager-footer">
        Drop music files or folders onto a profile to import them. Closing the
        window keeps your servers running in the tray. Use Quit to stop them.
      </p>
    </main>
  );
}
