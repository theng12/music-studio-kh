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
const endedLabels: Record<string, string> = {
  completed: "Completed",
  failed: "Failed",
  canceled: "Canceled",
  interrupted: "Interrupted",
};
const fmtBytes = (n: number) => {
  if (!Number.isFinite(n) || n <= 0) return "";
  if (n < 1024) return n + " B";
  const units = ["KB", "MB", "GB", "TB"];
  let v = n;
  let i = -1;
  do {
    v /= 1024;
    i++;
  } while (v >= 1024 && i < units.length - 1);
  return (v >= 10 ? v.toFixed(0) : v.toFixed(1)) + " " + units[i];
};
const stateLabel = (job: Job) => {
  if (job.state === "running" && job.phase === "processing")
    return "Processing";
  if (job.state === "running" && job.phase === "importing")
    return "Adding to library";
  if (job.state === "running" && job.phase === "downloading")
    return "Downloading";
  return labels[job.state] || job.state;
};
const progressLabel = (job: Job) => {
  if (job.state === "running" && job.phase === "importing")
    return "Adding to library…";
  if (job.state === "running" && job.phase === "processing")
    return "Processing…";
  const parts: string[] = [];
  parts.push(
    job.progress > 0 ? Math.round(job.progress) + "%" : "Preparing audio…",
  );
  const total = job.bytesTotal || 0;
  if (total > 0) {
    const done = fmtBytes(job.bytesDone || 0);
    const size = fmtBytes(total);
    if (done && size) parts.push(done + " of " + size);
  }
  return parts.join(" · ");
};
export default function Downloads({
  jobs,
  refresh,
  notice,
  loading = false,
  error = "",
}: {
  jobs: Job[];
  refresh: () => Promise<void>;
  notice: (s: string) => void;
  loading?: boolean;
  error?: string;
}) {
  const [url, setURL] = useState(""),
    [busy, setBusy] = useState(false),
    [clearing, setClearing] = useState(false),
    [filter, setFilter] = useState("all"),
    [destination, setDestination] = useState(""),
    [destinationError, setDestinationError] = useState("");
  useEffect(() => {
    let active = true;
    api<{ library: string }>("/api/me")
      .then((profile) => {
        if (active) {
          if (profile.library) setDestination(profile.library);
          else
            setDestinationError("The server did not provide a music folder.");
        }
      })
      .catch((e) => {
        if (active) setDestinationError(e.message);
      });
    return () => {
      active = false;
    };
  }, []);
  const submit = async (value: string) => {
    if (!value.trim() || busy) return;
    setBusy(true);
    try {
      const links = value
        .split(/\r?\n/)
        .map((s) => s.trim())
        .filter(Boolean);
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
  const clearFinished = async () => {
    if (clearing) return;
    setClearing(true);
    try {
      const res = await api<{ cleared: number }>(
        "/api/downloads/clear-finished",
        {
          method: "POST",
        },
      );
      notice(
        res.cleared
          ? "Cleared " +
              res.cleared +
              " finished from this list. Files in your library are not deleted."
          : "No finished downloads to clear.",
      );
      await refresh();
    } catch (e) {
      notice((e as Error).message);
    } finally {
      setClearing(false);
    }
  };
  const shown = jobs.filter(
    (j) =>
      filter === "all" ||
      (filter === "active" &&
        ["queued", "running", "waiting", "paused"].includes(j.state)) ||
      (filter === "complete" && j.state === "completed") ||
      (filter === "attention" &&
        ["failed", "interrupted", "paused"].includes(j.state)),
  );
  const active = jobs.filter(
    (j) =>
      !j.parentId &&
      ["queued", "running", "waiting", "paused"].includes(j.state),
  ).length;
  const failed = jobs.filter((j) => j.state === "failed").length;
  const finished = jobs.filter((j) =>
    ["completed", "failed", "canceled"].includes(j.state),
  ).length;
  return (
    <section className="downloads-page">
      <div
        className="download-entry"
        onDragOver={(e) => e.preventDefault()}
        onDrop={(e) => {
          e.preventDefault();
          const text =
            e.dataTransfer.getData("text/uri-list") ||
            e.dataTransfer.getData("text/plain");
          if (text) submit(text);
        }}
      >
        <h2>Add from YouTube</h2>
        <p>
          Paste a song or playlist link. Finished audio is added to your library
          automatically.
        </p>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            submit(url);
          }}
        >
          <label className="sr-only" htmlFor="download-url">
            YouTube video or playlist link
          </label>
          <input
            id="download-url"
            value={url}
            onChange={(e) => setURL(e.target.value)}
            placeholder="Paste a YouTube video or playlist link…"
            required
            disabled={busy}
          />
          <button
            className="primary"
            disabled={busy || !url.trim()}
            type="submit"
          >
            {busy ? "Adding…" : "Add to queue"}
          </button>
        </form>
        <div className="download-destination" aria-live="polite">
          <strong>Save location</strong>
          {destination ? (
            <>
              <span id="download-destination">{destination}</span>
              <p>
                This folder is on the computer running Music Studio KH. Manage
                it in Settings.
              </p>
            </>
          ) : destinationError ? (
            <p role="alert">
              Could not load the save location: {destinationError}
            </p>
          ) : (
            <p>Loading save location…</p>
          )}
        </div>
      </div>
      <div className="library-toolbar">
        <div>
          <h2>
            Download queue <span className="count">{jobs.length}</span>
          </h2>
          <p aria-live="polite">
            {active} in progress
            {failed ? ` · ${failed} failed — see the reason below` : ""}
          </p>
          <p>
            Clear finished removes completed, failed, and canceled from this
            list only. Downloads in progress stay. Audio already in your library
            is not deleted.
          </p>
        </div>
        <div className="toolbar-controls">
          <button
            className="secondary"
            onClick={() => refresh().catch((e) => notice(e.message))}
          >
            Refresh
          </button>
          <button
            className="secondary"
            onClick={clearFinished}
            disabled={clearing || !finished}
          >
            Clear finished
          </button>
        </div>
      </div>
      {error && <p role="alert">Could not refresh download history: {error}</p>}
      <div className="queue-tabs" role="group" aria-label="Filter downloads">
        {[
          ["all", "All downloads"],
          ["active", "In progress"],
          ["complete", "Completed"],
          ["attention", "Needs attention"],
        ].map(([id, label]) => (
          <button
            key={id}
            className={filter === id ? "selected" : ""}
            aria-pressed={filter === id}
            onClick={() => setFilter(id)}
          >
            {label}
          </button>
        ))}
      </div>
      {!shown.length ? (
        loading && !jobs.length ? (
          <div className="empty queue-empty">
            <h3>Loading download history…</h3>
            <p>Reading the queue for this profile.</p>
          </div>
        ) : jobs.length ? (
          <div className="empty queue-empty">
            <h3>Nothing in this view</h3>
            <p>Try another filter.</p>
          </div>
        ) : error ? (
          <div className="empty queue-empty">
            <h3>Download history unavailable</h3>
            <p>Fix the error above, then choose Refresh.</p>
          </div>
        ) : (
          <div className="empty queue-empty">
            <h3>No downloads yet</h3>
            <p>
              Add a YouTube song or playlist above. You can also drop a link
              into the box.
            </p>
          </div>
        )
      ) : (
        <div className="queue-list">
          {shown.map((job) => (
            <article
              key={job.id}
              className={"queue-card " + (job.parentId ? "queue-child" : "")}
              data-state={job.state}
            >
              <div className="queue-content">
                <div className="queue-title">
                  <strong>{job.title || "YouTube audio"}</strong>
                  <span className={"state-pill " + job.state}>
                    {stateLabel(job)}
                  </span>
                </div>
                <p>{job.parentId ? "Song from your playlist" : job.url}</p>
                {["running", "waiting"].includes(job.state) && (
                  <div className="job-progress">
                    <progress
                      aria-label={`Download progress for ${job.title || "YouTube audio"}`}
                      max="100"
                      value={job.progress || undefined}
                    />
                    <span>{progressLabel(job)}</span>
                  </div>
                )}
                {!!job.completedAt &&
                  ["completed", "failed", "canceled", "interrupted"].includes(
                    job.state,
                  ) && (
                    <p>
                      {endedLabels[job.state] || "Ended"}{" "}
                      {new Date(job.completedAt).toLocaleString()}
                    </p>
                  )}
                {job.error && <p className="queue-error">{job.error}</p>}
              </div>
              <div className="queue-actions">
                {["queued", "running", "waiting", "paused"].includes(
                  job.state,
                ) && (
                  <button
                    className="subtle"
                    onClick={() => action(job, "cancel")}
                  >
                    Cancel
                  </button>
                )}
                {["failed", "canceled", "interrupted"].includes(job.state) && (
                  <button
                    className="secondary"
                    onClick={() => action(job, "retry")}
                  >
                    Retry
                  </button>
                )}
              </div>
            </article>
          ))}
        </div>
      )}
    </section>
  );
}
