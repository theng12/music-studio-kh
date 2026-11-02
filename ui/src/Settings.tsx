import React, { useEffect, useState } from "react";
import { Appearance, api, sendDesktopCommand, StorageSummary } from "./api";
import { formatBytes as bytes } from "./format";
import { APP_VERSION } from "./version";
import StreamingActivity from "./StreamingActivity";
import Icon from "./Icon";

export default function Settings({
  profile,
  onProfileUpdated,
  tracksCount,
  settings,
  save,
  logout,
  notice,
  openWhatsNew,
  initialSection = "library",
  onRescan,
}: {
  profile: any;
  onProfileUpdated?: (p: any) => void;
  tracksCount?: number;
  settings: Appearance;
  save: (s: Appearance) => void;
  logout: () => void;
  notice: (s: string) => void;
  openWhatsNew: () => void;
  initialSection?: string;
  onRescan?: () => Promise<void> | void;
}) {
  const normalizeSection = (s?: string) => {
    if (!s || s === "all") return "library";
    return s;
  };

  const [section, setSection] = useState(() =>
    normalizeSection(initialSection),
  );
  const [name, setName] = useState(profile?.name || "");
  const [username, setUsername] = useState(profile?.username || "");
  const [password, setPassword] = useState("");
  const [savingProfile, setSavingProfile] = useState(false);
  const [rescanning, setRescanning] = useState(false);
  const [confirmLogoutMode, setConfirmLogoutMode] = useState<
    null | "single" | "all"
  >(null);
  const [loggingOut, setLoggingOut] = useState(false);
  const [storage, setStorage] = useState<StorageSummary | null>(null);
  const [storageError, setStorageError] = useState("");
  useEffect(() => {
    if (section !== "library" || rescanning) return;
    let active = true;
    api<StorageSummary>("/api/storage")
      .then((value) => {
        if (active) {
          setStorage(value);
          setStorageError("");
        }
      })
      .catch((e) => {
        if (active) {
          setStorage(null);
          setStorageError(e.message);
        }
      });
    return () => {
      active = false;
    };
  }, [section, rescanning, profile?.library, tracksCount]);

  useEffect(() => {
    setSection(normalizeSection(initialSection));
  }, [initialSection]);

  useEffect(() => {
    setName(profile?.name || "");
    setUsername(profile?.username || "");
  }, [profile?.name, profile?.username]);

  const selectSection = (id: string) => {
    setSection(id);
    window.scrollTo({ top: 0, behavior: "instant" });
  };

  const show = (id: string) => section === id;

  const saveProfileAccount = async (e: React.FormEvent) => {
    e.preventDefault();
    setSavingProfile(true);
    try {
      const updated = await api("/api/profile", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: name.trim(),
          username: username.trim(),
          password,
        }),
      });
      setPassword("");
      onProfileUpdated?.(updated);
      notice("Profile & account settings saved.");
    } catch (e) {
      notice((e as Error).message);
    } finally {
      setSavingProfile(false);
    }
  };

  const handleRescan = async () => {
    setRescanning(true);
    try {
      if (onRescan) {
        await onRescan();
      } else {
        await api("/api/scan", { method: "POST" });
        notice("Library scan completed.");
      }
    } catch (e) {
      notice((e as Error).message);
    } finally {
      setRescanning(false);
    }
  };

  const executeLogout = async () => {
    if (!confirmLogoutMode) return;
    setLoggingOut(true);
    try {
      if (confirmLogoutMode === "all") {
        await api("/api/logout-all", { method: "POST" });
      }
      setConfirmLogoutMode(null);
      logout();
    } catch (e) {
      notice((e as Error).message);
    } finally {
      setLoggingOut(false);
    }
  };

  const SETTINGS_SECTIONS = [
    ["library", "Library & storage"],
    ["profile", "Profile & account"],
    ["playback", "Playback & audio"],
    ["appearance", "Appearance & display"],
    ["server", "Streaming & access"],
    ["session", "Session & about"],
  ] as const;

  return (
    <section id="settings-page">
      <nav className="settings-tabs" aria-label="Settings categories">
        {SETTINGS_SECTIONS.map(([id, label]) => (
          <button
            key={id}
            type="button"
            aria-pressed={section === id}
            className={section === id ? "selected" : ""}
            onClick={() => selectSection(id)}
          >
            {label}
          </button>
        ))}
      </nav>

      {show("library") && (
        <div className="settings-card" id="library-settings-card">
          <h2>Music library folder</h2>
          <p className="settings-description">
            This folder is on the computer running your Music Studio KH server.
            The library reads your audio in place; editing song details or
            artwork updates those files.
          </p>
          <div className="settings-folder-box" id="settings-folder">
            {profile.library}
          </div>
          <div
            className="storage-summary"
            id="storage-summary"
            aria-live="polite"
          >
            {storage ? (
              <>
                <strong>
                  {storage.indexedFiles} indexed music files ·{" "}
                  {bytes(storage.indexedBytes)}
                </strong>
                <p>
                  Music in this profile’s index, including subfolders. This is
                  not the size of all disk contents.
                </p>
                <p>
                  Server volume: {bytes(storage.diskFreeBytes)} available of{" "}
                  {bytes(storage.diskTotalBytes)} capacity.
                </p>
                {storage.rootAvailable && storage.rootError && (
                  <p role="alert">{storage.rootError}</p>
                )}
                {!storage.rootAvailable && (
                  <p role="alert">
                    Music folder unavailable:{" "}
                    {storage.rootError || "Cannot read this folder"}. Indexed
                    totals are from the last scan.
                  </p>
                )}
              </>
            ) : (
              <p>
                {storageError
                  ? "Storage information unavailable: " + storageError
                  : "Loading storage information…"}
              </p>
            )}
          </div>
          <p className="settings-description">
            Add files to this folder and they appear automatically, usually
            within about 20 seconds. Downloads save here too.
          </p>
          {!(window as any).__TUNEHARBOR_DESKTOP__ && (
            <p className="settings-description">
              To choose a different folder or change phone access, open this
              profile's settings in the desktop app.
            </p>
          )}
          <div className="action-row">
            {(window as any).__TUNEHARBOR_DESKTOP__ && (
              <button
                type="button"
                className="primary"
                id="manage-library-folder"
                onClick={() => sendDesktopCommand("manage-profile")}
              >
                Manage music folder & access
              </button>
            )}
            <button
              type="button"
              className="secondary"
              id="settings-rescan"
              disabled={rescanning}
              onClick={handleRescan}
            >
              <Icon name="refresh" />{" "}
              {rescanning ? "Scanning…" : "Rescan music folder"}
            </button>
            <span className="state-pill completed">
              {typeof tracksCount === "number"
                ? `${tracksCount} ${tracksCount === 1 ? "song" : "songs"} indexed`
                : "Active"}
            </span>
          </div>
        </div>
      )}

      {show("profile") && (
        <div className="settings-card" id="profile-settings-card">
          <div className="profile-card-top">
            <div className="profile-header-meta">
              <span className="profile-avatar">
                {(profile.name || "TH").slice(0, 2).toUpperCase()}
              </span>
              <div>
                <h2>{profile.name}</h2>
              </div>
            </div>
            <span className="state-pill completed">
              {typeof tracksCount === "number"
                ? `${tracksCount} ${tracksCount === 1 ? "song" : "songs"}`
                : "Active"}
            </span>
          </div>
          <form onSubmit={saveProfileAccount}>
            <div className="form-columns">
              <label>
                Profile display name
                <input
                  id="pref-profile-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  required
                  maxLength={80}
                />
              </label>
              <label>
                Sign-in username
                <input
                  id="pref-username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  required
                  maxLength={80}
                />
              </label>
            </div>
            <label>
              Change password (optional)
              <input
                id="pref-password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                minLength={6}
                autoComplete="new-password"
                placeholder="Leave blank to keep your current password"
              />
            </label>
            <div className="action-row">
              <button
                className="primary"
                id="save-profile-btn"
                type="submit"
                disabled={savingProfile}
              >
                {savingProfile ? "Saving…" : "Save profile"}
              </button>
            </div>
          </form>
        </div>
      )}

      {show("playback") && (
        <div className="settings-card" id="playback-settings-card">
          <h2>Listening preferences</h2>
          <label htmlFor="streamQuality">
            Streaming audio quality
            <select
              id="streamQuality"
              value={settings.streamQuality || "original"}
              onChange={(e) =>
                save({ ...settings, streamQuality: e.target.value })
              }
            >
              <option value="original">Original file (no conversion)</option>
              <option value="mp3">
                Compatible MP3 transcode (Best for older browsers)
              </option>
            </select>
          </label>
          <label htmlFor="repeatMode">
            Queue & repeat behavior
            <select
              id="repeatMode"
              value={settings.repeatMode || "all"}
              onChange={(e) =>
                save({ ...settings, repeatMode: e.target.value })
              }
            >
              <option value="all">Play next song & loop queue</option>
              <option value="one">Repeat current song</option>
              <option value="off">Stop when current song finishes</option>
            </select>
          </label>
          <label htmlFor="closeBehavior">
            When closing the window (red × button)
            <select
              id="closeBehavior"
              value={settings.closeBehavior || "ask"}
              onChange={(e) =>
                save({
                  ...settings,
                  closeBehavior: e.target.value as Appearance["closeBehavior"],
                })
              }
            >
              <option value="ask">
                Ask me whether to play in background or quit
              </option>
              <option value="background">
                Keep playing in background (Minimize to menu bar)
              </option>
              <option value="quit">Close window and stop music playback</option>
            </select>
          </label>
          <p className="muted">
            Original quality streams FLAC, WAV, M4A, AAC, Ogg, Opus, and MP3
            directly without re-encoding whenever your device supports the
            format.
          </p>
        </div>
      )}

      {show("appearance") && (
        <div className="settings-card" id="appearance-settings-card">
          <h2>Appearance & display</h2>
          {(
            [
              [
                "theme",
                "Theme",
                [
                  ["dark", "Midnight (Dark)"],
                  ["light", "Daylight (Light)"],
                  ["system", "Follow system"],
                ],
              ],
              [
                "accent",
                "Accent color",
                [
                  ["mint", "Harbor mint"],
                  ["violet", "Evening violet"],
                  ["amber", "Golden hour"],
                ],
              ],
              [
                "density",
                "Library density",
                [
                  ["comfortable", "Comfortable"],
                  ["compact", "Compact"],
                ],
              ],
              [
                "layout",
                "Default library layout",
                [
                  ["grid", "Cover grid"],
                  ["list", "Compact list"],
                ],
              ],
              [
                "defaultTab",
                "Default library view",
                [
                  ["songs", "Songs"],
                  ["albums", "Albums"],
                  ["artists", "Artists"],
                  ["folders", "Folders"],
                ],
              ],
            ] as const
          ).map(([key, label, choices]) => (
            <label key={key} htmlFor={key}>
              {label}
              <select
                id={key}
                value={(settings as any)[key] || choices[0][0]}
                onChange={(e) => save({ ...settings, [key]: e.target.value })}
              >
                {choices.map(([value, name]) => (
                  <option key={value} value={value}>
                    {name}
                  </option>
                ))}
              </select>
            </label>
          ))}
          <label htmlFor="recentDays">
            Show New badges for this many days (0 turns them off)
            <input
              id="recentDays"
              type="number"
              min="0"
              step="1"
              key={settings.recentDays ?? 7}
              defaultValue={settings.recentDays ?? 7}
              onBlur={(e) => {
                const days = Number(e.currentTarget.value);
                if (
                  e.currentTarget.value !== "" &&
                  Number.isSafeInteger(days) &&
                  days >= 0
                ) {
                  if (days !== (settings.recentDays ?? 7))
                    save({ ...settings, recentDays: days });
                } else {
                  e.currentTarget.value = String(settings.recentDays ?? 7);
                  notice(
                    "Use a whole number of days, or 0 to turn New badges off.",
                  );
                }
              }}
            />
          </label>
          <p className="muted">
            Uses a known date added or downloaded, separate from a song’s
            release year. Older tracks without that date stay unmarked.
          </p>
          <label htmlFor="reduceTransparency">
            Control surface appearance
            <select
              id="reduceTransparency"
              value={settings.reduceTransparency ? "opaque" : "system"}
              onChange={(e) =>
                save({
                  ...settings,
                  reduceTransparency: e.target.value === "opaque",
                })
              }
            >
              <option value="system">
                Glass style, respecting system accessibility
              </option>
              <option value="opaque">Opaque controls</option>
            </select>
          </label>
          <p className="muted">
            Saved to this profile. Your desktop and browser player share these
            defaults.
          </p>
        </div>
      )}

      {show("server") && (
        <div className="settings-card" id="server-settings-card">
          <h2>Your server & streaming access</h2>
          <dl>
            <dt>Profile</dt>
            <dd id="settings-profile">{profile.name}</dd>
            <dt>Username</dt>
            <dd id="settings-username">{profile.username}</dd>
            <dt>Server address</dt>
            <dd id="settings-address">{location.origin}</dd>
            <dt>Access mode</dt>
            <dd id="settings-access">
              {profile.lan ? "Local network enabled" : "This computer only"}
            </dd>
            {profile.lan && profile.addresses?.length > 0 && (
              <>
                <dt>Phone addresses</dt>
                <dd className="address-list">
                  {profile.addresses.map((address: string) => (
                    <div key={address}>{address}</div>
                  ))}
                </dd>
              </>
            )}
          </dl>
          <p className="muted">
            Use a phone address, username, and password in Subsonic, Amperfy, or
            Symfonium. Keep this computer awake and Music Studio KH running.
            Local-network mode uses HTTP on your trusted Wi-Fi.
          </p>
          <StreamingActivity />
        </div>
      )}

      {show("session") && (
        <div className="settings-card" id="session-settings-card">
          <h2>Session security & release</h2>
          <p className="settings-description">
            Signing out closes your current browser or desktop session without
            stopping the background music server or touching your library files.
          </p>
          <div className="action-row">
            <button
              type="button"
              className="secondary"
              id="settings-logout"
              onClick={() => setConfirmLogoutMode("single")}
            >
              Sign out of this profile…
            </button>
            <button
              type="button"
              className="secondary"
              id="settings-logout-all"
              onClick={() => setConfirmLogoutMode("all")}
            >
              Sign out all browser devices…
            </button>
          </div>
          <div className="version-footer">
            <div className="version-meta">
              <span className="version-badge" id="settings-version">
                v{APP_VERSION}
              </span>
              <span>Music Studio KH {APP_VERSION}</span>
            </div>
            <button
              type="button"
              className="secondary compact-button"
              id="settings-whats-new"
              onClick={openWhatsNew}
            >
              What’s new
            </button>
          </div>
          <p className="muted">
            Desktop music library and local Subsonic streaming server. App and
            download tools update together through a new installer. Profiles and
            music are preserved.
          </p>
        </div>
      )}

      {confirmLogoutMode && (
        <div
          className="modal-backdrop"
          onClick={(e) => {
            if (e.target === e.currentTarget && !loggingOut)
              setConfirmLogoutMode(null);
          }}
        >
          <section
            className="settings-card whats-new-dialog"
            id="logout-confirm-dialog"
            role="alertdialog"
            aria-modal="true"
            aria-labelledby="logout-confirm-title"
          >
            <button
              type="button"
              className="close-dialog"
              aria-label="Cancel sign out"
              onClick={() => setConfirmLogoutMode(null)}
            >
              ×
            </button>
            <h2 id="logout-confirm-title">
              {confirmLogoutMode === "all"
                ? "Sign out all browser devices?"
                : `Sign out of ${profile.name}?`}
            </h2>
            <p>
              {confirmLogoutMode === "all"
                ? "Every browser and desktop window signed in to this profile will require your username and password again. Your music server and library files remain untouched."
                : "You will return to the sign-in screen for this profile. Your music server and library files remain safe on your computer."}
            </p>
            <div className="action-row">
              <button
                type="button"
                className="secondary"
                id="cancel-logout"
                disabled={loggingOut}
                onClick={() => setConfirmLogoutMode(null)}
              >
                Cancel
              </button>
              <button
                type="button"
                className="primary danger-button"
                id="confirm-logout"
                disabled={loggingOut}
                onClick={executeLogout}
              >
                {loggingOut ? "Signing out…" : "Sign out"}
              </button>
            </div>
          </section>
        </div>
      )}
    </section>
  );
}
