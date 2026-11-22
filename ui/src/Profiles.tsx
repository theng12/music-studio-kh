import React, { useEffect, useState } from "react";
import { useEscape } from "./useEscape";
import { open } from "@tauri-apps/plugin-dialog";
import { enable, disable } from "@tauri-apps/plugin-autostart";
import { invoke } from "@tauri-apps/api/core";
import { getCurrentWebviewWindow } from "@tauri-apps/api/webviewWindow";
import { Profile, rpc } from "./api";
import { APP_VERSION } from "./version";
import Icon from "./Icon";
export default function Profiles({
  notice,
  openWhatsNew,
}: {
  notice: (s: string) => void;
  openWhatsNew: () => void;
}) {
  const [profiles, setProfiles] = useState<Profile[]>([]),
    [creating, setCreating] = useState(false),
    [busy, setBusy] = useState(false),
    [editing, setEditing] = useState<Profile | null>(null);
  useEscape(creating, () => setCreating(false));
  useEscape(editing !== null, () => setEditing(null));
  const [confirmAsk, setConfirmAsk] = useState<{
    title: string;
    body: string;
    confirmLabel: string;
    destructive?: boolean;
    run: () => Promise<void>;
  } | null>(null);
  useEscape(confirmAsk !== null, () => setConfirmAsk(null));
  const [mobilePassword, setMobilePassword] = useState("");
  // Problems shown where the owner is looking: in the open dialog, or a
  // lasting banner when bundled download tools cannot run.
  const [formError, setFormError] = useState(""),
    [toolProblem, setToolProblem] = useState("");
  const [name, setName] = useState("Personal"),
    [library, setLibrary] = useState(""),
    [defaultLibrary, setDefaultLibrary] = useState(""),
    [useDefaultFolder, setUseDefaultFolder] = useState(true),
    [username, setUsername] = useState("owner"),
    [password, setPassword] = useState(""),
    [lan, setLAN] = useState(false);
  // Paths read plainly: the home folder is shown as ~, everything else in full.
  const home = defaultLibrary.replace(/\/Music\/Music Studio KH$/, "");
  const shownPath = (path: string) =>
    home && path.startsWith(home + "/") ? "~" + path.slice(home.length) : path;
  const refresh = async () => setProfiles(await rpc("profiles"));
  useEffect(() => {
    const manage = (event: Event) => {
      const profile = profiles.find(
        (p) => p.id === (event as CustomEvent<string>).detail,
      );
      if (profile) setEditing({ ...profile });
    };
    // Sent from a profile window's library screen: choose its folder, then reopen it.
    const relocateFromLibrary = (event: Event) => {
      const profile = profiles.find(
        (p) => p.id === (event as CustomEvent<string>).detail,
      );
      if (profile) relocate(profile, true);
      else notice("That profile is no longer listed here. Refresh and try again.");
    };
    // Sent from a profile window's library screen: pick a folder to join into
    // that profile's library; the profile's main folder stays the import one.
    const addLibraryRoot = async (event: Event) => {
      const id = (event as CustomEvent<string>).detail;
      try {
        const path = await open({
          directory: true,
          multiple: false,
          title: "Choose a folder to add to the library",
        });
        if (typeof path !== "string") return;
        await rpc("add_library_root", { id, path });
        notice(
          "Folder added to the library. Its songs are being indexed now.",
        );
      } catch (e) {
        notice(String(e));
      }
    };
    window.addEventListener("manage-profile", manage);
    window.addEventListener("relocate-profile", relocateFromLibrary);
    window.addEventListener("add-library-root", addLibraryRoot);
    return () => {
      window.removeEventListener("manage-profile", manage);
      window.removeEventListener("relocate-profile", relocateFromLibrary);
      window.removeEventListener("add-library-root", addLibraryRoot);
    };
  }, [profiles]);

  useEffect(() => {
    rpc("doctor").catch((e) => setToolProblem(String(e)));
    rpc<{ path: string }>("default_library")
      .then((r) => setDefaultLibrary(r.path))
      .catch((e) => notice(String(e)));
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
            "Drop music files or folders onto a profile to copy them into its library.",
          );
          return;
        }
        try {
          await rpc("start", { id });
          await rpc("import", { id, paths: e.payload.paths });
          notice(
            "Copying into the library. Music stays in the library folder afterwards; to play a folder without copying, point the profile at it instead.",
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
      if (typeof path === "string") {
        setLibrary(path);
        setUseDefaultFolder(false);
      }
    } catch (e) {
      notice(String(e));
    }
  };
  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setFormError("");
    try {
      const p: Profile = await rpc("create", {
        name,
        library: useDefaultFolder ? "" : library,
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
      setFormError(String(e));
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
    setConfirmAsk({
      title: "Remove " + p.name + "?",
      body: "Its private settings are removed. Your music files remain where they are.",
      confirmLabel: "Remove",
      destructive: true,
      run: async () => {
        try {
          await rpc("remove", { id: p.id, confirmed: true });
          const rows: Profile[] = await rpc("profiles");
          setProfiles(rows);
          // Launch at sign-in only while some remaining profile asks for it.
          if (!rows.some((r) => r.autoStart)) await disable().catch(() => {});
        } catch (e) {
          notice(String(e));
        }
      },
    });
  };
  const relocate = async (p: Profile, reopen = false) => {
    try {
      const path = await open({
        directory: true,
        multiple: false,
        title: "Choose the music folder for " + p.name,
      });
      if (typeof path !== "string") return;
      setConfirmAsk({
        title: "Use this folder for " + p.name + "?",
        body:
          path +
          (path === p.library
            ? " is already this profile's folder. The profile restarts and reads it again from scratch; favourites and play counts stay."
            : " — the profile restarts and indexes the new folder, including its subfolders. Files in the old folder stay where they are."),
        confirmLabel: "Use folder",
        run: async () => {
          setBusy(true);
          try {
            await rpc("relocate", { id: p.id, library: path });
            setEditing(null);
            await refresh();
            notice(
              reopen
                ? "Music folder changed. Reopening the library…"
                : "Music folder changed. Press Listen to open the library.",
            );
          } catch (e) {
            notice(String(e));
            return;
          } finally {
            setBusy(false);
          }
          if (reopen) await listen(p.id);
        },
      });
    } catch (e) {
      notice(String(e));
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
      <div className="manager-topbar">
        <div className="brand">
          <span className="brand-mark">
            <Icon name="music" size={22} />
          </span>
          Music Studio KH
        </div>
        <button
          type="button"
          className="version-pill"
          id="manager-whats-new"
          onClick={openWhatsNew}
          title="What’s new in this version"
        >
          Music Studio KH {APP_VERSION}
        </button>
      </div>
      <header className="manager-heading">
        <div>
          <h1>Settings</h1>
          <p>Music profiles and this Mac.</p>
        </div>
        <button
          className="primary"
          onClick={() => {
            setFormError("");
            setUseDefaultFolder(true);
            setCreating(true);
          }}
        >
          <Icon name="plus" /> New profile
        </button>
      </header>
      {toolProblem && (
        <p className="error manager-warning" role="alert">
          {toolProblem} Profiles, imports and playback still work.
        </p>
      )}
      {!profiles.length && !creating && (
        <p className="muted">No music profiles yet. Create one to start listening.</p>
      )}
      <h2 className="manager-section-title">Music profiles</h2>
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
            <p className="profile-folder">{shownPath(p.library)}</p>
            {defaultLibrary && p.library === defaultLibrary && (
              <span className="chip">Default location</span>
            )}
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
                    : "Same Wi-Fi · phone app sign-in needs setup in Settings"}
              </dd>
            </dl>
            {p.error && <p className="error">{p.error}</p>}
            {(p.importing || p.imported > 0 || p.importError) && (
              <p
                className={p.importError ? "error" : "profile-import"}
                role="status"
              >
                {p.importing ? "Copying into library… " : "Copied into library: "}
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
                <Icon name="play" /> Listen
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
                <Icon name="settings" />
              </button>
              <button className="subtle" onClick={() => remove(p)}>
                Remove
              </button>
            </div>
          </article>
        ))}
      </div>
      {creating && (
        <form
          className="settings-card profile-form"
          id="create-profile-card"
          aria-labelledby="create-profile-title"
          onSubmit={create}
        >
            <button
              type="button"
              className="close-dialog"
              aria-label="Close"
              onClick={() => setCreating(false)}
            >
              ×
            </button>
            <h2 id="create-profile-title">Create a music profile</h2>
            <label>
              Profile name
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                maxLength={80}
              />
            </label>
            <fieldset className="folder-choice">
              <legend>Music folder</legend>
              <label className="radio-row">
                <input
                  type="radio"
                  name="folder"
                  checked={useDefaultFolder}
                  onChange={() => setUseDefaultFolder(true)}
                />
                <span>
                  Use the default location
                  <code>
                    {defaultLibrary ? shownPath(defaultLibrary) : "Loading..."}
                  </code>
                  <small className="radio-note">
                    Music stays where it is — read and streamed, nothing copied.
                  </small>
                </span>
              </label>
              <label className="radio-row">
                <input
                  type="radio"
                  name="folder"
                  checked={!useDefaultFolder}
                  onChange={() => setUseDefaultFolder(false)}
                />
                <span>
                  Choose a folder
                  <button className="secondary" type="button" onClick={choose}>
                    Choose...
                  </button>
                  <code>{library ? shownPath(library) : "No folder chosen yet"}</code>
                </span>
              </label>
              {profiles.length > 0 && (
                <label>
                  Or share an existing library
                  <select
                    value={useDefaultFolder ? "" : library}
                    onChange={(e) => {
                      setLibrary(e.target.value);
                      setUseDefaultFolder(false);
                    }}
                  >
                    <option value="">Choose a profile’s music folder...</option>
                    {profiles.map((p) => (
                      <option key={p.id} value={p.library}>
                        {p.name}
                      </option>
                    ))}
                  </select>
                </label>
              )}
              <small>
                Music stays where it is. The app reads and streams from this
                folder; nothing is copied. “Copy into library” above is only
                for files you choose to bring in — reading in place needs no
                copy and uses no extra disk space.
              </small>
              <small>
                The folder is remembered for this profile and shown in Settings.
                Existing profiles keep their own folder; nothing is moved.
              </small>
            </fieldset>
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
            {formError && (
              <p className="error" role="alert">
                {formError}
              </p>
            )}
            <button
              className="primary"
              type="submit"
              disabled={busy || (!useDefaultFolder && !library)}
            >
              {busy ? (
                "Creating…"
              ) : (
                <>
                  Create & listen{" "}
                  <Icon name="arrow-up-right" size={14} />
                </>
              )}
            </button>
        </form>
      )}
      {editing && (
        <div className="modal-backdrop">
          <form
            className="settings-card profile-form"
            role="dialog"
            aria-modal="true"
            aria-labelledby="edit-profile-title"
            onSubmit={save}
          >
            <button
              type="button"
              className="close-dialog"
              aria-label="Close"
              onClick={() => setEditing(null)}
            >
              ×
            </button>
            <h2 id="edit-profile-title">Profile settings</h2>
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
                  encrypted, in this app's own data folder. After upgrading, or
                  if saving failed before, enter the password here once.
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
      {confirmAsk && (
        <div
          className="modal-backdrop"
          onClick={(e) => {
            if (e.target === e.currentTarget) setConfirmAsk(null);
          }}
        >
          <section
            className="settings-card whats-new-dialog"
            role="alertdialog"
            aria-modal="true"
            aria-labelledby="profile-confirm-title"
          >
            <button
              type="button"
              className="close-dialog"
              aria-label="Cancel"
              onClick={() => setConfirmAsk(null)}
            >
              ×
            </button>
            <h2 id="profile-confirm-title">{confirmAsk.title}</h2>
            <p className="muted">{confirmAsk.body}</p>
            <button
              className={confirmAsk.destructive ? "danger" : "primary"}
              disabled={busy}
              onClick={async () => {
                const run = confirmAsk.run;
                setConfirmAsk(null);
                await run();
              }}
            >
              {confirmAsk.confirmLabel}
            </button>
            <button className="secondary" onClick={() => setConfirmAsk(null)}>
              Keep
            </button>
          </section>
        </div>
      )}
    </main>
  );
}
