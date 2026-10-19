import React, { useEffect, useRef, useState } from "react";
import { art, Track } from "./api";
const time = (n: number) =>
  Number.isFinite(n)
    ? Math.floor(n / 60) + ":" + String(Math.floor(n % 60)).padStart(2, "0")
    : "0:00";
export default function Player({
  track,
  version,
  next,
  previous,
  notice,
  queue,
  select,
  streamQuality = "original",
  repeatMode = "all",
}: {
  queue: Track[];
  select: (t: Track) => void;
  track: Track | null;
  version: number;
  next: () => void;
  previous: () => void;
  notice: (m: string) => void;
  streamQuality?: string;
  repeatMode?: string;
}) {
  const ref = useRef<HTMLAudioElement>(null);
  const [paused, setPaused] = useState(true),
    [position, setPosition] = useState(0),
    [duration, setDuration] = useState(0),
    [volume, setVolume] = useState(0.8),
    [showQueue, setShowQueue] = useState(false);
  useEffect(() => {
    if (!track || !ref.current) return;
    const audio = ref.current;
    const mime: Record<string, string> = {
      mp3: "audio/mpeg",
      m4a: "audio/mp4",
      aac: "audio/aac",
      wav: "audio/wav",
      flac: "audio/flac",
      ogg: "audio/ogg",
      opus: "audio/ogg",
    };
    const convert =
      streamQuality === "mp3" ||
      !audio.canPlayType(mime[track.format] || "audio/mpeg");
    audio.src = "/api/stream/" + track.id + (convert ? "?format=mp3" : "");
    audio
      .play()
      .catch(() =>
        notice("Playback unavailable. Check the file and try again."),
      );
  }, [track, streamQuality]);
  useEffect(() => {
    if (ref.current) ref.current.volume = volume;
  }, [volume]);
  const toggle = () => {
    const audio = ref.current;
    if (!audio || !track) return;
    if (audio.paused)
      audio.play().catch(() => notice("Could not start playback."));
    else audio.pause();
  };
  return (
    <footer className="player">
      <div className="playing-info">
        <div className="playing-cover" id="playing-cover">
          {track ? <img src={art(track.id, version)} alt="" /> : "♪"}
        </div>
        <div>
          <strong id="playing-title">
            {track?.title || "Find your rhythm"}
          </strong>
          <span id="playing-artist">
            {track?.artist || "Choose a song to start listening"}
          </span>
        </div>
      </div>
      <div className="playback">
        <div className="playback-actions">
          <button
            className="icon-button"
            id="previous"
            aria-label="Previous song"
            onClick={previous}
          >
            ⏮︎
          </button>
          <button
            className="play-button"
            id="play-pause"
            aria-label="Play or pause"
            onClick={toggle}
          >
            {paused ? "▶" : "Ⅱ"}
          </button>
          <button
            className="icon-button"
            id="next"
            aria-label="Next song"
            onClick={next}
          >
            ⏭︎
          </button>
        </div>
        <div className="timeline">
          <span id="elapsed">{time(position)}</span>
          <input
            id="seek"
            aria-label="Seek"
            type="range"
            min="0"
            max="100"
            value={duration > 0 ? (position / duration) * 100 : 0}
            onChange={(e) => {
              if (ref.current && duration > 0)
                ref.current.currentTime =
                  (Number(e.target.value) / 100) * duration;
            }}
          />
          <span id="duration">{time(duration)}</span>
        </div>
      </div>
      {showQueue && (
        <section className="player-queue settings-card">
          <button
            className="close-dialog"
            aria-label="Close queue"
            onClick={() => setShowQueue(false)}
          >
            ×
          </button>
          <h3>Up next</h3>
          {queue.map((t) => (
            <button
              className={"album-song " + (t.id === track?.id ? "selected" : "")}
              key={t.id}
              onClick={() => select(t)}
            >
              {t.title}
              <small>{t.artist}</small>
            </button>
          ))}
        </section>
      )}
      <div className="volume">
        <button
          className="icon-button"
          aria-label="Show play queue"
          onClick={() => setShowQueue(!showQueue)}
        >
          ☷
        </button>
        <label htmlFor="volume">Volume</label>
        <input
          id="volume"
          type="range"
          min="0"
          max="1"
          step=".01"
          value={volume}
          onChange={(e) => setVolume(Number(e.target.value))}
        />
      </div>
      <audio
        id="audio"
        ref={ref}
        preload="metadata"
        onPlay={() => setPaused(false)}
        onPause={() => setPaused(true)}
        onEnded={() => {
          if (repeatMode === "one" && ref.current) {
            ref.current.currentTime = 0;
            ref.current.play().catch(() => {});
          } else if (repeatMode !== "off") {
            next();
          }
        }}
        onLoadedMetadata={() => setDuration(ref.current?.duration || 0)}
        onTimeUpdate={() => setPosition(ref.current?.currentTime || 0)}
        onError={() =>
          notice(
            "This file is unavailable or could not be played. Rescan the library or retry.",
          )
        }
      />
    </footer>
  );
}
