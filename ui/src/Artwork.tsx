import React, { useEffect, useRef, useState } from "react";
import { api, Track } from "./api";
export default function Artwork({
  selected,
  close,
  changed,
  notice,
}: {
  selected: { track: Track; album: boolean } | null;
  close: () => void;
  changed: () => void;
  notice: (m: string) => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [title, setTitle] = useState("");
  const [artist, setArtist] = useState("");
  const [album, setAlbum] = useState("");
  const [version, setVersion] = useState("");
  const [year, setYear] = useState("");
  const [trackNumber, setTrackNumber] = useState("");
  const [lyrics, setLyrics] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (selected) {
      const t = selected.track;
      setTitle(t.title || "");
      setArtist(t.artist || "");
      setAlbum(t.album || "");
      setVersion(t.version || "");
      setYear(t.year ? String(t.year) : "");
      setTrackNumber(t.trackNumber ? String(t.trackNumber) : "");
      setLyrics(t.lyrics || "");
      dialog.current?.showModal();
    } else {
      dialog.current?.close();
    }
  }, [selected]);

  const path = selected
    ? "/api/" +
      (selected.album ? "album-artwork/" : "artwork/") +
      selected.track.id
    : "";
  const upload = async (file?: File) => {
    if (!file) return;
    try {
      await api(path, { method: "PUT", body: file });
      changed();
      close();
      notice("Artwork updated.");
    } catch (e) {
      notice((e as Error).message);
    }
  };
  const saveDetails = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selected) return;
    setBusy(true);
    try {
      const endpoint = selected.album
        ? "/api/albums/" + selected.track.id
        : "/api/tracks/" + selected.track.id;
      await api(endpoint, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          title: selected.album ? undefined : title,
          artist,
          album,
          version,
          year: year ? Number(year) : 0,
          trackNumber: trackNumber ? Number(trackNumber) : 0,
          lyrics: selected.album ? undefined : (lyrics.trim() || undefined),
        }),
      });
      changed();
      close();
      notice(selected.album ? "Album details saved." : "Song details saved.");
    } catch (e) {
      notice((e as Error).message);
    } finally {
      setBusy(false);
    }
  };
  return (
    <dialog
      id="artwork-dialog"
      className="metadata-dialog"
      ref={dialog}
      onCancel={close}
    >
      <button className="close-dialog" aria-label="Close" onClick={close}>
        ×
      </button>
      <span className="eyebrow">A PERSONAL TOUCH</span>
      <h2>
        {selected?.album ? "Album details & artwork" : "Song details & artwork"}
      </h2>
      <p id="artwork-title">
        {selected?.album ? selected.track.album : selected?.track.title}
        {selected?.track.version ? ` · ${selected.track.version}` : ""}
      </p>

      <form className="metadata-form" onSubmit={saveDetails}>
        {!selected?.album && (
          <label>
            Song title
            <input
              id="edit-title"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              required
              maxLength={240}
            />
          </label>
        )}
        <div className="form-columns">
          <label>
            Artist
            <input
              id="edit-artist"
              value={artist}
              onChange={(e) => setArtist(e.target.value)}
              required
              maxLength={180}
            />
          </label>
          <label>
            Album
            <input
              id="edit-album"
              value={album}
              onChange={(e) => setAlbum(e.target.value)}
              required
              maxLength={240}
            />
          </label>
        </div>
        <div className="form-columns">
          <label>
            Version / edition
            <input
              id="edit-version"
              value={version}
              onChange={(e) => setVersion(e.target.value)}
              placeholder="e.g. Deluxe, Acoustic, Remastered"
              maxLength={120}
            />
          </label>
          <label>
            Release year
            <input
              id="edit-year"
              type="number"
              min="0"
              max="2100"
              value={year}
              onChange={(e) => setYear(e.target.value)}
              placeholder="e.g. 2026"
            />
          </label>
        </div>
        {!selected?.album && (
          <label>
            Track #
            <input
              id="edit-track-number"
              type="number"
              min="0"
              max="999"
              value={trackNumber}
              onChange={(e) => setTrackNumber(e.target.value)}
              placeholder="e.g. 1"
            />
          </label>
        )}
        {!selected?.album && (
          <label>
            Lyrics
            <textarea
              id="edit-lyrics"
              rows={4}
              value={lyrics}
              onChange={(e) => setLyrics(e.target.value)}
              placeholder="Paste plain lyrics or [mm:ss] LRC time-synced lyrics here…"
            />
          </label>
        )}
        <button
          className="primary"
          id="save-metadata"
          type="submit"
          disabled={busy}
        >
          {busy ? "Saving…" : "Save details"}
        </button>
      </form>

      <div className="artwork-section">
        <label className="artwork-upload">
          Choose a JPEG, PNG, or WebP cover image
          <input
            id="artwork-file"
            type="file"
            accept="image/jpeg,image/png,image/webp"
            onChange={(e) => {
              upload(e.target.files?.[0]);
              e.target.value = "";
            }}
          />
        </label>
        <small>Up to 10 MiB and 16 megapixels.</small>
        <button
          id="reset-artwork"
          type="button"
          className="secondary"
          onClick={async () => {
            try {
              await api(path, { method: "DELETE" });
              changed();
              close();
              notice("Original artwork restored.");
            } catch (e) {
              notice((e as Error).message);
            }
          }}
        >
          Reset to original artwork
        </button>
      </div>
    </dialog>
  );
}
