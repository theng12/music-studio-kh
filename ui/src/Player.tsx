import React, { useEffect, useMemo, useRef, useState } from "react";
import { art, api, sendDesktopCommand, Track } from "./api";
import Icon from "./Icon";

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
  shuffle?: boolean;
  onShuffleChange?: (s: boolean) => void;
  onFavourite: (t: Track, next: boolean) => void;
  onPlayed: (t: Track) => void;
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
  shuffle,
  onShuffleChange,
  onFavourite,
  onPlayed,
}: PlayerProps) {
  const ref = useRef<HTMLAudioElement>(null);
  const surfaceRef = useRef<HTMLElement>(null);
  useEffect(() => {
    const surface = surfaceRef.current;
    if (!surface) return;
    const measure = () =>
      document.documentElement.style.setProperty(
        "--player-clearance",
        `${innerHeight - surface.getBoundingClientRect().top + 12}px`,
      );
    const observer = new ResizeObserver(measure);
    observer.observe(surface);
    measure();
    return () => {
      observer.disconnect();
      document.documentElement.style.removeProperty("--player-clearance");
    };
  }, []);
  const lyricsContainerRef = useRef<HTMLDivElement>(null);
  const activeLyricRef = useRef<HTMLDivElement>(null);

  const [paused, setPaused] = useState(true);
  const [position, setPosition] = useState(0);
  const [duration, setDuration] = useState(0);
  const [volume, setVolume] = useState(0.8);
  const [muted, setMuted] = useState(false);
  const [playbackRate, setPlaybackRate] = useState(1);
  const [localShuffle, setLocalShuffle] = useState(false);
  const [showQueue, setShowQueue] = useState(false);
  const [showLyrics, setShowLyrics] = useState(false);
  const [editingLyrics, setEditingLyrics] = useState(false);
  const [lyricsDraft, setLyricsDraft] = useState("");
  const [savingLyrics, setSavingLyrics] = useState(false);
  const [miniMode, setMiniMode] = useState(false);
  const [pinned, setPinned] = useState(false);

  // Lifted shuffle when App provides the callback; local fallback otherwise.
  const shuffleOn = onShuffleChange ? !!shuffle : localShuffle;
  const setShuffleOn = (value: boolean) => {
    if (onShuffleChange) onShuffleChange(value);
    else setLocalShuffle(value);
  };

  const currentRepeat = (repeatMode as "all" | "one" | "off") || "all";

  // Playback-session bookkeeping: count real listened time only, once per session.
  const lastIdRef = useRef<string | null>(null);
  const resumeAtRef = useRef(0);
  const listenedRef = useRef(0);
  const lastTickRef = useRef(0);
  const countedRef = useRef(false);

  // Stream track changes. Depends on id only so favourite/lyrics metadata
  // merges rebuild props without restarting audio or transport time.
  useEffect(() => {
    if (!track) {
      lastIdRef.current = null;
      return;
    }
    const audio = ref.current;
    if (!audio) return;
    const sameSession = lastIdRef.current === track.id;
    resumeAtRef.current = sameSession ? audio.currentTime : 0;
    if (!sameSession) {
      lastIdRef.current = track.id;
      listenedRef.current = 0;
      lastTickRef.current = 0;
      countedRef.current = false;
    }
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
  }, [track?.id, streamQuality]);

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

  const seekTo = (seconds: number) => {
    const audio = ref.current;
    if (!audio) return;
    const max =
      Number.isFinite(audio.duration) && audio.duration > 0
        ? audio.duration
        : duration;
    const clamped = Math.max(0, Math.min(max || 0, seconds));
    audio.currentTime = clamped;
    // A user seek is not listened time.
    lastTickRef.current = clamped;
  };

  const seekRelative = (seconds: number) => {
    if (!ref.current) return;
    seekTo(ref.current.currentTime + seconds);
  };

  const restartCurrent = () => {
    const audio = ref.current;
    if (!audio) return;
    audio.currentTime = 0;
    lastTickRef.current = 0;
    listenedRef.current = 0;
    countedRef.current = false;
    audio.play().catch(() => {});
  };

  // Queue selection: same id resumes in place without touching queue/shuffle.
  const choose = (t: Track) => {
    if (t.id === track?.id) {
      const audio = ref.current;
      if (audio?.paused)
        audio.play().catch(() => notice("Could not start playback."));
      return;
    }
    select(t);
  };

  const toggle = () => {
    const audio = ref.current;
    if (!audio) return;
    if (!track) {
      if (allTracks.length > 0) {
        choose(allTracks[0]);
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
      if (pool[0].id === track?.id) restartCurrent();
      else select(pool[0]);
      return;
    }
    const poolRemaining = pool.filter((t) => t.id !== track?.id);
    const chosen =
      poolRemaining.length > 0
        ? poolRemaining[Math.floor(Math.random() * poolRemaining.length)]
        : pool[0];
    if (chosen.id === track?.id) restartCurrent();
    else select(chosen);
  };

  const handleNext = () => {
    if (!track) {
      if (allTracks.length > 0) choose(allTracks[0]);
      return;
    }
    if (shuffleOn) {
      playRandomNext();
    } else {
      next();
    }
  };

  const handlePrevious = () => {
    if (!track) {
      if (allTracks.length > 0) choose(allTracks[0]);
      return;
    }
    if (ref.current && ref.current.currentTime > 3) {
      seekTo(0);
      return;
    }
    if (shuffleOn) {
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

  // Real-play qualification: accumulate small positive timeupdate deltas
  // (seek jumps are excluded via seekTo), fire once per listening session.
  const handleTimeUpdate = () => {
    const audio = ref.current;
    if (!audio || !track) return;
    setPosition(audio.currentTime);
    if (audio.paused) return;
    const delta = audio.currentTime - lastTickRef.current;
    lastTickRef.current = audio.currentTime;
    if (delta < -0.5) {
      // Rewind or repeat restart begins a fresh listening session.
      listenedRef.current = 0;
      countedRef.current = false;
      return;
    }
    if (countedRef.current) return;
    if (delta > 0 && delta <= 2) listenedRef.current += delta;
    const real =
      Number.isFinite(audio.duration) && audio.duration > 0
        ? audio.duration
        : 0;
    // Qualify at 30s played or half the track, whichever comes first.
    const threshold = real > 0 ? Math.min(30, real / 2) : 30;
    if (listenedRef.current >= threshold) {
      countedRef.current = true;
      onPlayed(track);
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
          setShuffleOn(!shuffleOn);
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
      shuffle: shuffleOn,
      repeat: currentRepeat,
      lyrics: showLyrics,
      mini: miniMode,
      pinned,
    };
  }, [paused, track, shuffleOn, currentRepeat, showLyrics, miniMode, pinned]);

  return (
    <>
      <footer
        ref={surfaceRef}
        className={"player" + (miniMode ? " mini-player" : "")}
      >
        <div className="playing-info">
          <div className="playing-cover" id="playing-cover">
            {track ? (
              <img src={art(track.id, version)} alt="" />
            ) : (
              <Icon name="music" />
            )}
          </div>
          <div className="playing-text">
            <strong id="playing-title">
              {track?.title || "Find your rhythm"}
            </strong>
            <span id="playing-artist">
              {track?.artist || "Choose a song to start listening"}
            </span>
          </div>
          {track && (
            <button
              className="favourite-button"
              id="playing-favourite"
              aria-pressed={!!track.favourite}
              aria-label={
                (track.favourite ? "Remove" : "Add") +
                " favourite: " +
                track.title
              }
              title={
                track.favourite ? "Remove from favourites" : "Add to favourites"
              }
              onClick={() => onFavourite(track, !track.favourite)}
            >
              <Icon name="heart" />
            </button>
          )}
        </div>

        <div className="playback">
          <div className="playback-actions">
            <button
              className={"icon-button " + (shuffleOn ? "active" : "")}
              id="shuffle-btn"
              aria-label={`Shuffle: ${shuffleOn ? "On" : "Off"}`}
              title={`Shuffle: ${shuffleOn ? "On" : "Off"}`}
              onClick={() => setShuffleOn(!shuffleOn)}
            >
              <Icon name="shuffle" />
            </button>
            <button
              className="icon-button"
              id="previous"
              aria-label="Previous song"
              title="Previous song"
              onClick={handlePrevious}
            >
              <Icon name="previous" />
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
              <Icon name={paused ? "play" : "pause"} />
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
              <Icon name="next" />
            </button>
            <button
              className={
                "icon-button repeat-btn " +
                (currentRepeat !== "off" ? "active" : "") +
                (currentRepeat === "one" ? " repeat-one" : "")
              }
              id="repeat-btn"
              aria-label={`Repeat mode: ${currentRepeat}`}
              title={`Repeat: ${currentRepeat === "all" ? "Loop queue" : currentRepeat === "one" ? "Repeat current song" : "Off"}`}
              onClick={cycleRepeat}
            >
              <Icon name="repeat" />
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
                if (duration > 0)
                  seekTo((Number(e.target.value) / 100) * duration);
              }}
            />
            <span id="duration">{time(duration)}</span>
          </div>
        </div>

        <div className="player-extras">
          <div className="player-tools">
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
              <Icon name="lyrics" />
              <span>Lyrics</span>
            </button>

            <button
              className={"icon-button " + (showQueue ? "active" : "")}
              id="queue-btn"
              aria-label="Show play queue"
              title="Up next queue"
              onClick={() => setShowQueue(!showQueue)}
            >
              <Icon name="queue" />
              <span>Queue</span>
            </button>
          </div>

          <div className="player-secondary">
            <div className="volume">
              <button
                className="icon-button"
                id="mute-btn"
                aria-label={muted ? "Unmute" : "Mute"}
                title={muted ? "Unmute" : "Mute"}
                onClick={() => setMuted(!muted)}
              >
                <Icon name={muted || volume === 0 ? "mute" : "volume"} />
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
                <Icon name="expand" />
              </button>
              <button
                className={"icon-button " + (pinned ? "active pinned" : "")}
                id="pin-window-btn"
                aria-label={pinned ? "Unpin window" : "Pin window on top"}
                title={pinned ? "Unpin window" : "Pin window on top"}
                onClick={togglePin}
              >
                <Icon name="pin" />
              </button>
              <button
                className="icon-button"
                id="hide-window-btn"
                aria-label="Hide to menu bar"
                title="Hide to menu bar"
                onClick={hideToMenuBar}
              >
                <Icon name="chevron" className="chevron-down" />
              </button>
            </div>
          </div>
        </div>

        <audio
          id="audio"
          ref={ref}
          preload="metadata"
          onPlay={() => setPaused(false)}
          onPause={() => setPaused(true)}
          onEnded={() => {
            if (currentRepeat === "one") {
              restartCurrent();
              return;
            }
            if (currentRepeat === "off") return;
            const others = queue.filter((t) => t.id !== track?.id);
            if (
              !shuffleOn &&
              track &&
              queue.length > 0 &&
              others.length === 0
            ) {
              // Single-track queue: App state would not change, so replay here.
              restartCurrent();
              return;
            }
            handleNext();
          }}
          onLoadedMetadata={() => {
            const audio = ref.current;
            if (!audio) return;
            setDuration(audio.duration || 0);
            if (
              resumeAtRef.current > 0 &&
              Number.isFinite(audio.duration) &&
              resumeAtRef.current < audio.duration
            ) {
              audio.currentTime = resumeAtRef.current;
            }
            resumeAtRef.current = 0;
          }}
          onTimeUpdate={handleTimeUpdate}
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
            <Icon name="close" />
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
                onClick={() => choose(t)}
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
            <Icon name="close" />
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
                  onClick={() => seekTo(line.time)}
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
                  {line || " "}
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
                <Icon name="plus" />
                Add lyrics
              </button>
            </div>
          )}
        </section>
      )}
    </>
  );
}
