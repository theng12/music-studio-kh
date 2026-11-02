import React, { useEffect, useMemo, useRef, useState } from "react";
import { art, api, sendDesktopCommand, Track } from "./api";

const time = (n: number) =>
  Number.isFinite(n)
    ? Math.floor(n / 60) + ":" + String(Math.floor(n % 60)).padStart(2, "0")
    : "0:00";

interface LyricLine {
  time: number;
  text: string;
}

function parseLrc(lrcText: string): LyricLine[] | null {
  if (!lrcText) return null;
  const lines = lrcText.split("\n");
  const result: LyricLine[] = [];
  const lrcRegex = /^\[(\d{1,2}):(\d{2})(?:\.(\d{1,3}))?\](.*)$/;
  let foundAny = false;
  for (const rawLine of lines) {
    const line = rawLine.trim();
    const match = line.match(lrcRegex);
    if (match) {
      foundAny = true;
      const min = parseInt(match[1], 10);
      const sec = parseInt(match[2], 10);
      const msStr = match[3] || "0";
      const ms = parseInt(msStr.padEnd(3, "0").slice(0, 3), 10);
      const t = min * 60 + sec + ms / 1000;
      const text = match[4].trim();
      result.push({ time: t, text });
    }
  }
  if (!foundAny || result.length === 0) return null;
  return result.sort((a, b) => a.time - b.time);
}

export interface PlayerProps {
  queue: Track[];
  select: (t: Track) => void;
  track: Track | null;
  version: number;
  next: () => void;
  previous: () => void;
  notice: (m: string) => void;
  streamQuality?: string;
  repeatMode?: string;
  onRepeatModeChange?: (mode: "all" | "one" | "off") => void;
  onTrackUpdated?: (t: Track) => void;
  allTracks?: Track[];
}

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
  onRepeatModeChange,
  onTrackUpdated,
  allTracks = [],
}: PlayerProps) {
  const ref = useRef<HTMLAudioElement>(null);
  const lyricsContainerRef = useRef<HTMLDivElement>(null);
  const activeLyricRef = useRef<HTMLDivElement>(null);

  const [paused, setPaused] = useState(true);
  const [position, setPosition] = useState(0);
  const [duration, setDuration] = useState(0);
  const [volume, setVolume] = useState(0.8);
  const [muted, setMuted] = useState(false);
  const [playbackRate, setPlaybackRate] = useState(1);
  const [shuffle, setShuffle] = useState(false);
  const [showQueue, setShowQueue] = useState(false);
  const [showLyrics, setShowLyrics] = useState(false);
  const [editingLyrics, setEditingLyrics] = useState(false);
  const [lyricsDraft, setLyricsDraft] = useState("");
  const [savingLyrics, setSavingLyrics] = useState(false);
  const [miniMode, setMiniMode] = useState(false);
  const [pinned, setPinned] = useState(false);

  const currentRepeat = (repeatMode as "all" | "one" | "off") || "all";

  // Stream track changes
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
    audio.playbackRate = playbackRate;
    audio.muted = muted;
    audio
      .play()
      .catch(() =>
        notice("Playback unavailable. Check the file and try again."),
      );
  }, [track, streamQuality]);

  // Audio adjustments
  useEffect(() => {
    if (ref.current) ref.current.volume = volume;
  }, [volume]);

  useEffect(() => {
    if (ref.current) ref.current.muted = muted;
  }, [muted]);

  useEffect(() => {
    if (ref.current) ref.current.playbackRate = playbackRate;
  }, [playbackRate]);

  // Sync lyrics draft when track changes
  useEffect(() => {
    if (track) {
      setLyricsDraft(track.lyrics || "");
      if (!editingLyrics) {
        setLyricsDraft(track.lyrics || "");
      }
    }
  }, [track?.id, track?.lyrics]);

  // Parse LRC
  const parsedLrc = useMemo(() => {
    return track?.lyrics ? parseLrc(track.lyrics) : null;
  }, [track?.lyrics]);

  // Active lyric index
  const activeLyricIndex = useMemo(() => {
    if (!parsedLrc || parsedLrc.length === 0) return -1;
    let idx = -1;
    for (let i = 0; i < parsedLrc.length; i++) {
      if (position >= parsedLrc[i].time) {
        idx = i;
      } else {
        break;
      }
    }
    return idx;
  }, [parsedLrc, position]);

  // Auto-scroll active lyric
  useEffect(() => {
    if (activeLyricRef.current && lyricsContainerRef.current) {
      activeLyricRef.current.scrollIntoView({
        behavior: "smooth",
        block: "center",
      });
    }
  }, [activeLyricIndex]);

  const toggle = () => {
    const audio = ref.current;
    if (!audio) return;
    if (!track) {
      if (allTracks.length > 0) {
        select(allTracks[0]);
      } else {
        notice("No songs in your library yet.");
      }
      return;
    }
    if (audio.paused) {
      audio.play().catch(() => notice("Could not start playback."));
    } else {
      audio.pause();
    }
  };

  const playRandomNext = () => {
    const pool = queue.length > 0 ? queue : allTracks;
    if (pool.length === 0) return;
    if (pool.length === 1) {
      select(pool[0]);
      if (ref.current) {
        ref.current.currentTime = 0;
        ref.current.play().catch(() => {});
      }
      return;
    }
    const poolRemaining = pool.filter((t) => t.id !== track?.id);
    const chosen =
      poolRemaining.length > 0
        ? poolRemaining[Math.floor(Math.random() * poolRemaining.length)]
        : pool[0];
    select(chosen);
  };

  const handleNext = () => {
    if (!track) {
      if (allTracks.length > 0) select(allTracks[0]);
      return;
    }
    if (shuffle) {
      playRandomNext();
    } else {
      next();
    }
  };

  const handlePrevious = () => {
    if (!track) {
      if (allTracks.length > 0) select(allTracks[0]);
      return;
    }
    if (ref.current && ref.current.currentTime > 3) {
      ref.current.currentTime = 0;
      return;
    }
    if (shuffle) {
      playRandomNext();
    } else {
      previous();
    }
  };

  const cycleRepeat = () => {
    const modes: Array<"all" | "one" | "off"> = ["all", "one", "off"];
    const nextMode = modes[(modes.indexOf(currentRepeat) + 1) % modes.length];
    onRepeatModeChange?.(nextMode);
  };

  const seekRelative = (seconds: number) => {
    if (!ref.current) return;
    ref.current.currentTime = Math.max(
      0,
      Math.min(duration || 0, ref.current.currentTime + seconds),
    );
  };

  const toggleMini = () => {
    const nextVal = !miniMode;
    setMiniMode(nextVal);
    document.documentElement.dataset.mini = nextVal ? "true" : "false";
    sendDesktopCommand(nextVal ? "mini-on" : "mini-off");
  };

  const togglePin = () => {
    const nextVal = !pinned;
    setPinned(nextVal);
    sendDesktopCommand("toggle-pin");
  };

  const hideToMenuBar = () => {
    sendDesktopCommand("hide-window");
  };

  const saveLyrics = async () => {
    if (!track) return;
    setSavingLyrics(true);
    try {
      let updated: Track;
      try {
        updated = await api<Track>(`/api/tracks/${track.id}`, {
          method: "PATCH",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ lyrics: lyricsDraft }),
        });
      } catch {
        updated = await api<Track>(`/api/tracks/${track.id}`, {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            title: track.title,
            artist: track.artist,
            album: track.album,
            version: track.version,
            year: track.year,
            trackNumber: track.trackNumber,
            lyrics: lyricsDraft,
          }),
        });
      }
      onTrackUpdated?.(updated);
      setEditingLyrics(false);
      notice("Lyrics updated.");
    } catch (e) {
      notice((e as Error).message || "Could not save lyrics");
    } finally {
      setSavingLyrics(false);
    }
  };

  // Global menu command hook & state broadcast
  useEffect(() => {
    const handler = (cmd: string) => {
      switch (cmd) {
        case "play_pause":
          toggle();
          break;
        case "next_track":
          handleNext();
          break;
        case "prev_track":
          handlePrevious();
          break;
        case "toggle_shuffle":
          setShuffle((s) => !s);
          break;
        case "cycle_repeat":
          cycleRepeat();
          break;
        case "toggle_lyrics":
          setShowLyrics((l) => !l);
          break;
        case "toggle_mini":
          toggleMini();
          break;
        case "toggle_pin":
          togglePin();
          break;
      }
    };
    (window as any).__TUNEHARBOR_MENU_CMD__ = handler;
    return () => {
      if ((window as any).__TUNEHARBOR_MENU_CMD__ === handler) {
        delete (window as any).__TUNEHARBOR_MENU_CMD__;
      }
    };
  });

  useEffect(() => {
    (window as any).__TH_DESKTOP_STATE__ = {
      playing: !paused,
      title: track?.title || "",
      artist: track?.artist || "",
      shuffle,
      repeat: currentRepeat,
      lyrics: showLyrics,
      mini: miniMode,
      pinned,
    };
  }, [paused, track, shuffle, currentRepeat, showLyrics, miniMode, pinned]);

  return (
    <>
      <footer className={"player" + (miniMode ? " mini-player" : "")}>
        <div className="playing-info">
          <div className="playing-cover" id="playing-cover">
            {track ? <img src={art(track.id, version)} alt="" /> : "♪"}
          </div>
          <div className="playing-text">
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
              className={"icon-button " + (shuffle ? "active" : "")}
              id="shuffle-btn"
              aria-label={`Shuffle: ${shuffle ? "On" : "Off"}`}
              title={`Shuffle: ${shuffle ? "On" : "Off"}`}
              onClick={() => setShuffle(!shuffle)}
            >
              🔀
            </button>
            <button
              className="icon-button"
              id="previous"
              aria-label="Previous song"
              title="Previous song"
              onClick={handlePrevious}
            >
              ⏮︎
            </button>
            <button
              className="icon-button seek-step-btn"
              id="seek-back"
              aria-label="Rewind 10 seconds"
              title="Rewind 10s"
              onClick={() => seekRelative(-10)}
            >
              -10s
            </button>
            <button
              className="play-button"
              id="play-pause"
              aria-label={paused ? "Play" : "Pause"}
              title={paused ? "Play" : "Pause"}
              onClick={toggle}
            >
              {paused ? "▶" : "Ⅱ"}
            </button>
            <button
              className="icon-button seek-step-btn"
              id="seek-forward"
              aria-label="Forward 10 seconds"
              title="Forward 10s"
              onClick={() => seekRelative(10)}
            >
              +10s
            </button>
            <button
              className="icon-button"
              id="next"
              aria-label="Next song"
              title="Next song"
              onClick={handleNext}
            >
              ⏭︎
            </button>
            <button
              className={
                "icon-button repeat-btn " +
                (currentRepeat !== "off" ? "active" : "")
              }
              id="repeat-btn"
              aria-label={`Repeat mode: ${currentRepeat}`}
              title={`Repeat: ${currentRepeat === "all" ? "Loop queue" : currentRepeat === "one" ? "Repeat current song" : "Off"}`}
              onClick={cycleRepeat}
            >
              {currentRepeat === "one"
                ? "🔂"
                : currentRepeat === "all"
                  ? "🔁"
                  : "→"}
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

        <div className="player-extras">
          <select
            id="playback-speed"
            className="playback-speed"
            aria-label="Playback speed"
            title="Playback speed"
            value={playbackRate}
            onChange={(e) => setPlaybackRate(Number(e.target.value))}
          >
            <option value="0.75">0.75×</option>
            <option value="1">1×</option>
            <option value="1.25">1.25×</option>
            <option value="1.5">1.5×</option>
            <option value="2">2×</option>
          </select>

          <button
            className={"icon-button " + (showLyrics ? "active" : "")}
            id="lyrics-btn"
            aria-label="Lyrics"
            title="Toggle lyrics drawer"
            onClick={() => setShowLyrics(!showLyrics)}
          >
            🎤 <span>Lyrics</span>
          </button>

          <button
            className={"icon-button " + (showQueue ? "active" : "")}
            id="queue-btn"
            aria-label="Show play queue"
            title="Up next queue"
            onClick={() => setShowQueue(!showQueue)}
          >
            ☰ <span>Queue</span>
          </button>

          <div className="volume">
            <button
              className="icon-button"
              id="mute-btn"
              aria-label={muted ? "Unmute" : "Mute"}
              title={muted ? "Unmute" : "Mute"}
              onClick={() => setMuted(!muted)}
            >
              {muted || volume === 0 ? "🔇" : volume < 0.5 ? "🔉" : "🔊"}
            </button>
            <input
              id="volume"
              aria-label="Volume"
              type="range"
              min="0"
              max="1"
              step=".01"
              value={muted ? 0 : volume}
              onChange={(e) => {
                setVolume(Number(e.target.value));
                if (muted) setMuted(false);
              }}
            />
          </div>

          <div className="window-controls">
            <button
              className={"icon-button " + (miniMode ? "active" : "")}
              id="mini-player-btn"
              aria-label={miniMode ? "Exit mini player" : "Mini player mode"}
              title={miniMode ? "Exit mini player mode" : "Mini player mode"}
              onClick={toggleMini}
            >
              {miniMode ? "⤢" : "⧉"}
            </button>
            <button
              className={"icon-button " + (pinned ? "active pinned" : "")}
              id="pin-window-btn"
              aria-label={pinned ? "Unpin window" : "Pin window on top"}
              title={pinned ? "Unpin window" : "Pin window on top"}
              onClick={togglePin}
            >
              📌
            </button>
            <button
              className="icon-button"
              id="hide-window-btn"
              aria-label="Hide to menu bar"
              title="Hide to menu bar"
              onClick={hideToMenuBar}
            >
              ⤓
            </button>
          </div>
        </div>

        <audio
          id="audio"
          ref={ref}
          preload="metadata"
          onPlay={() => setPaused(false)}
          onPause={() => setPaused(true)}
          onEnded={() => {
            if (currentRepeat === "one" && ref.current) {
              ref.current.currentTime = 0;
              ref.current.play().catch(() => {});
            } else if (currentRepeat !== "off") {
              handleNext();
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

      {showQueue && (
        <section
          className="player-queue settings-card"
          role="region"
          aria-label="Up next queue"
        >
          <button
            className="close-dialog"
            aria-label="Close queue"
            onClick={() => setShowQueue(false)}
          >
            ×
          </button>
          <h3>Up next</h3>
          {queue.length === 0 ? (
            <p className="muted">No songs in queue.</p>
          ) : (
            queue.map((t) => (
              <button
                className={
                  "album-song " + (t.id === track?.id ? "selected" : "")
                }
                key={t.id}
                onClick={() => select(t)}
              >
                <strong>{t.title}</strong>
                <small>{t.artist}</small>
              </button>
            ))
          )}
        </section>
      )}

      {showLyrics && (
        <section
          className="lyrics-panel settings-card"
          id="lyrics-panel"
          role="region"
          aria-label="Lyrics drawer"
        >
          <button
            className="close-dialog"
            aria-label="Close lyrics"
            onClick={() => setShowLyrics(false)}
          >
            ×
          </button>
          <div className="lyrics-header">
            <div>
              <span className="eyebrow">SONG LYRICS</span>
              <h3>{track?.title || "Lyrics"}</h3>
              <p className="lyrics-artist">
                {track?.artist || "No song selected"}
              </p>
            </div>
            <button
              id="edit-lyrics-btn"
              type="button"
              className="secondary compact-button"
              onClick={() => {
                setEditingLyrics(!editingLyrics);
                setLyricsDraft(track?.lyrics || "");
              }}
            >
              {editingLyrics ? "Cancel edit" : "Add / Edit lyrics"}
            </button>
          </div>

          {editingLyrics ? (
            <div className="lyrics-editor">
              <textarea
                id="lyrics-textarea"
                rows={12}
                value={lyricsDraft}
                onChange={(e) => setLyricsDraft(e.target.value)}
                placeholder="Paste plain lyrics or [mm:ss] LRC time-synced lyrics here..."
              />
              <div className="action-row" style={{ marginTop: "12px" }}>
                <button
                  id="save-lyrics-btn"
                  className="primary compact-button"
                  type="button"
                  disabled={savingLyrics || !track}
                  onClick={saveLyrics}
                >
                  {savingLyrics ? "Saving…" : "Save lyrics"}
                </button>
                <button
                  className="secondary compact-button"
                  type="button"
                  onClick={() => setEditingLyrics(false)}
                >
                  Cancel
                </button>
              </div>
            </div>
          ) : parsedLrc && parsedLrc.length > 0 ? (
            <div className="lyrics-content lrc-scroll" ref={lyricsContainerRef}>
              {parsedLrc.map((line, idx) => (
                <div
                  key={idx}
                  ref={activeLyricIndex === idx ? activeLyricRef : null}
                  className={
                    "lyric-line " +
                    (activeLyricIndex === idx ? "active-lyric-line" : "")
                  }
                  onClick={() => {
                    if (ref.current) ref.current.currentTime = line.time;
                  }}
                  title="Click to seek"
                >
                  {line.text || "♪"}
                </div>
              ))}
            </div>
          ) : track?.lyrics && track.lyrics.trim() ? (
            <div className="lyrics-content plain-lyrics">
              {track.lyrics.split("\n").map((line, idx) => (
                <p key={idx} className="plain-lyric-line">
                  {line || "\u00A0"}
                </p>
              ))}
            </div>
          ) : (
            <div className="empty-lyrics">
              <p>No lyrics available for this song yet.</p>
              <button
                id="edit-lyrics-btn-empty"
                className="secondary compact-button"
                style={{ marginTop: "12px" }}
                onClick={() => {
                  setEditingLyrics(true);
                  setLyricsDraft("");
                }}
              >
                ＋ Add lyrics
              </button>
            </div>
          )}
        </section>
      )}
    </>
  );
}
