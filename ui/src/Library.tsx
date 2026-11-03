import React, { useMemo, useState } from "react";
import { art, ImportItem, Track } from "./api";
import { droppedFiles } from "./drop";
import Icon from "./Icon";
import { formatBytes, formatDuration } from "./format";

interface Props {
  notice: (s: string) => void;
  tracks: Track[];
  query: string;
  onQuery: (q: string) => void;
  onPlay: (t: Track, queue?: Track[]) => void;
  onPlayShuffle: (queue: Track[]) => void;
  onFavourite: (t: Track, next: boolean) => void;
  onArtwork: (t: Track, album: boolean) => void;
  chooseFiles: () => void;
  chooseFolder: () => void;
  openCustomImport: () => void;
  importFiles: (files: (File | ImportItem)[]) => Promise<void>;
  version: number;
  layout: string;
  defaultTab?: string;
  activeTab?: "songs" | "albums" | "artists" | "folders";
  onTabChange?: (tab: "songs" | "albums" | "artists" | "folders") => void;
  toggleLayout: () => void;
  newDays?: number;
}

type View = "songs" | "albums" | "artists" | "folders";

const songs = (n: number) => n + (n === 1 ? " song" : " songs");

// Mirrors the server-side search fields (title/artist/album/version/folder/album_artist).
const matchesQuery = (t: Track, needle: string) =>
  (t.title || "").toLowerCase().includes(needle) ||
  (t.artist || "").toLowerCase().includes(needle) ||
  (t.album || "").toLowerCase().includes(needle) ||
  (t.version || "").toLowerCase().includes(needle) ||
  (t.folder || "").toLowerCase().includes(needle) ||
  (t.albumArtist || "").toLowerCase().includes(needle);

const dateFormat = (unixSeconds: number) =>
  new Date(unixSeconds * 1000).toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });

const provenance = (t: Track) => {
  const parts: string[] = [];
  if (t.addedAt != null) parts.push("Added " + dateFormat(t.addedAt));
  if (t.downloadedAt != null)
    parts.push("Downloaded " + dateFormat(t.downloadedAt));
  return parts.join(" · ");
};

// A total is only exact when every track contributes a known value.
const totals = (group: Track[]) => {
  let size = 0;
  let duration = 0;
  let sizeKnown = true;
  let durationKnown = true;
  for (const t of group) {
    if (typeof t.size === "number" && t.size > 0) size += t.size;
    else sizeKnown = false;
    if (typeof t.duration === "number" && t.duration > 0)
      duration += t.duration;
    else durationKnown = false;
  }
  const parts: string[] = [];
  parts.push(
    durationKnown
      ? formatDuration(duration)
      : duration > 0
        ? "known duration " + formatDuration(duration)
        : "duration unknown",
  );
  parts.push(
    sizeKnown
      ? formatBytes(size)
      : size > 0
        ? "known size " + formatBytes(size)
        : "size unknown",
  );
  return parts.join(" · ");
};

export default function Library(p: Props) {
  const [localView, setLocalView] = useState<View>(
      (p.defaultTab as View) || "songs",
    ),
    [limit, setLimit] = useState(100),
    [album, setAlbum] = useState<Track[] | null>(null);
  const view = p.activeTab ?? localView;
  const setView = (v: View) => {
    if (p.onTabChange) p.onTabChange(v);
    else setLocalView(v);
  };
  const newDays = p.newDays ?? 7;

  // App hands over the whole index; search narrows what this page shows.
  const visible = useMemo(
    () =>
      p.query
        ? p.tracks.filter((t) => matchesQuery(t, p.query.toLowerCase()))
        : p.tracks,
    [p.tracks, p.query],
  );
  const groups = useMemo(() => {
    const map = new Map<string, Track[]>();
    for (const t of visible) {
      const keys =
        view === "artists"
          ? t.artists || [t.artist || "Unknown artist"]
          : [
              view === "folders"
                ? t.folder || "Root library"
                : t.artist + "\0" + t.album + "\0" + (t.version || ""),
            ];
      for (const key of keys) {
        const group = map.get(key);
        if (group) group.push(t);
        else map.set(key, [t]);
      }
    }
    return [...map].map(([key, tracks]) => ({
      title: view === "artists" || view === "folders" ? key : tracks[0].album,
      tracks,
    }));
  }, [visible, view]);
  const items =
    view === "songs"
      ? visible.map((t) => ({ title: t.title, tracks: [t] }))
      : groups;

  // Quick actions always cover the whole index, not the current search.
  const favourites = useMemo(
    () => p.tracks.filter((t) => t.favourite),
    [p.tracks],
  );
  const mostPlayed = useMemo(
    () =>
      p.tracks
        .filter((t) => (t.playCount ?? 0) > 0)
        .sort((a, b) => (b.playCount ?? 0) - (a.playCount ?? 0)),
    [p.tracks],
  );
  const libraryTotals = useMemo(() => totals(visible), [visible]);

  const isRecent = (t: Track) => {
    if (newDays <= 0) return false;
    const now = Date.now() / 1000;
    const within = (ts?: number) => ts != null && now - ts <= newDays * 86400;
    return within(t.addedAt) || within(t.downloadedAt);
  };
  const groupIsRecent = (group: Track[]) =>
    view === "songs" ? isRecent(group[0]) : group.some(isRecent);

  const viewHeading =
    view === "albums"
      ? "Albums"
      : view === "artists"
        ? "Artists"
        : view === "folders"
          ? "Folders"
          : "Songs";
  const countLine =
    view === "albums"
      ? `${items.length} ${items.length === 1 ? "album" : "albums"}`
      : view === "artists"
        ? `${items.length} ${items.length === 1 ? "artist" : "artists"}`
        : view === "folders"
          ? `${items.length} ${items.length === 1 ? "folder" : "folders"}`
          : songs(visible.length);
  const kind =
    view === "albums" ? "album" : view === "artists" ? "artist" : "folder";

  return (
    <section id="library-page">
      <div className="library-toolbar">
        <div>
          <h2>
            {viewHeading}{" "}
            <span className="count" id="track-count">
              {items.length}
            </span>
          </h2>
        </div>
        <div className="toolbar-controls">
          <div className="add-music-menu" role="group" aria-label="Add music">
            <button
              className="primary compact-button"
              id="choose-music"
              onClick={p.chooseFiles}
              title="Add individual music files"
            >
              <Icon name="plus" />
              Add music
            </button>
            <button
              className="secondary compact-button"
              id="choose-folder"
              onClick={p.chooseFolder}
              title="Import whole album or artist folder"
            >
              <Icon name="folder" />
              Folder
            </button>
            <button
              className="secondary compact-button"
              id="open-custom-import"
              onClick={p.openCustomImport}
              title="Import files with custom metadata and cover artwork"
            >
              <Icon name="edit" />
              Details…
            </button>
          </div>
          <label className="search-label">
            <span>
              <Icon name="search" />
            </span>
            <input
              id="search"
              type="search"
              aria-label="Search your library"
              placeholder="Search songs, artists, albums, versions, folders"
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
            title={
              p.layout === "list"
                ? "Switch to grid view"
                : "Switch to list view"
            }
          >
            <Icon name={p.layout === "list" ? "grid" : "list"} />
          </button>
        </div>
      </div>
      <p className="library-summary" aria-label="Indexed library summary">
        {countLine}
        {visible.length ? ` · ${libraryTotals}` : ""}
      </p>
      <div
        className="action-row quick-actions"
        role="group"
        aria-label="Quick play"
      >
        <button
          className="secondary"
          id="play-all"
          disabled={!p.tracks.length}
          onClick={() => p.onPlay(p.tracks[0], p.tracks)}
        >
          <Icon name="play" />
          Play all
        </button>
        <button
          className="secondary"
          id="play-favourites"
          disabled={!favourites.length}
          onClick={() => p.onPlay(favourites[0], favourites)}
        >
          <Icon name="heart" />
          Play favourites
        </button>
        <button
          className="secondary"
          id="play-most-played"
          disabled={!mostPlayed.length}
          onClick={() => p.onPlay(mostPlayed[0], mostPlayed)}
        >
          <Icon name="clock" />
          Play most played
        </button>
        <button
          className="secondary"
          id="shuffle-all"
          disabled={!p.tracks.length}
          onClick={() => p.onPlayShuffle(p.tracks)}
        >
          <Icon name="shuffle" />
          Shuffle all
        </button>
      </div>
      {(!favourites.length || !mostPlayed.length) && (
        <p className="quick-play-hints muted">
          {!favourites.length && <span>No favourites yet</span>}
          {!mostPlayed.length && <span>No listening history yet</span>}
        </p>
      )}
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
            <span className="empty-note">
              <Icon name="music" size={38} />
            </span>
            <h3>
              {p.query ? "No songs found" : "Your next favorite starts here"}
            </h3>
            <p>
              {p.query
                ? "Try another title, artist, album, or version."
                : "Drop music files or album folders here, or choose files or a folder to add to your library."}
            </p>
            <div className="action-row empty-actions">
              <button
                id="empty-add"
                className="secondary"
                onClick={p.chooseFiles}
              >
                Choose music files
              </button>
              <button
                id="empty-add-folder"
                className="secondary"
                onClick={p.chooseFolder}
              >
                Choose music folder
              </button>
              <button
                id="empty-custom-import"
                className="secondary"
                onClick={p.openCustomImport}
              >
                Import with details…
              </button>
            </div>
            <small>
              MP3 · M4A · AAC · FLAC · Ogg · Opus · WAV · Folder cover art
            </small>
          </div>
        ) : (
          <div
            id="tracks"
            className={"track-grid " + (p.layout === "list" ? "list" : "")}
          >
            {items.slice(0, limit).map(({ title, tracks: group }) => {
              const t = group[0];
              const badge =
                view === "songs" || view === "albums" ? t.version : "";
              const coverOpens = view !== "songs";
              const metadata = [
                (t.format || "").toUpperCase(),
                formatBytes(t.size),
                formatDuration(t.duration),
                t.path || t.folder || "Root library",
              ]
                .filter((part) => part && part !== "Unknown")
                .join(" · ");
              const dates = provenance(t);
              return (
                <article
                  className="track-card"
                  key={view + title + (t.version || "") + t.id}
                >
                  <button
                    className="cover-button"
                    aria-label={(coverOpens ? "Open " : "Play ") + title}
                    onClick={() => {
                      if (view === "artists") {
                        p.onQuery(title);
                        setView("albums");
                      } else if (view === "albums" || view === "folders")
                        setAlbum(group);
                      else p.onPlay(t, visible);
                    }}
                  >
                    <img src={art(t.id, p.version)} alt="" loading="lazy" />
                    <span className="cover-play">
                      <Icon name="play" size={14} />
                    </span>
                  </button>
                  <div className="track-detail">
                    <div>
                      <div className="track-title-row">
                        <strong>{title}</strong>
                        {badge && (
                          <span className="version-badge track-version">
                            {badge}
                          </span>
                        )}
                        {groupIsRecent(group) && (
                          <span className="new-badge">New</span>
                        )}
                      </div>
                      <p>
                        {view === "songs"
                          ? t.artist +
                            " · " +
                            t.album +
                            (t.year ? " · " + t.year : "")
                          : view === "albums"
                            ? t.artist +
                              " · " +
                              songs(group.length) +
                              (t.year ? " · " + t.year : "")
                            : view === "folders"
                              ? t.album + " · " + songs(group.length)
                              : songs(group.length)}
                      </p>
                      {view === "songs" ? (
                        <p className="track-metadata">
                          {metadata}
                          {dates ? " · " + dates : ""}
                        </p>
                      ) : (
                        <p className="group-summary">
                          {songs(group.length)} · {totals(group)}
                        </p>
                      )}
                    </div>
                    <div className="card-actions">
                      {view === "songs" ? (
                        <button
                          className="favourite-button"
                          aria-pressed={!!t.favourite}
                          aria-label={
                            (t.favourite ? "Remove" : "Add") +
                            " favourite: " +
                            t.title
                          }
                          title={
                            t.favourite
                              ? "Remove from favourites"
                              : "Add to favourites"
                          }
                          onClick={() => p.onFavourite(t, !t.favourite)}
                        >
                          <Icon name="heart" />
                        </button>
                      ) : (
                        <>
                          <button
                            className="icon-button"
                            aria-label={`Play ${kind} ${title}`}
                            title={`Play ${kind} ${title}`}
                            onClick={() => p.onPlay(group[0], group)}
                          >
                            <Icon name="play" />
                          </button>
                          <button
                            className="icon-button"
                            aria-label={`Shuffle ${kind} ${title}`}
                            title={`Shuffle ${kind} ${title}`}
                            onClick={() => p.onPlayShuffle(group)}
                          >
                            <Icon name="shuffle" />
                          </button>
                        </>
                      )}
                      <button
                        className="icon-button"
                        aria-label={"Edit artwork for " + title}
                        onClick={() =>
                          p.onArtwork(
                            t,
                            view === "albums" || view === "folders",
                          )
                        }
                        title="Edit artwork and metadata"
                      >
                        <Icon name="edit" />
                      </button>
                    </div>
                  </div>
                </article>
              );
            })}
          </div>
        )}
      </div>
      {limit < items.length && (
        <div className="action-row load-more">
          <button
            className="secondary"
            id="load-more"
            onClick={() => setLimit(limit + 100)}
          >
            Show more ({items.length - limit} remaining)
          </button>
        </div>
      )}
      {album && (
        <div className="modal-backdrop">
          <section
            className="settings-card album-dialog"
            role="dialog"
            aria-modal="true"
            aria-label="Album tracks"
          >
            <button
              className="close-dialog"
              aria-label="Close album"
              onClick={() => setAlbum(null)}
            >
              <Icon name="close" />
            </button>
            <img
              className="album-cover"
              src={art(album[0].id, p.version)}
              alt=""
            />
            <div className="whats-new-header">
              {album[0].version && (
                <span className="version-badge">{album[0].version}</span>
              )}
            </div>
            <h2>
              {view === "folders"
                ? album[0].folder || "Root library"
                : album[0].album}
            </h2>
            <p>
              {album[0].artist} · {songs(album.length)}
              {album[0].year ? ` · ${album[0].year}` : ""} · {totals(album)}
            </p>
            <div className="action-row">
              <button
                className="primary"
                onClick={() => {
                  p.onPlay(album[0], album);
                  setAlbum(null);
                }}
              >
                <Icon name="play" />
                Play album
              </button>
              <button
                className="secondary"
                onClick={() => {
                  p.onPlayShuffle(album);
                  setAlbum(null);
                }}
              >
                <Icon name="shuffle" />
                Shuffle album
              </button>
              <button
                className="secondary"
                onClick={() => {
                  const first = album[0];
                  setAlbum(null);
                  p.onArtwork(first, true);
                }}
              >
                <Icon name="edit" />
                Edit album & cover
              </button>
            </div>
            <div className="album-songs">
              {album.map((t, index) => (
                <div className="album-song" key={t.id}>
                  <button
                    type="button"
                    className="album-song-play"
                    onClick={() => p.onPlay(t, album)}
                  >
                    <span>{t.trackNumber || index + 1}</span>
                    <strong>
                      {t.title}
                      {t.version && t.version !== album[0].version
                        ? ` (${t.version})`
                        : ""}
                    </strong>
                    <small>
                      {(t.format || "").toUpperCase()} · {formatBytes(t.size)} ·{" "}
                      {formatDuration(t.duration)}
                    </small>
                  </button>
                  <button
                    type="button"
                    className="favourite-button"
                    aria-pressed={!!t.favourite}
                    aria-label={
                      (t.favourite ? "Remove" : "Add") +
                      " favourite: " +
                      t.title
                    }
                    title={
                      t.favourite
                        ? "Remove from favourites"
                        : "Add to favourites"
                    }
                    onClick={() => p.onFavourite(t, !t.favourite)}
                  >
                    <Icon name="heart" />
                  </button>
                </div>
              ))}
            </div>
          </section>
        </div>
      )}
    </section>
  );
}
