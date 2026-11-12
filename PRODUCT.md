# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

One person: the owner, on a Mac, listening to music they own. They also listen from
their phone, including away from home, over the network. No other audience has been
confirmed; a household sharing one machine is possible through separate profiles but
was not stated as a requirement.

## Product Purpose

Play, organise and grow a music library the owner already has on disk, and be the
app they reach for instead of a streaming service. Success means daily listening
happens here, with the files, playlists and settings the owner chose intact.

## Positioning

A streaming replacement that owns its library: files stay on the owner's disk, there
is no account and no cloud, and the same library is reachable from the owner's phone
over the network. A streaming service cannot copy that without giving up its account
model.

## Operating Context

- macOS desktop app, one window; each music profile opens as a tab in that window.
- Libraries live in ordinary folders on disk; the app reads them in place and can
  import files and download audio into them.
- A profile runs a local server; the phone reaches it over the same network (or the
  owner's private network when away from home).
- Updates arrive through the shared Studio Launcher / Update Studio feed, never a
  second updater.

## Capabilities and Constraints

- Tauri 2 shell, Go helper service named "Music Studio KH Helper", React UI served
  from the helper; one accent for primary action, selection and state.
- Multiple profiles, each with its own library folder, port, credentials and
  appearance; the folder is remembered per profile and shown in the Users screen.
- Free with every feature unlocked; an optional one-time donation is the only ask.
- Local only by default: no accounts, no cloud. Network listening is an explicit
  per-profile switch.
- Explicitly not a binding rule: the app may offer to move or reorganise files when
  the owner asks for it, but never silently.

## Brand Commitments

- Name: Music Studio KH; the background service is "Music Studio KH Helper" and no
  old project name (music-studio-kh) appears on any user-visible surface.
- Existing identity: the "harbor mint" green accent and the incumbent typography and
  layout are the app's look and are preserved unless the owner asks for a new world.

## Evidence on Hand

- No marketing claims, testimonials, benchmarks or pricing exist and none may be
  invented. The Support page states only that the app is free and fully unlocked.
- Real content is the owner's own library; the profile on this Mac currently has no
  library folder, so screens must read correctly with zero songs.

## Product Principles

1. The library belongs to the owner: files stay where they are and the app reads
   them in place.
2. No account, no cloud, no nagging; network access is the owner's explicit choice.
3. Every surface tells the truth about state — "no data" is never shown as success,
   and a paused scan says why.
4. One place per job: fewer screens, plain words, one accent.
5. Free and complete; the donation is offered once and never required.

## Accessibility & Inclusion

Body and placeholder text at 4.5:1 or better in every appearance mode; every control
has default, hover, focus, active, disabled and error states; states are never
carried by colour or icon alone.
