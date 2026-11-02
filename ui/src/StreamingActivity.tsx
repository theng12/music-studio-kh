import React, { useEffect, useState } from "react";
import { api } from "./api";

interface ActivityData {
  windowSeconds: number;
  activeStreams: number;
  recentDevices: number;
  totalStreams: number;
  deviceLimit: number;
  devicesCapped: boolean;
  recentClients?: { name: string; lastSeen: string }[];
}

export default function StreamingActivity() {
  const [data, setData] = useState<ActivityData | null>(null);
  const [error, setError] = useState("");
  const [refreshing, setRefreshing] = useState(false);
  const [refresh, setRefresh] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    let pending = false;
    const load = async () => {
      if (pending) return;
      pending = true;
      setRefreshing(true);
      try {
        const result = await api<ActivityData>("/api/activity", { signal: controller.signal });
        if (!controller.signal.aborted) {
          setData(result);
          setError("");
        }
      } catch (e) {
        if (!controller.signal.aborted) setError((e as Error).message);
      } finally {
        if (!controller.signal.aborted) setRefreshing(false);
        pending = false;
      }
    };
    load();
    const timer = setInterval(load, 5000);
    return () => { controller.abort(); clearInterval(timer); };
  }, [refresh]);

  return (
    <section id="streaming-activity" aria-labelledby="activity-title">
      <div className="activity-heading">
        <h3 id="activity-title">Streaming activity</h3>
        <button type="button" className="secondary" disabled={refreshing}
          aria-label="Refresh streaming activity" onClick={() => setRefresh(value => value + 1)}>
          {refreshing ? "Refreshing…" : error && !data ? "Retry" : "Refresh"}
        </button>
      </div>
      {error && <p className="error" id="activity-error" role="alert">
        Could not refresh streaming activity: {error}{data ? ". Showing the last successful reading." : ""}
      </p>}
      {!data && !error && <p id="activity-loading" role="status">Loading streaming activity…</p>}
      {data && <>
        <dl>
          <dt>Active stream requests</dt><dd id="activity-active-streams">{data.activeStreams}</dd>
          <dt>Devices seen in last {data.windowSeconds / 60} minutes</dt>
          <dd id="activity-recent-devices">{data.recentDevices}{data.devicesCapped ? ` (tracking limit of ${data.deviceLimit} reached)` : ""}</dd>
          <dt>Stream requests since server started</dt><dd id="activity-total-streams">{data.totalStreams}</dd>
          {!!data.recentClients?.length && <>
            <dt>Recent clients{data.recentClients.length < data.recentDevices ? ` (latest ${data.recentClients.length})` : ""}</dt>
            <dd id="activity-recent-clients">{data.recentClients.map((client, index) => <div key={index}>{client.name}</div>)}</dd>
          </>}
        </dl>
        <p className="muted" id="activity-note">Counts come from authenticated browser sessions and phone-app requests, not device discovery. One device can have several sessions. Buffered audio can keep playing with no active stream request.</p>
      </>}
    </section>
  );
}
