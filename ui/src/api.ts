import { invoke } from "@tauri-apps/api/core";
export const isManager =
  !!(window as any).__TAURI_INTERNALS__ &&
  (location.protocol === "tauri:" ||
    location.hostname === "tauri.localhost" ||
    (import.meta.env.DEV && location.port === "1420"));
export async function api<T = any>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const response = await fetch(path, options);
  let value: any;
  try {
    value = await response.json();
  } catch {
    throw new Error("Unexpected server response");
  }
  if (!response.ok) {
    if (response.status === 401 && path !== "/api/login")
      window.dispatchEvent(new Event("session-expired"));
    throw new Error(value.error || "Request failed");
  }
  return value;
}
export const rpc = <T = any>(method: string, params: any = {}) =>
  invoke<T>("request", { method, params });
export const art = (id: string, version = 0) =>
  "/api/artwork/" + encodeURIComponent(id) + "?v=" + version;
export interface Track {
  id: string;
  title: string;
  artist: string;
  album: string;
  version?: string;
  year?: number;
  trackNumber?: number;
  folder?: string;
  format: string;
  size: number;
  duration: number;
}
export interface MetadataInput {
  title?: string;
  artist?: string;
  album?: string;
  version?: string;
  year?: number;
  trackNumber?: number;
}
export interface ImportItem {
  file: File;
  relPath?: string;
}
export interface Job {
  id: string;
  title: string;
  url: string;
  state: string;
  progress: number;
  error: string;
  parentId?: string;
  createdAt: number;
}
export interface Profile {
  id: string;
  name: string;
  library: string;
  username: string;
  port: number;
  lan: boolean;
  autoStart: boolean;
  running: boolean;
  mobileReady: boolean;
  error?: string;
  importing: boolean;
  imported: number;
  importSkipped: number;
  importError?: string;
}
export interface Appearance {
  theme: string;
  accent: string;
  density: string;
  layout: string;
}
