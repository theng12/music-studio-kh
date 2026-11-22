# Changelog

All notable changes to Music Studio KH are documented in this file.

## 0.3.18 — 2026-11-23

- **Several music folders in one library.** Settings → Library gains **Extra music folders**: add more folders (a native folder picker in the app, or a typed path in the browser) and their songs join the library alongside the main folder's. Extra folders are only read from — new songs, downloads and edits still land in the main folder. A folder that is unplugged keeps its songs listed until you remove it, and removing a folder never touches the files on disk or the other folders' songs.
- **Linked folders now play.** A library reached through an alias or a mounted volume scans normally, and artist folders kept as links inside the library are followed. Favourites and play counts survive a link being moved or removed.
- **The player bar stays usable on narrow windows.** The seek bar and both time labels keep their place from full width down to the phone layout.

- **Settings now shows where the app keeps its own files.** A row under the music folder shows the app's data folder — settings, artwork and conversion caches, and logs — with a Reveal in Finder button, so you can look at it or tidy it. Your music is not stored there.
- **There is no Uninstall button, on purpose.** Quitting already leaves nothing running and no login item behind; removing the app is a drag to the Trash, which you can undo.

## 0.3.16 — 2026-11-20

- **A download that stops responding no longer blocks the queue.** If YouTube stops sending data for ten minutes, the download stops by itself and says so, so the next track starts instead of the queue waiting out a four-hour limit with the bar motionless.
- **Reading a playlist has its own short limit.** Listing a playlist now gives up after three minutes and says so, instead of quietly using up the download's time before the first track begins.

## 0.3.15 — 2026-11-19

- **The installer is signed with the studio's current signing identity.** The certificate no longer carries a personal email address. Nothing else changes.

## 0.3.14 — 2026-11-19

- **The app no longer carries this Mac's folder names.** Every release build now rewrites the Rust build paths to a neutral `/home/owner`, so the installer no longer reveals the owner's username and folder layout. The release build then reads the finished DMG byte by byte and stops if any of it comes back.

## 0.3.13 — 2026-11-17

- **Only the updater can put the app on hold.** Before Studio Launcher KH replaces the app, the app now checks the updater's own credential on every hold request — a private file on this Mac that the Launcher creates and renews. Any other program, and any web page, is refused instead of being able to pause downloads. Nothing changes in normal use.

## 0.3.12 — 2026-11-16

- **A missing music folder now says so.** If the library folder was moved, deleted or is on a drive that isn't connected (such as a network share), the library names the folder, says what is wrong in plain words, and offers **Try again** and **Choose the music folder…**. It no longer looks like an empty library with 0 songs.
- **Use a music folder where it is.** An empty library now offers to point the profile at a folder you already have — on this Mac or a network drive, with any number of artist and album subfolders — instead of copying the files in. The library's folder button is now called **Copy folder**, because that is what it does.
- After choosing a new folder from the library, the library window reopens on it by itself.

## 0.3.11 — 2026-11-15

- **Updates no longer cut a download off.** Before Studio Launcher KH replaces this app, it puts the app on hold: no new download starts, the one running is left alone, and if it is still running a moment later it is marked to carry on by itself after the restart. Nothing is cancelled and there is no Retry to press.
- The hold ends by itself after two minutes, so a cancelled or failed update can never leave the app refusing new downloads.
- While a hold is in place, pasting a new link says exactly why it waits: “An update is installing, so no new download starts right now.”

## 0.3.10 — 2026-11-15

- **Check for Updates** now really checks. The app reads the signed Studio update feed, matches Music Studio KH by its own bundle id, and then says exactly what it found: still checking, an update and the version it has, “You’re on the latest version.”, or the reason the check failed. Nothing is guessed and nothing is faked.
- The same control offers a hand-off to Studio Launcher KH, which stays the only place that installs updates. The hand-off only opens from the Mac itself, never from a phone on the network.

## 0.3.9 — 2026-11-13

- Clicking the app icon, choosing **Open Music Studio KH** from the app menu, or using the menu-bar item brings the window back after it was closed to the tray. Before this, a closed window stayed hidden until the app was quit.
- Adds a hidden `--hide-window` flag used by the release test on the test Mac to start in the closed-to-tray state; it changes nothing in normal use.


## 0.3.7 — 2026-11-13

- The app no longer contacts any credential store at all — not even once. Keys live only in the app's own `secrets.json` (0600, atomic). Profiles from earlier releases start without a key: enter the password once in Settings and phone sign-in works again.
- Removing a profile and changing its folder ask in the app's own dialog with a Keep button (from 0.3.6).

## 0.3.6 — 2026-11-13

- Removing a profile and changing its music folder now ask in the app's own dialog with a clear Keep button, instead of the system's browser-style prompt that some windows never showed.

## 0.3.5 — 2026-11-13

- Phone-app sign-in keys now live in one `secrets.json` file inside the app's own data folder (mode 0600, written atomically) instead of the macOS Keychain — no Keychain reads, writes or prompts. Existing profiles keep working: their key is imported once on first launch.
- "Create a music profile" sits in Settings, the window opens on Settings, and the separate Users screen stays gone.

## 0.3.4 — 2026-11-13

- Profile creation now lives in Settings: the separate Users screen is gone, the window opens on Settings with your music profiles, and "New profile" opens the creation form right there.
- Escape now closes every open overlay: the create-profile form, What's New, album tracks, the sign-out confirm, the artwork editor, the queue and lyrics panels — and it clears the library search.
- Reopening from the menu-bar icon or the application menu always brings the window back; if the window is ever missing it is rebuilt instead of doing nothing.

## 0.3.3 — 2026-11-12

- Failed downloads are readable now: the reason and the "Download failed" tag were too faint. Messages appear bottom-right, just above the player, instead of covering the top of the page.
- A song's cover shows its play button without hovering, the search hint fits its field, and a song's details line ends cleanly instead of with a cut-off folder path.
- In the up-next queue, the song that is playing is marked and announced, and hovering a different song no longer looks like it is playing.
- "Copy into library" has a full-width lyrics box, a proper "Choose cover artwork…" button, closes with Escape and keeps keyboard focus inside. Field hints are easier to read.

## 0.3.2 — 2026-11-12

- The player bar has room now: the title block sits beside the seek bar instead of under it, and each side shows one time value, so nothing overlaps.
- Create a music profile has proper controls: a clear choice between the default location and your own folder, a real checkbox for phone listening, and a styled primary button.
- "Read in place" and "Copy into library" are named for what they do: pointing a profile at a folder streams from it and copies nothing, and the import action says it copies and refuses a source already inside the library.

## 0.3.1 — 2026-11-12

- **Quit Music Studio KH** is now a visible item in the application menu with ⌘Q, and the menu-bar menu uses the same name, so the app and its background service stop without Activity Monitor. Closing the window still keeps music available in the tray.
- Settings → Appearance offers System, Light and Dark only. The Default option is gone, and anyone who had chosen it starts on System.
- Same version, rebuilt with the appearance change: this 0.3.1 supersedes the earlier 0.3.1 build, which offered a fourth “Default” choice.
- The Users screen and the sign-in screen lost their small uppercase kickers, keyboard focus is visible on form fields, and the sign-in headline keeps its words apart in a narrow window.

## 0.3.0 — 2026-11-12

- Profiles now open as tabs in one window, with a Users tab for the profile list, switching profiles, adding one and removing one. No separate player windows.
- The menu-bar menu is a short quick menu: Open, Play/Pause, Next, Previous and Quit.
- Create a profile with the default music folder (`~/Music/Music Studio KH`) or choose your own; your choice is remembered and shown on the profile card. Existing profiles keep their folder, and nothing is moved.
- New sidebar footer: Support this app, the Music Studio KH version pill (opens What’s New) and Check for Updates, with an icon on every sidebar entry. Check for Updates reports that this app is not published to the shared updater’s feed yet; it will switch on when it is.
- Support page: the app is free with every feature unlocked. A one-time donation opens the shared donation page in your browser; it is optional and nothing depends on it.
- Theme now follows your Mac by default, with System, Light, Dark and Default in Settings → Appearance. An old dark default moves to System once; a theme you pick is kept.
- The app itself is named Music Studio KH everywhere macOS lists it, including Activity Monitor and notifications.

## 0.2.9 — 2026-11-11

- The music server that runs in the background now appears as Music Studio KH Helper, with the app’s own name and icon, instead of a bare tool name.
- Everything else works exactly as before: your profiles, music, downloads and settings are untouched.

## 0.2.8 — 2026-11-03

- Connect the Android client to a Mac library and use the existing sign-in, library and player on phones and tablets.
- Navigate Android controls with TV remote arrows and OK; use Back to close panels or return to the connection screen.
- See the Android app version and What’s new. Music, downloads and conversion stay on your Mac; offline and background playback are not yet qualified.
- Try the unsigned Windows and Linux desktop pilots. Windows is build-only, not runtime-tested: the Music app window has not been opened there yet.

## 0.2.7 — 2026-11-03

- Listen with an evenly spaced, single-row player: controls on the left, song and progress in the middle, favourite and other options on the right.
- Control playback, seeking, volume, shuffle, repeat, lyrics, queue and fullscreen from the keyboard. Shortcuts pause while you type and are listed in Help and button tooltips.
- Use system media play, next and previous controls through Media Session, with the current song and artwork.
- Open speed, volume and window options without crowding the player; keep mini player, pinning and hide-to-menu-bar controls.

## 0.2.6 — 2026-11-03

- Keep the library heading and toolbar aligned, with simpler totals and neatly grouped quick-play hints.
- Use a more compact player with clearly spaced speed, lyrics, queue and volume controls at every window width.

## 0.2.5 — 2026-11-03

- Clear finished download records while keeping your music and active downloads safe.
- See file sizes, indexed library totals, useful song details and clearer download progress.
- Mark favourites and play your favourites, most played music or a shuffled selection.
- Find recently added music with optional New badges and real added or downloaded dates.
- Enjoy aligned vector icons and clearer sidebar and player controls with an optional opaque appearance.

## 0.2.4 — 2026-11-03

- The What's new list inside the app now shows the right release notes.
- The installer file is named Music Studio KH to match the app.

## 0.2.3 — 2026-11-03

- Find collaborations under the main artist, with featured performers listed separately.
- Keep full song credits and band names such as Simon & Garfunkel. Your music files stay unchanged.

## [0.2.2] - November 2026

### A clearer music library and Settings

- Browse Songs, Albums, Artists and Folders from the sidebar, with a compact Add music control that keeps your collection in view.
- Settings opens one category at a time, with steadier layout, readable text and clearer playback controls.
- Downloads shows the actual save folder and actionable errors. The bundled audio tool can convert cover images, fixing downloads that previously failed during artwork embedding.
- Streaming settings shows active audio requests and devices seen recently, with the activity window explained.

## [0.2.1] - November 2026

### Music Studio KH

- New name: Music Studio KH, with the version and What’s new inside the app.
- Your existing libraries, music files, profiles and passwords stay in place.
- The Mac installer includes the music server and download tools. Requires macOS 13 or later.

## [0.2.0] - October 2026

### Smart duplicate skipping, full player controls & lyrics

- Smart duplicate-skipping music & folder imports (skips existing songs automatically without creating -2 copies).
- Full music player controls: Shuffle (🔀), Repeat (Off / All / One), ±10s seek buttons, playback speed (0.75×–2×), mute toggle, and Up Next queue.
- Built-in Lyrics drawer (🎤 Lyrics) with live [mm:ss] LRC synchronized line highlighting, click-to-seek, and inline lyrics editor.
- Non-overlapping sidebar layout + compact Mini-Player mode (⧉), Pin on Top (📌), Hide to Menu Bar (🙈), and native macOS Menu Bar & Tray music controller.
- Reorganized sidebar usage navigation (Songs, Albums, Artists, Folders with live item counts, Downloads, and Settings containing Appearance & access).
- Red close button background playback vs. quit preference with prompt dialog and Settings option (keeps music playing smoothly in background or quits on demand).

## [0.1.0] - October 2026

### Your music harbor, ready at home

- Multiple independent server profiles with separate or shared music folders, ports, and logs.
- Folder-preserving imports and drag-and-drop (MP3, M4A, AAC, FLAC, Ogg, Opus, WAV) with automatic folder cover art and album/artist/track inference.
- Custom import & metadata editor: import or edit songs and albums with custom titles, artists, album names, versions/editions (Deluxe, Acoustic, Live, Remastered), release years, and cover artwork.
- Persistent YouTube song and playlist download queues powered by bundled yt-dlp, Deno, and static LGPL FFmpeg.
- Subsonic / OpenSubsonic mobile compatibility (Amperfy, Symfonium) over local Wi-Fi with OS Keychain secret encryption.
- Industry-standard Profile, Preferences & Settings hub (account credentials, streaming quality, playback repeat mode, theme, default library view, and sign-out confirmation dialog).
- Refined button spacing, modal actions, responsive layouts, version badge, and in-app What’s new release notes.
