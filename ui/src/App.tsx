import React, { useEffect, useRef, useState } from "react";
import {
  api,
  Appearance,
  ImportItem,
  isManager,
  Job,
  MetadataInput,
  sendDesktopCommand,
  Track,
} from "./api";
import Icon from "./Icon";
import Downloads from "./Downloads";
import Library from "./Library";
import Player from "./Player";
import Artwork from "./Artwork";
import Settings from "./Settings";
import Profiles from "./Profiles";
import WhatsNew from "./WhatsNew";
import ImportDialog from "./ImportDialog";
import { isImportableFile } from "./drop";
import { APP_VERSION } from "./version";
import { shuffledQueue } from "./playback";
const defaults: Appearance = {
  theme: "dark",
  accent: "mint",
  density: "comfortable",
  layout: "grid",
};
let bootstrapPromise: Promise<any> | null = null;
async function initialProfile() {
  const token = (window as any).__TUNEHARBOR_BOOTSTRAP__;
  if (token) {
    delete (window as any).__TUNEHARBOR_BOOTSTRAP__;
    bootstrapPromise = api("/api/desktop-bootstrap", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ token }),
    }).catch(() => api("/api/me"));
  }
  return bootstrapPromise || api("/api/me");
}
export default function App() {
  const [profile, setProfile] = useState<any>(null),
    [page, setPage] = useState("library"),
    [tracks, setTracks] = useState<Track[]>([]),
    [tracksLoaded, setTracksLoaded] = useState(false),
    [tracksError, setTracksError] = useState(""),
    [jobs, setJobs] = useState<Job[]>([]),
    [jobsError, setJobsError] = useState(""),
    [jobsLoaded, setJobsLoaded] = useState(false),
    [query, setQuery] = useState(""),
    [playing, setPlaying] = useState<Track | null>(null),
    [playQueue, setPlayQueue] = useState<Track[]>([]),
    [shuffle, setShuffle] = useState(false),
    [appearance, setAppearance] = useState(defaults),
    [version, setVersion] = useState(0),
    [artwork, setArtwork] = useState<{ track: Track; album: boolean } | null>(
      null,
    ),
    [message, setMessage] = useState(""),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [whatsNew, setWhatsNew] = useState(false),
    [customImport, setCustomImport] = useState(false),
    [settingsSection, setSettingsSection] = useState("all"),
    [libraryTab, setLibraryTab] = useState<
      "songs" | "albums" | "artists" | "folders"
    >("songs"),
    [closePromptOpen, setClosePromptOpen] = useState(false),
    [rememberCloseChoice, setRememberCloseChoice] = useState(true);
  const files = useRef<HTMLInputElement>(null),
    folders = useRef<HTMLInputElement>(null),
    messageTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const notify = (text: string) => {
    setMessage(text);
    if (messageTimer.current) clearTimeout(messageTimer.current);
    messageTimer.current = setTimeout(() => setMessage(""), 6000);
  };
  const loadTracks = async () => {
    try {
      setTracks(await api("/api/tracks"));
      setTracksLoaded(true);
      setTracksError("");
    } catch (e) {
      setTracksError((e as Error).message);
      throw e;
    }
  };
  const completed = useRef(new Set<string>());
  const loadJobs = async () => {
    let rows: Job[];
    try {
      rows = await api("/api/downloads");
      setJobs(rows);
      setJobsLoaded(true);
      setJobsError("");
    } catch (e) {
      setJobsError((e as Error).message);
      throw e;
    }
    if (
      rows.some((j) => j.state === "completed" && !completed.current.has(j.id))
    ) {
      rows
        .filter((j) => j.state === "completed")
        .forEach((j) => completed.current.add(j.id));
      await loadTracks();
    }
  };
  useEffect(() => {
    if (isManager) return;
    initialProfile()
      .then(setProfile)
      .catch(() => {});
    const expired = () => {
      setProfile(null);
      setPlaying(null);
    };
    window.addEventListener("session-expired", expired);
    return () => window.removeEventListener("session-expired", expired);
  }, []);
  useEffect(() => {
    if (!profile) return;
    api<Appearance>("/api/settings")
      .then((settings) => {
        setAppearance(settings);
        if (
          settings.defaultTab &&
          ["songs", "albums", "artists", "folders"].includes(
            settings.defaultTab,
          )
        ) {
          setLibraryTab(settings.defaultTab as any);
        }
      })
      .catch((e) => notify(e.message));
    loadTracks().catch((e) => notify(e.message));
    loadJobs().catch(() => {});
  }, [profile]);
  useEffect(() => {
    if (!profile) return;
    const timer = setInterval(
      () => {
        if (document.hidden) return;
        loadJobs().catch(() => {});
        if (page === "library") loadTracks().catch(() => {});
      },
      page === "downloads" ? 2000 : 20000,
    );
    return () => clearInterval(timer);
  }, [profile, page, query]);
  useEffect(() => {
    const apply = () => {
      let theme = appearance.theme;
      if (theme === "system")
        theme = matchMedia("(prefers-color-scheme:dark)").matches
          ? "dark"
          : "light";
      document.documentElement.dataset.theme = theme;
      document.documentElement.dataset.accent = appearance.accent;
      document.documentElement.dataset.density = appearance.density;
      document.documentElement.dataset.reduceTransparency = String(
        appearance.reduceTransparency || false,
      );
    };
    apply();
    const media = matchMedia("(prefers-color-scheme:dark)");
    media.addEventListener("change", apply);
    return () => media.removeEventListener("change", apply);
  }, [appearance]);
  const saveAppearance = async (value: Appearance) => {
    setAppearance(value);
    try {
      await api("/api/settings", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(value),
      });
    } catch (e) {
      notify((e as Error).message);
    }
  };

  useEffect(() => {
    const handler = () => {
      const mode = appearance.closeBehavior || "ask";
      if (mode === "background") {
        sendDesktopCommand("hide-window");
        notify("Playing in the background. Use the menu bar icon to reopen.");
      } else if (mode === "quit") {
        sendDesktopCommand("close-profile-window");
      } else {
        setClosePromptOpen(true);
      }
    };
    (window as any).__TUNEHARBOR_ON_CLOSE_REQUESTED__ = handler;
    return () => {
      if ((window as any).__TUNEHARBOR_ON_CLOSE_REQUESTED__ === handler) {
        delete (window as any).__TUNEHARBOR_ON_CLOSE_REQUESTED__;
      }
    };
  }, [appearance]);
  const logout = async () => {
    try {
      await api("/api/logout", { method: "POST" });
    } catch {}
    setProfile(null);
    setPlaying(null);
    bootstrapPromise = null;
  };
  const login = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const data = new FormData(e.currentTarget);
    setBusy(true);
    setError("");
    try {
      setProfile(
        await api("/api/login", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            username: data.get("username"),
            password: data.get("password"),
          }),
        }),
      );
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  };
  const importFiles = async (
    selected: (File | ImportItem)[],
    meta?: MetadataInput,
    customArt?: File | null,
  ) => {
    const normalized: ImportItem[] = selected
      .map((entry) =>
        "file" in entry
          ? entry
          : {
              file: entry,
              relPath: (entry as any).webkitRelativePath || entry.name,
            },
      )
      .filter((it) =>
        isImportableFile(
          it.file.name,
          Boolean(it.relPath && it.relPath.includes("/")),
        ),
      );
    const images = normalized.filter((it) =>
      /\.(jpg|jpeg|png|webp)$/i.test(it.file.name),
    );
    const audios = normalized.filter((it) =>
      /\.(mp3|m4a|aac|flac|ogg|opus|wav)$/i.test(it.file.name),
    );
    for (const img of images) {
      const form = new FormData();
      if (img.relPath) form.append("path", img.relPath);
      form.append("file", img.file);
      try {
        await api("/api/import", { method: "POST", body: form });
      } catch {}
    }
    let added = 0;
    let skipped = 0;
    let firstTrackId = "";
    for (const item of audios) {
      const form = new FormData();
      if (item.relPath) form.append("path", item.relPath);
      if (meta?.title && audios.length === 1) form.append("title", meta.title);
      if (meta?.artist) form.append("artist", meta.artist);
      if (meta?.album) form.append("album", meta.album);
      if (meta?.version) form.append("version", meta.version);
      if (meta?.year) form.append("year", String(meta.year));
      if (meta?.trackNumber)
        form.append("trackNumber", String(meta.trackNumber));
      if (meta?.lyrics) form.append("lyrics", meta.lyrics);
      form.append("file", item.file);
      notify("Importing " + (item.relPath || item.file.name) + "…");
      try {
        const created: Track = await api("/api/import", {
          method: "POST",
          body: form,
        });
        if (created?.skipped) {
          skipped++;
        } else {
          if (!firstTrackId && created?.id) firstTrackId = created.id;
          added++;
        }
      } catch (e) {
        notify((e as Error).message);
      }
    }
    if (customArt && firstTrackId) {
      try {
        const endpoint =
          audios.length > 1 || meta?.album
            ? "/api/album-artwork/" + firstTrackId
            : "/api/artwork/" + firstTrackId;
        await api(endpoint, { method: "PUT", body: customArt });
        setVersion((n) => n + 1);
      } catch (e) {
        notify((e as Error).message);
      }
    } else if (images.length > 0) {
      setVersion((n) => n + 1);
    }
    await loadTracks();
    if (added > 0 && skipped > 0) {
      notify(
        `${added} song${added === 1 ? "" : "s"} added, ${skipped} duplicate${skipped === 1 ? "" : "s"} skipped.`,
      );
    } else if (added === 0 && skipped > 0) {
      notify(
        `Skipped ${skipped} already existing song${skipped === 1 ? "" : "s"}.`,
      );
    } else if (added > 0) {
      notify(`${added} song${added === 1 ? "" : "s"} added to your library.`);
    }
  };
  const updateTrack = (id: string, changes: Partial<Track>) => {
    const merge = (t: Track) => (t.id === id ? { ...t, ...changes } : t);
    setTracks((prev) => prev.map(merge));
    setPlayQueue((prev) => prev.map(merge));
    setPlaying((prev) => (prev ? merge(prev) : prev));
  };
  const favourite = async (track: Track, value: boolean) => {
    try {
      const result = await api<{ favourite: boolean }>(
        "/api/tracks/" + track.id + "/favourite",
        {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ favourite: value }),
        },
      );
      updateTrack(track.id, result);
    } catch (e) {
      notify((e as Error).message);
    }
  };
  const played = async (track: Track) => {
    try {
      const result = await api<Pick<Track, "playCount" | "lastPlayedAt">>(
        "/api/tracks/" + track.id + "/plays",
        { method: "POST" },
      );
      updateTrack(track.id, result);
    } catch (e) {
      notify("Could not save listening history: " + (e as Error).message);
    }
  };
  const play = (track: Track, queue: Track[] = tracks) => {
    setShuffle(false);
    setPlayQueue([...new Map(queue.map((t) => [t.id, t])).values()]);
    setPlaying(track);
    const audio = document.querySelector<HTMLAudioElement>("#audio");
    if (playing?.id === track.id && audio?.paused)
      audio.play().catch((e) => notify(e.message));
  };
  const playShuffled = (scope: Track[]) => {
    const queue = shuffledQueue(scope);
    if (queue.length) play(queue[0], queue);
  };
  const skip = (offset: number) => {
    if (!playQueue.length) return;
    const index = playQueue.findIndex((t) => t.id === playing?.id);
    setPlaying(
      playQueue[
        (Math.max(0, index) + offset + playQueue.length) % playQueue.length
      ],
    );
  };
  const toast = message ? (
    <div id="toast" role="status" aria-live="polite">
      {message}
    </div>
  ) : (
    <div id="toast" hidden />
  );
  if (isManager)
    return (
      <>
        {toast}
        <Profiles notice={notify} openWhatsNew={() => setWhatsNew(true)} />
        <WhatsNew open={whatsNew} onClose={() => setWhatsNew(false)} />
      </>
    );
  if (!profile)
    return (
      <>
        {toast}
        <section className="login-screen" id="login-screen">
          <div className="welcome">
            <div className="brand">
              <span className="brand-mark">
                <Icon name="music" size={22} />
              </span>
              Music Studio KH
            </div>
            <p className="eyebrow">YOUR MUSIC, AT HOME</p>
            <h1>
              A little harbor.
              <br />A whole world
              <br />
              of music.
            </h1>
            <p className="intro">
              Your collection, on your computer.
              <br />
              Ready whenever you want to listen.
            </p>
            <div className="record">
              <div className="record-center">KH</div>
            </div>
          </div>
          <form className="login-card" id="login-form" onSubmit={login}>
            <h2>Open your library</h2>
            <p>Sign in to the profile running at this address.</p>
            <label>
              Username
              <input
                id="username"
                name="username"
                autoComplete="username"
                required
                placeholder="Your username"
              />
            </label>
            <label>
              Password
              <input
                id="password"
                name="password"
                type="password"
                autoComplete="current-password"
                required
                placeholder="Your password"
              />
            </label>
            <button className="primary" type="submit" disabled={busy}>
              Open library <span>↗</span>
            </button>
            <p id="login-error" className="error" role="alert">
              {error}
            </p>
            <small>Your files stay on your computer.</small>
          </form>
        </section>
      </>
    );
  const songsCount = tracks.length;
  const albumsCount = new Set(
    tracks.map(
      (t) => (t.album || "Singles & Unsorted") + "::" + (t.version || ""),
    ),
  ).size;
  const artistsCount = new Set(
    tracks.flatMap((t) => t.artists || [t.artist || "Unknown artist"]),
  ).size;
  const foldersCount = new Set(tracks.map((t) => t.folder || "Root library"))
    .size;

  return (
    <>
      {toast}
      <div id="app" className="app">
        <aside className="sidebar">
          <a
            className="brand"
            href="/"
            onClick={(e) => {
              e.preventDefault();
              setPage("library");
              setLibraryTab("songs");
            }}
          >
            <span className="brand-mark">
              <Icon name="music" size={22} />
            </span>
            Music Studio KH
          </a>
          <span className="sidebar-label">LIBRARY</span>
          <button
            className={
              "nav-item " +
              (page === "library" && libraryTab === "songs" ? "active" : "")
            }
            id="library-nav"
            aria-label="Songs"
            onClick={() => {
              setPage("library");
              setLibraryTab("songs");
            }}
          >
            <Icon name="music" /> <span>Songs</span>
            <small className="nav-count">
              {tracksLoaded ? songsCount : "—"}
            </small>
          </button>
          <button
            className={
              "nav-item " +
              (page === "library" && libraryTab === "albums" ? "active" : "")
            }
            id="nav-albums"
            aria-label="Albums"
            onClick={() => {
              setPage("library");
              setLibraryTab("albums");
            }}
          >
            <Icon name="album" /> <span>Albums</span>
            <small className="nav-count">
              {tracksLoaded ? albumsCount : "—"}
            </small>
          </button>
          <button
            className={
              "nav-item " +
              (page === "library" && libraryTab === "artists" ? "active" : "")
            }
            id="nav-artists"
            aria-label="Artists"
            onClick={() => {
              setPage("library");
              setLibraryTab("artists");
            }}
          >
            <Icon name="artist" /> <span>Artists</span>
            <small className="nav-count">
              {tracksLoaded ? artistsCount : "—"}
            </small>
          </button>
          <button
            className={
              "nav-item " +
              (page === "library" && libraryTab === "folders" ? "active" : "")
            }
            id="nav-folders"
            aria-label="Folders"
            onClick={() => {
              setPage("library");
              setLibraryTab("folders");
            }}
          >
            <Icon name="folder" /> <span>Folders</span>
            <small className="nav-count">
              {tracksLoaded ? foldersCount : "—"}
            </small>
          </button>

          <span className="sidebar-label" style={{ marginTop: "18px" }}>
            MANAGE
          </span>
          <button
            className={"nav-item " + (page === "downloads" ? "active" : "")}
            id="downloads-nav"
            aria-label="Downloads"
            onClick={() => setPage("downloads")}
          >
            <Icon name="download" /> <span>Downloads</span>
            {jobs.some((j) => j.state === "running") && (
              <i className="download-dot" />
            )}
          </button>
          <button
            className={"nav-item " + (page === "settings" ? "active" : "")}
            id="settings-nav"
            aria-label="Settings"
            onClick={() => {
              setSettingsSection("library");
              setPage("settings");
            }}
          >
            <Icon name="settings" /> <span>Settings</span>
          </button>
          <div className="sidebar-bottom">
            <button
              type="button"
              className="sidebar-profile-btn"
              id="sidebar-profile-btn"
              onClick={() => {
                setSettingsSection("profile");
                setPage("settings");
              }}
              aria-label="Open profile and account settings"
            >
              <div className="profile-avatar">
                {profile.name.slice(0, 2).toUpperCase()}
              </div>
              <div className="sidebar-profile-text">
                <strong id="profile-name">{profile.name}</strong>
                <span className="status">
                  <i />
                  Profile & settings ›
                </span>
              </div>
            </button>
            <div className="sidebar-meta">
              <span className="version-badge">v{APP_VERSION}</span>
              <button
                type="button"
                className="subtle"
                id="sidebar-whats-new"
                onClick={() => setWhatsNew(true)}
              >
                What’s new
              </button>
            </div>
          </div>
        </aside>
        <main>
          <header className="topbar">
            <div>
              <h1 id="page-title">
                {page === "downloads"
                  ? "Downloads"
                  : page === "settings"
                    ? "Settings & preferences"
                    : "Music library"}
                <span className="accent">.</span>
              </h1>
            </div>
          </header>
          {page === "library" && tracksError && (
            <p role="alert">
              Could not refresh your library: {tracksError}
              {tracksLoaded ? ". Showing the last successful reading." : ""}
            </p>
          )}
          {page === "library" && !tracksLoaded && !tracksError && (
            <p role="status">Loading your music library…</p>
          )}
          {page === "library" && tracksLoaded && (
            <Library
              notice={notify}
              tracks={tracks}
              query={query}
              onQuery={setQuery}
              onPlay={play}
              onPlayShuffle={playShuffled}
              onFavourite={favourite}
              newDays={appearance.recentDays ?? 7}
              onArtwork={(track, album) => setArtwork({ track, album })}
              chooseFiles={() => files.current?.click()}
              chooseFolder={() => folders.current?.click()}
              openCustomImport={() => setCustomImport(true)}
              importFiles={importFiles}
              version={version}
              layout={appearance.layout}
              defaultTab={appearance.defaultTab}
              activeTab={libraryTab}
              onTabChange={setLibraryTab}
              toggleLayout={() =>
                saveAppearance({
                  ...appearance,
                  layout: appearance.layout === "grid" ? "list" : "grid",
                })
              }
            />
          )}
          {page === "downloads" && (
            <Downloads
              jobs={jobs}
              refresh={loadJobs}
              notice={notify}
              loading={!jobsLoaded && !jobsError}
              error={jobsError}
            />
          )}
          {page === "settings" && (
            <Settings
              notice={notify}
              profile={profile}
              onProfileUpdated={setProfile}
              settings={appearance}
              save={saveAppearance}
              logout={logout}
              openWhatsNew={() => setWhatsNew(true)}
              tracksCount={tracksLoaded ? tracks.length : undefined}
              initialSection={settingsSection}
              onRescan={async () => {
                await api("/api/scan", { method: "POST" });
                await loadTracks();
                notify("Library refreshed.");
              }}
            />
          )}
          <input
            id="music-files"
            ref={files}
            type="file"
            multiple
            accept=".mp3,.m4a,.aac,.flac,.ogg,.opus,.wav"
            hidden
            onChange={(e) => {
              importFiles([...(e.target.files || [])]);
              e.target.value = "";
            }}
          />
          <input
            id="music-folder"
            ref={folders}
            type="file"
            multiple
            {...({ webkitdirectory: "", directory: "" } as any)}
            hidden
            onChange={(e) => {
              importFiles([...(e.target.files || [])]);
              e.target.value = "";
            }}
          />
        </main>
        <Player
          queue={playQueue}
          shuffle={shuffle}
          onShuffleChange={setShuffle}
          onFavourite={favourite}
          onPlayed={played}
          select={setPlaying}
          track={playing}
          version={version}
          next={() => skip(1)}
          previous={() => skip(-1)}
          notice={notify}
          streamQuality={appearance.streamQuality}
          repeatMode={appearance.repeatMode}
          onRepeatModeChange={(mode) =>
            saveAppearance({ ...appearance, repeatMode: mode })
          }
          onTrackUpdated={(updated) => updateTrack(updated.id, updated)}
          allTracks={tracks}
        />
      </div>
      <Artwork
        selected={artwork}
        close={() => setArtwork(null)}
        changed={() => {
          setVersion((n) => n + 1);
          loadTracks().catch(() => {});
        }}
        notice={notify}
      />
      <ImportDialog
        open={customImport}
        onClose={() => setCustomImport(false)}
        onImport={importFiles}
      />
      <WhatsNew open={whatsNew} onClose={() => setWhatsNew(false)} />
      {closePromptOpen && (
        <div
          className="modal-backdrop"
          onClick={(e) => {
            if (e.target === e.currentTarget) setClosePromptOpen(false);
          }}
        >
          <section
            className="settings-card whats-new-dialog"
            id="close-behavior-dialog"
            role="dialog"
            aria-modal="true"
            aria-labelledby="close-behavior-title"
          >
            <button
              type="button"
              className="close-dialog"
              aria-label="Cancel close dialog"
              onClick={() => setClosePromptOpen(false)}
            >
              ×
            </button>
            <h2 id="close-behavior-title">
              Keep music playing in the background?
            </h2>
            <p className="muted">
              Choose whether clicking the red close button minimizes Music
              Studio KH to the menu bar so your music keeps playing, or closes
              the window and stops playback. You can change this anytime in
              Settings.
            </p>
            <label
              className="checkbox"
              style={{
                display: "flex",
                alignItems: "center",
                gap: "8px",
                margin: "18px 0 14px",
                cursor: "pointer",
                fontSize: "12px",
                color: "var(--text)",
              }}
            >
              <input
                type="checkbox"
                id="remember-close-choice"
                checked={rememberCloseChoice}
                onChange={(e) => setRememberCloseChoice(e.target.checked)}
                style={{ width: "auto" }}
              />
              <span>Always use this choice (change anytime in Settings)</span>
            </label>
            <div className="action-row">
              <button
                type="button"
                className="primary"
                id="close-keep-playing"
                onClick={async () => {
                  if (rememberCloseChoice)
                    await saveAppearance({
                      ...appearance,
                      closeBehavior: "background",
                    });
                  setClosePromptOpen(false);
                  sendDesktopCommand("hide-window");
                  notify(
                    "Playing in the background. Use the menu bar icon to reopen.",
                  );
                }}
              >
                Keep playing in background
              </button>
              <button
                type="button"
                className="secondary"
                id="close-quit-now"
                onClick={async () => {
                  if (rememberCloseChoice)
                    await saveAppearance({
                      ...appearance,
                      closeBehavior: "quit",
                    });
                  setClosePromptOpen(false);
                  sendDesktopCommand("close-profile-window");
                }}
              >
                Close & stop music
              </button>
              <button
                type="button"
                className="subtle"
                id="close-cancel"
                onClick={() => setClosePromptOpen(false)}
              >
                Cancel
              </button>
            </div>
          </section>
        </div>
      )}
    </>
  );
}
