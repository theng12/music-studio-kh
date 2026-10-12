import React, { useMemo, useState } from "react";
import { art, Track } from "./api";
import { droppedFiles } from "./drop";
interface Props {
  notice: (s: string) => void;
  tracks: Track[];
  query: string;
  onQuery: (q: string) => void;
  onPlay: (t: Track, queue?: Track[]) => void;
  onArtwork: (t: Track, album: boolean) => void;
  chooseFiles: () => void;
  importFiles: (files: File[]) => Promise<void>;
  version: number;
  layout: string;
  toggleLayout: () => void;
}
const songs = (n: number) => n + (n === 1 ? " song" : " songs");
export default function Library(p: Props) {
  const [view, setView] = useState("songs"),
    [limit, setLimit] = useState(100),
    [album, setAlbum] = useState<Track[] | null>(null);
  const groups = useMemo(() => {
    const map = new Map<string, Track[]>();
    for (const t of p.tracks) {
      const key = view === "artists" ? t.artist : t.artist + "\0" + t.album;
      const group = map.get(key);
      if (group) group.push(t);
      else map.set(key, [t]);
    }
    return [...map.values()];
  }, [p.tracks, view]);
  const items = view === "songs" ? p.tracks.map((t) => [t]) : groups;
  return (
    <section id="library-page">
      <div className="hero">
        <div>
          <span className="eyebrow">MAKE YOURSELF AT HOME</span>
          <h2>
            Good music.
            <br />
            Your own little corner.
          </h2>
          <p>A collection that belongs to you.</p>
          <button className="primary" id="choose-music" onClick={p.chooseFiles}>
            ＋ Add music
          </button>
        </div>
        <div className="hero-art" aria-hidden="true">
          <div className="small-record" />
          <span className="hero-note">♫</span>
        </div>
      </div>
      <div className="library-toolbar">
        <div>
          <h2>
            Your collection{" "}
            <span className="count" id="track-count">
              {p.tracks.length}
            </span>
          </h2>
          <p>{songs(p.tracks.length)}, ready to listen.</p>
        </div>
        <div className="toolbar-controls">
          <label className="search-label">
            <span>⌕</span>
            <input
              id="search"
              type="search"
              aria-label="Search your library"
              placeholder="Search songs, artists, albums"
              value={p.query}
              onChange={(e) => {
                setLimit(100);
                p.onQuery(e.target.value);
              }}
            />
          </label>
          <button
            className="icon-button"
            id="layout-toggle"
            onClick={p.toggleLayout}
            aria-label="Toggle grid or list"
          >
            ▦
          </button>
        </div>
      </div>
      <div className="queue-tabs">
        {["songs", "albums", "artists"].map((v) => (
          <button
            key={v}
            className={view === v ? "selected" : ""}
            onClick={() => {
              setView(v);
              setLimit(100);
            }}
          >
            {v[0].toUpperCase() + v.slice(1)}
          </button>
        ))}
      </div>
      <div
        id="drop-zone"
        onDragOver={(e) => {
          e.preventDefault();
          e.currentTarget.classList.add("dragging");
        }}
        onDragLeave={(e) => e.currentTarget.classList.remove("dragging")}
        onDrop={(e) => {
          e.preventDefault();
          e.currentTarget.classList.remove("dragging");
          droppedFiles(e.dataTransfer)
            .then(p.importFiles)
            .catch((e) => p.notice(e.message));
        }}
      >
        {!items.length ? (
          <div id="empty-library" className="empty">
            <span className="empty-note">♫</span>
            <h3>
              {p.query ? "No songs found" : "Your next favorite starts here"}
            </h3>
            <p>
              {p.query
                ? "Try another title, artist, or album."
                : "Drop music files here, or choose files to add to your library."}
            </p>
            <button
              id="empty-add"
              className="secondary"
              onClick={p.chooseFiles}
            >
              Choose music files
            </button>
            <small>MP3 · M4A · AAC · FLAC · Ogg · Opus · WAV</small>
          </div>
        ) : (
          <div
            id="tracks"
            className={"track-grid " + (p.layout === "list" ? "list" : "")}
          >
            {items.slice(0, limit).map((group) => {
              const t = group[0];
              const title =
                view === "songs"
                  ? t.title
                  : view === "albums"
                    ? t.album
                    : t.artist;
              return (
                <article className="track-card" key={view + title + t.id}>
                  <button
                    className="cover-button"
                    aria-label={"Play " + title}
                    onClick={() => {
                      if (view === "artists") {
                        p.onQuery(t.artist);
                        setView("albums");
                      } else if (view === "albums") setAlbum(group);
                      else p.onPlay(t);
                    }}
                  >
                    <img src={art(t.id, p.version)} alt="" loading="lazy" />
                    <span className="cover-play">▶</span>
                  </button>
                  <div className="track-detail">
                    <div>
                      <strong>{title}</strong>
                      <p>
                        {view === "songs"
                          ? t.artist + " · " + t.album
                          : view === "albums"
                            ? t.artist + " · " + songs(group.length)
                            : songs(group.length)}
                      </p>
                    </div>
                    <button
                      className="icon-button"
                      aria-label={"Edit artwork for " + title}
                      onClick={() => p.onArtwork(t, view === "albums")}
                    >
                      ⋯
                    </button>
                  </div>
                </article>
              );
            })}
          </div>
        )}
      </div>
      {items.length > limit && (
        <button
          className="secondary load-more"
          onClick={() => setLimit((n) => n + 100)}
        >
          Show more
        </button>
      )}
      {album && (
        <div className="modal-backdrop">
          <section className="settings-card album-dialog">
            <button
              className="close-dialog"
              aria-label="Close album"
              onClick={() => setAlbum(null)}
            >
              ×
            </button>
            <img
              className="album-cover"
              src={art(album[0].id, p.version)}
              alt=""
            />
            <span className="eyebrow">ALBUM</span>
            <h2>{album[0].album}</h2>
            <p>
              {album[0].artist} · {album.length} songs
            </p>
            <button
              className="primary"
              onClick={() => {
                p.onPlay(album[0], album);
                setAlbum(null);
              }}
            >
              ▶ Play album
            </button>
            <div className="album-songs">
              {album.map((t, index) => (
                <button
                  className="album-song"
                  key={t.id}
                  onClick={() => p.onPlay(t, album)}
                >
                  <span>{index + 1}</span>
                  <strong>{t.title}</strong>
                  <small>
                    {Math.floor(t.duration / 60)}:
                    {String(Math.floor(t.duration % 60)).padStart(2, "0")}
                  </small>
                </button>
              ))}
            </div>
          </section>
        </div>
      )}
    </section>
  );
}
