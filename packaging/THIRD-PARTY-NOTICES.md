# Third-party software in development packages

Music Studio KH uses independent bundled executables and libraries. No Navidrome, MeTube or Amperfy source was copied. Amperfy's public request definitions informed interoperability tests.

| Component | Pinned version | License/source |
|---|---|---|
| yt-dlp standalone | 2026.08.19 | GPL-3.0-or-later standalone bundle; corresponding project source archive and SHA-256 included. https://github.com/yt-dlp/yt-dlp |
| FFmpeg/ffprobe | 9.0.2, commit 946fcce07b6dcd0331c8cc609192aeff5e1924f8 | LGPL-2.1-or-later. Native source build disables GPL/nonfree components and networking. Source archive, configuration and build script included. https://ffmpeg.org |
| LAME | 3.100 | LGPL. Source archive and COPYING included. https://lame.sourceforge.io |
| Deno | 2.9.7 | MIT and third-party licenses in upstream LICENSE.md. https://github.com/denoland/deno |
| Go/Rust/npm dependencies | Exact versions in go.sum, Cargo.lock, package-lock.json | `notices/dependencies/` contains a generated manifest and available upstream license/notice files. |

The standalone yt-dlp executable contains additional bundled components; the project source archive alone does not prove all corresponding-source obligations for these components are satisfied. Before public distribution, audit the exact standalone assets and their dependency sources/notices, the statically linked FFmpeg/LAME obligations, and all compiled application dependencies. This legal/source-completeness audit remains a release gate. Development artifacts are private. macOS packages are ad-hoc signed, without Developer ID signing or notarization.

Users update these tools by installing a new integrity-checked Music Studio KH package. The app does not invoke unauthenticated self-update commands.
