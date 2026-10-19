import React, { useEffect, useRef, useState } from "react";
import { api, Appearance, isManager, Job, Track } from "./api";
import Downloads from "./Downloads";
import Library from "./Library";
import Player from "./Player";
import Artwork from "./Artwork";
import Settings from "./Settings";
import Profiles from "./Profiles";
import WhatsNew from "./WhatsNew";
import { APP_VERSION } from "./version";
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
    [jobs, setJobs] = useState<Job[]>([]),
    [query, setQuery] = useState(""),
    [playing, setPlaying] = useState<Track | null>(null),
    [playQueue, setPlayQueue] = useState<Track[]>([]),
    [appearance, setAppearance] = useState(defaults),
    [version, setVersion] = useState(0),
    [artwork, setArtwork] = useState<{ track: Track; album: boolean } | null>(
      null,
    ),
    [message, setMessage] = useState(""),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [whatsNew, setWhatsNew] = useState(false);
  const files = useRef<HTMLInputElement>(null),
    messageTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const notify = (text: string) => {
    setMessage(text);
    if (messageTimer.current) clearTimeout(messageTimer.current);
    messageTimer.current = setTimeout(() => setMessage(""), 6000);
  };
  const loadTracks = async () =>
    setTracks(await api("/api/tracks?q=" + encodeURIComponent(query)));
  const completed = useRef(new Set<string>());
  const loadJobs = async () => {
    const rows: Job[] = await api("/api/downloads");
    setJobs(rows);
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
      .then(setAppearance)
      .catch((e) => notify(e.message));
    loadTracks().catch((e) => notify(e.message));
    loadJobs().catch(() => {});
  }, [profile]);
  useEffect(() => {
    if (!profile) return;
    const timer = setTimeout(
      () => loadTracks().catch((e) => notify(e.message)),
      180,
    );
    return () => clearTimeout(timer);
  }, [query, profile]);
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
  const importFiles = async (selected: File[]) => {
    let count = 0;
    for (const file of selected) {
      const form = new FormData();
      form.append("file", file);
      notify("Importing " + file.name + "…");
      try {
        await api("/api/import", { method: "POST", body: form });
        count++;
      } catch (e) {
        notify((e as Error).message);
      }
    }
    await loadTracks();
    if (count)
      notify(
        count + " song" + (count === 1 ? "" : "s") + " added to your library.",
      );
  };
  const play = (track: Track, queue: Track[] = tracks) => {
    setPlayQueue([...queue]);
    setPlaying(track);
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
              <span className="brand-mark">♪</span>TuneHarbor
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
              <div className="record-center">TH</div>
            </div>
          </div>
          <form className="login-card" id="login-form" onSubmit={login}>
            <span className="eyebrow">WELCOME ABOARD</span>
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
  return (
    <>
      {toast}
      <div id="app" className="app">
        <aside className="sidebar">
          <a className="brand" href="/">
            <span className="brand-mark">♪</span>TuneHarbor
          </a>
          <span className="sidebar-label">YOUR SPACE</span>
          <button
            className={"nav-item " + (page === "library" ? "active" : "")}
            id="library-nav"
            onClick={() => setPage("library")}
          >
            ♫ <span>Music library</span>
          </button>
          <button
            className={"nav-item " + (page === "downloads" ? "active" : "")}
            id="downloads-nav"
            aria-label="Downloads"
            onClick={() => setPage("downloads")}
          >
            ↓ <span>Downloads</span>
            {jobs.some((j) => j.state === "running") && (
              <i className="download-dot" />
            )}
          </button>
          <button
            className={"nav-item " + (page === "settings" ? "active" : "")}
            id="settings-nav"
            onClick={() => setPage("settings")}
          >
            ⚙︎ <span>Appearance & access</span>
          </button>
          <div className="sidebar-bottom">
            <div className="profile-avatar">
              {profile.name.slice(0, 2).toUpperCase()}
            </div>
            <strong id="profile-name">{profile.name}</strong>
            <span className="status">
              <i />
              Server running
            </span>
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
            <button className="subtle" id="logout" onClick={logout}>
              Sign out ↗
            </button>
          </div>
        </aside>
        <main>
          <header className="topbar">
            <div>
              <span className="eyebrow">YOUR PERSONAL MUSIC SERVER</span>
              <h1 id="page-title">
                {page === "downloads"
                  ? "Downloads"
                  : page === "settings"
                    ? "Your space"
                    : "Music library"}
                <span className="accent">.</span>
              </h1>
            </div>
            <button
              className="secondary"
              id="rescan"
              onClick={async () => {
                try {
                  await api("/api/scan", { method: "POST" });
                  await loadTracks();
                  notify("Library refreshed.");
                } catch (e) {
                  notify((e as Error).message);
                }
              }}
            >
              ↻ Rescan library
            </button>
          </header>
          {page === "library" && (
            <Library
              notice={notify}
              tracks={tracks}
              query={query}
              onQuery={setQuery}
              onPlay={play}
              onArtwork={(track, album) => setArtwork({ track, album })}
              chooseFiles={() => files.current?.click()}
              importFiles={importFiles}
              version={version}
              layout={appearance.layout}
              toggleLayout={() =>
                saveAppearance({
                  ...appearance,
                  layout: appearance.layout === "grid" ? "list" : "grid",
                })
              }
            />
          )}
          {page === "downloads" && (
            <Downloads jobs={jobs} refresh={loadJobs} notice={notify} />
          )}
          {page === "settings" && (
            <Settings
              notice={notify}
              profile={profile}
              settings={appearance}
              save={saveAppearance}
              logout={logout}
              openWhatsNew={() => setWhatsNew(true)}
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
        </main>
        <Player
          queue={playQueue}
          select={setPlaying}
          track={playing}
          version={version}
          next={() => skip(1)}
          previous={() => skip(-1)}
          notice={notify}
        />
      </div>
      <Artwork
        selected={artwork}
        close={() => setArtwork(null)}
        changed={() => setVersion((n) => n + 1)}
        notice={notify}
      />
      <WhatsNew open={whatsNew} onClose={() => setWhatsNew(false)} />
    </>
  );
}
