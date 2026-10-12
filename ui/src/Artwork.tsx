import React, { useEffect, useRef } from "react";
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
  useEffect(() => {
    if (selected) dialog.current?.showModal();
    else dialog.current?.close();
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
  return (
    <dialog id="artwork-dialog" ref={dialog} onCancel={close}>
      <button className="close-dialog" aria-label="Close" onClick={close}>
        ×
      </button>
      <span className="eyebrow">A PERSONAL TOUCH</span>
      <h2>{selected?.album ? "Album artwork" : "Song artwork"}</h2>
      <p id="artwork-title">
        {selected?.album ? selected.track.album : selected?.track.title}
      </p>
      <label className="artwork-upload">
        Choose a JPEG, PNG, or WebP image
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
    </dialog>
  );
}
