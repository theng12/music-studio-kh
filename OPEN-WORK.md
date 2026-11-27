# Open work — music-studio-kh

Open tasks and known defects for this repository only; updated in the same commit whenever work starts, changes, or closes.

- [ ] **Verify start-at-login on the next logout or reboot** — The LaunchAgent plist self-heal shipped in 0.3.22; only macOS actually launching the installed app at the next user login remains unexercised. **Next:** Confirm Music Studio KH starts by itself on the next logout/login or reboot, and if it does not, check macOS background task management state.

- [ ] **Add a second music folder through the app and confirm both scan and stream** — The backend path is proven in the gate by `TestAddLibraryRootScansAndStreamsBothRoots`; what is untested is the user's own route: Settings → Extra music folders → Add a folder → native panel, then both roots scanning and streaming in the shipped app. **Next:** With the installed build open, add a second folder through Settings and confirm titles from both roots appear and play.
