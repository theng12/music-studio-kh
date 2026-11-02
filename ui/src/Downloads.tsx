import React, { useEffect, useState } from "react";
import { api, Job } from "./api";
const labels: Record<string, string> = {
  paused: "Waiting for music folder",
  queued: "Queued",
  running: "Downloading",
  waiting: "Playlist in progress",
  completed: "In your library",
  failed: "Download failed",
  canceled: "Canceled",
  interrupted: "Ready to retry",
};
export default function Downloads({
  jobs,
  refresh,
  notice,
}: {
  jobs: Job[];
  refresh: () => Promise<void>;
  notice: (s: string) => void;
}) {
  const [url, setURL] = useState(""),
    [busy, setBusy] = useState(false),
    [filter, setFilter] = useState("all"),
    [destination, setDestination] = useState(""),
    [destinationError, setDestinationError] = useState("");
  useEffect(() => {
    let active = true;
    api<{ library: string }>("/api/me")
      .then((profile) => {
        if (active) {
          if (profile.library) setDestination(profile.library);
          else setDestinationError("The server did not provide a music folder.");
        }
      })
      .catch((e) => {
        if (active) setDestinationError(e.message);
      });
    return () => { active = false; };
  }, []);
  const submit = async (value: string) => {
    if (!value.trim() || busy) return;
    setBusy(true);
    try {
      const links = value.split(/\r?\n/).map((s) => s.trim()).filter(Boolean);
      for (const url of links)
        await api("/api/downloads", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ url }),
        });
      setURL("");
      notice("Added to your download queue.");
    } catch (e) {
      notice((e as Error).message);
    } finally {
      setBusy(false);
      await refresh().catch((e) => notice(e.message));
    }
  };
  const action = async (job: Job, kind: string) => {
    try {
      await api("/api/downloads/" + job.id + "/" + kind, { method: "POST" });
      await refresh();
    } catch (e) {
      notice((e as Error).message);
    }
  };
  const shown = jobs.filter((j) =>
    filter === "all" ||
    (filter === "active" && ["queued", "running", "waiting", "paused"].includes(j.state)) ||
    (filter === "complete" && j.state === "completed") ||
    (filter === "attention" && ["failed", "interrupted", "paused"].includes(j.state)),
  );
  const active = jobs.filter((j) => !j.parentId &&
    ["queued", "running", "waiting", "paused"].includes(j.state)).length;
  const failed = jobs.filter((j) => j.state === "failed").length;
  return (
    <section className="downloads-page">
      <div className="download-entry"
        onDragOver={(e) => e.preventDefault()}
        onDrop={(e) => {
          e.preventDefault();
          const text = e.dataTransfer.getData("text/uri-list") || e.dataTransfer.getData("text/plain");
          if (text) submit(text);
        }}>
        <h2>Add from YouTube</h2>
        <p>Paste a song or playlist link. Finished audio is added to your library automatically.</p>
        <form onSubmit={(e) => { e.preventDefault(); submit(url); }}>
          <label className="sr-only" htmlFor="download-url">YouTube video or playlist link</label>
          <input id="download-url" value={url} onChange={(e) => setURL(e.target.value)}
            placeholder="Paste a YouTube video or playlist link…" required disabled={busy} />
          <button className="primary" disabled={busy || !url.trim()} type="submit">
            {busy ? "Adding…" : "Add to queue"}
          </button>
        </form>
        <div className="download-destination" aria-live="polite">
          <strong>Save location</strong>
          {destination ? <>
            <span id="download-destination">{destination}</span>
            <p>This folder is on the computer running Music Studio KH. Manage it in Settings.</p>
          </> : destinationError ?
            <p role="alert">Could not load the save location: {destinationError}</p> :
            <p>Loading save location…</p>}
        </div>
      </div>
      <div className="library-toolbar">
        <div>
          <h2>Download queue <span className="count">{jobs.length}</span></h2>
          <p aria-live="polite">{active} in progress{failed ? ` · ${failed} failed — see the reason below` : ""}</p>
        </div>
        <button className="secondary" onClick={() => refresh().catch((e) => notice(e.message))}>Refresh</button>
      </div>
      <div className="queue-tabs" role="group" aria-label="Filter downloads">
        {[["all", "All downloads"], ["active", "In progress"], ["complete", "Completed"], ["attention", "Needs attention"]].map(([id, label]) => (
          <button key={id} className={filter === id ? "selected" : ""}
            aria-pressed={filter === id} onClick={() => setFilter(id)}>{label}</button>
        ))}
      </div>
      {!shown.length ? (
        <div className="empty queue-empty">
          <h3>{jobs.length ? "Nothing in this view" : "No downloads yet"}</h3>
          <p>{jobs.length ? "Try another filter." : "Add a YouTube song or playlist above. You can also drop a link into the box."}</p>
        </div>
      ) : (
        <div className="queue-list">
          {shown.map((job) => (
            <article key={job.id} className={"queue-card " + (job.parentId ? "queue-child" : "")} data-state={job.state}>
              <div className="queue-content">
                <div className="queue-title">
                  <strong>{job.title || "YouTube audio"}</strong>
                  <span className={"state-pill " + job.state}>{labels[job.state] || job.state}</span>
                </div>
                <p>{job.parentId ? "Song from your playlist" : job.url}</p>
                {["running", "waiting"].includes(job.state) && (
                  <div className="job-progress">
                    <progress aria-label={`Download progress for ${job.title || "YouTube audio"}`} max="100" value={job.progress || undefined} />
                    <span>{job.progress > 0 ? Math.round(job.progress) + "%" : "Preparing audio…"}</span>
                  </div>
                )}
                {job.error && <p className="queue-error">{job.error}</p>}
              </div>
              <div className="queue-actions">
                {["queued", "running", "waiting", "paused"].includes(job.state) && (
                  <button className="subtle" onClick={() => action(job, "cancel")}>Cancel</button>
                )}
                {["failed", "canceled", "interrupted"].includes(job.state) && (
                  <button className="secondary" onClick={() => action(job, "retry")}>Retry</button>
                )}
              </div>
            </article>
          ))}
        </div>
      )}
    </section>
  );
}
