import React, { useRef, useState } from "react";
import { ImportItem, MetadataInput } from "./api";
import { isImportableFile } from "./drop";

export default function ImportDialog({
  open,
  onClose,
  onImport,
}: {
  open: boolean;
  onClose: () => void;
  onImport: (
    items: ImportItem[],
    meta: MetadataInput,
    artworkFile: File | null,
  ) => Promise<void>;
}) {
  const [items, setItems] = useState<ImportItem[]>([]);
  const [title, setTitle] = useState("");
  const [artist, setArtist] = useState("");
  const [album, setAlbum] = useState("");
  const [version, setVersion] = useState("");
  const [year, setYear] = useState("");
  const [lyrics, setLyrics] = useState("");
  const [artwork, setArtwork] = useState<File | null>(null);
  const [busy, setBusy] = useState(false);

  const fileInput = useRef<HTMLInputElement>(null);
  const folderInput = useRef<HTMLInputElement>(null);

  if (!open) return null;

  const audioItems = items.filter((it) =>
    /\.(mp3|m4a|aac|flac|ogg|opus|wav)$/i.test(it.file.name),
  );

  const selectFiles = (selected: File[], fromFolder: boolean) => {
    const mapped: ImportItem[] = selected
      .filter((f) => isImportableFile(f.name, fromFolder))
      .map((file) => ({
        file,
        relPath: (file as any).webkitRelativePath || file.name,
      }));
    setItems(mapped);

    const audios = mapped.filter((it) =>
      /\.(mp3|m4a|aac|flac|ogg|opus|wav)$/i.test(it.file.name),
    );
    if (audios.length === 1 && !title) {
      const stem = audios[0].file.name.replace(/\.[^.]+$/, "");
      setTitle(stem.replace(/^\s*\d{1,3}\s*(?:[-._]\s*|\s+)/, ""));
    }
    if (fromFolder && mapped.length > 0 && !album) {
      const parts = (mapped[0].relPath || "").split("/").filter(Boolean);
      if (parts.length >= 2) {
        const folderName = parts[parts.length - 2];
        setAlbum(folderName);
      }
    }
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!audioItems.length) return;
    setBusy(true);
    try {
      await onImport(
        items,
        {
          title: audioItems.length === 1 ? title.trim() : undefined,
          artist: artist.trim() || undefined,
          album: album.trim() || undefined,
          version: version.trim() || undefined,
          year: year ? Number(year) : undefined,
          lyrics: lyrics.trim() || undefined,
        },
        artwork,
      );
      setItems([]);
      setTitle("");
      setArtist("");
      setAlbum("");
      setVersion("");
      setYear("");
      setLyrics("");
      setArtwork(null);
      onClose();
    } finally {
      setBusy(false);
    }
  };

  return (
    <div
      className="modal-backdrop"
      onClick={(e) => {
        if (e.target === e.currentTarget && !busy) onClose();
      }}
    >
      <form
        className="settings-card profile-form"
        id="custom-import-dialog"
        onSubmit={submit}
      >
        <button
          type="button"
          className="close-dialog"
          aria-label="Close import dialog"
          onClick={onClose}
        >
          ×
        </button>
        <span className="eyebrow">CUSTOM IMPORT</span>
        <h2>Import with details & artwork</h2>
        <p>
          Choose songs or an entire album folder, and set custom titles, album
          names, versions, or artwork before importing.
        </p>

        <div className="action-row">
          <button
            type="button"
            className="secondary"
            id="pick-import-files"
            onClick={() => fileInput.current?.click()}
          >
            ♫ Choose audio files…
          </button>
          <button
            type="button"
            className="secondary"
            id="pick-import-folder"
            onClick={() => folderInput.current?.click()}
          >
            📁 Choose music folder…
          </button>
        </div>

        <input
          id="custom-import-files"
          ref={fileInput}
          type="file"
          multiple
          accept=".mp3,.m4a,.aac,.flac,.ogg,.opus,.wav"
          hidden
          onChange={(e) => {
            selectFiles([...(e.target.files || [])], false);
            e.target.value = "";
          }}
        />
        <input
          id="custom-import-folder"
          ref={folderInput}
          type="file"
          multiple
          {...({ webkitdirectory: "", directory: "" } as any)}
          hidden
          onChange={(e) => {
            selectFiles([...(e.target.files || [])], true);
            e.target.value = "";
          }}
        />

        <small className="import-selection-summary" id="import-selection-summary">
          {audioItems.length > 0
            ? `${audioItems.length} ${audioItems.length === 1 ? "song" : "songs"} selected (${audioItems[0].relPath || audioItems[0].file.name}${audioItems.length > 1 ? ` + ${audioItems.length - 1} more` : ""})`
            : "No files or folder selected yet."}
        </small>

        {audioItems.length === 1 && (
          <label>
            Song title
            <input
              id="import-title"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Leave blank to use file tag or filename"
              maxLength={240}
            />
          </label>
        )}

        <div className="form-columns">
          <label>
            Artist
            <input
              id="import-artist"
              value={artist}
              onChange={(e) => setArtist(e.target.value)}
              placeholder="e.g. Daft Punk"
              maxLength={180}
            />
          </label>
          <label>
            Album
            <input
              id="import-album"
              value={album}
              onChange={(e) => setAlbum(e.target.value)}
              placeholder="e.g. Discovery"
              maxLength={240}
            />
          </label>
        </div>

        <div className="form-columns">
          <label>
            Version / edition
            <input
              id="import-version"
              value={version}
              onChange={(e) => setVersion(e.target.value)}
              placeholder="e.g. Deluxe, Acoustic, Live, Remastered"
              maxLength={120}
            />
          </label>
          <label>
            Release year
            <input
              id="import-year"
              type="number"
              min="1900"
              max="2100"
              value={year}
              onChange={(e) => setYear(e.target.value)}
              placeholder="e.g. 2026"
            />
          </label>
        </div>

        <label>
          Lyrics (optional · plain text or [mm:ss] LRC)
          <textarea
            id="import-lyrics"
            rows={4}
            value={lyrics}
            onChange={(e) => setLyrics(e.target.value)}
            placeholder="Paste plain lyrics or [mm:ss] LRC time-synced lyrics here…"
          />
        </label>

        <label className="artwork-upload">
          Cover artwork (optional · JPEG, PNG, or WebP)
          <input
            id="import-artwork"
            type="file"
            accept="image/jpeg,image/png,image/webp"
            onChange={(e) => setArtwork(e.target.files?.[0] || null)}
          />
          {artwork && <small>Selected cover: {artwork.name}</small>}
        </label>

        <button
          className="primary"
          id="submit-custom-import"
          type="submit"
          disabled={busy || audioItems.length === 0}
        >
          {busy
            ? "Importing…"
            : `Import ${audioItems.length || ""} ${audioItems.length === 1 ? "song" : "songs"} ↗`}
        </button>
      </form>
    </div>
  );
}
