import type { Track } from "./api";

export function shuffledQueue(tracks: Track[]): Track[] {
  const queue = [...new Map(tracks.map((track) => [track.id, track])).values()];
  for (let i = queue.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [queue[i], queue[j]] = [queue[j], queue[i]];
  }
  return queue;
}
