import React from "react";
import { Appearance, api } from "./api";
export default function Settings({
  profile,
  settings,
  save,
  logout,
  notice,
}: {
  profile: any;
  settings: Appearance;
  save: (s: Appearance) => void;
  logout: () => void;
  notice: (s: string) => void;
}) {
  return (
    <section id="settings-page">
      <div className="settings-card">
        <span className="eyebrow">MAKE IT YOURS</span>
        <h2>Appearance</h2>
        {(
          [
            [
              "theme",
              "Theme",
              [
                ["dark", "Midnight"],
                ["light", "Daylight"],
                ["system", "Follow system"],
              ],
            ],
            [
              "accent",
              "Accent",
              [
                ["mint", "Harbor mint"],
                ["violet", "Evening violet"],
                ["amber", "Golden hour"],
              ],
            ],
            [
              "density",
              "Library density",
              [
                ["comfortable", "Comfortable"],
                ["compact", "Compact"],
              ],
            ],
          ] as const
        ).map(([key, label, choices]) => (
          <label key={key}>
            {label}
            <select
              id={key}
              value={settings[key]}
              onChange={(e) => save({ ...settings, [key]: e.target.value })}
            >
              {choices.map(([value, name]) => (
                <option key={value} value={value}>
                  {name}
                </option>
              ))}
            </select>
          </label>
        ))}
        <p className="muted">
          Saved to this profile. Your desktop and browser player share these
          defaults.
        </p>
      </div>
      <div className="settings-card">
        <span className="eyebrow">LISTEN AROUND THE HOUSE</span>
        <h2>Your server</h2>
        <dl>
          <dt>Profile</dt>
          <dd id="settings-profile">{profile.name}</dd>
          <dt>Username</dt>
          <dd id="settings-username">{profile.username}</dd>
          <dt>Address</dt>
          <dd id="settings-address">{location.origin}</dd>
          <dt>Access</dt>
          <dd id="settings-access">
            {profile.lan ? "Local network enabled" : "This computer only"}
          </dd>
          <dt>Music folder</dt>
          <dd id="settings-folder">{profile.library}</dd>
          {profile.lan && profile.addresses?.length > 0 && (
            <>
              <dt>Phone addresses</dt>
              <dd>
                {profile.addresses.map((address: string) => (
                  <div key={address}>{address}</div>
                ))}
              </dd>
            </>
          )}
        </dl>
        <p className="muted">
          Use a phone address, username, and password in Amperfy. Keep this
          computer awake and TuneHarbor running. Local-network mode uses HTTP on
          your trusted Wi-Fi.
        </p>
        <button className="secondary" id="settings-logout" onClick={logout}>
          Sign out of this profile
        </button>
        <button
          className="secondary"
          onClick={async () => {
            try {
              await api("/api/logout-all", { method: "POST" });
              logout();
            } catch (e) {
              notice((e as Error).message);
            }
          }}
        >
          Sign out all browser devices
        </button>
        <p className="muted">
          TuneHarbor 0.1.0 development · App and download tools update together
          through a new installer. Profiles and music are preserved. If your
          phone cannot connect, allow TuneHarbor through your computer’s
          firewall on the private network and confirm both devices use the same
          Wi-Fi.
        </p>
      </div>
    </section>
  );
}
