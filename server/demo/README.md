# MiraBase application demo

This directory contains the reproducible recording project. It uses a fresh,
isolated MiraBase process and fictional records. The existing superuser dashboard
is recorded directly. The accompanying example frontend calls that same running
backend for features without a dedicated dashboard screen; it is labeled as an
example client, not presented as a bundled administration interface.

The recording must include actual requests and their resulting rows, events,
files or receipts. No intercepted HTTP responses, fabricated results, hidden
failures, substituted application screens or pre-rendered success states are
permitted. Captions and chapter graphics are video annotations. Secrets are
redacted from the request inspector and never included in the published assets.

The chapter/feature coverage manifest records what was actually demonstrated.
Release qualification is tracked separately in `release/qualification.json`.
A successful recording is not permission to declare an unpassed release gate
qualified.

The hackathon cut is exactly three minutes, including an explicitly planned
roadmap. `story.mjs` owns narration and scene durations. Run `fixture.mjs --probe`
for the isolated server setup; `probe.mjs` exercises the connected frontend.
The recording controller uses real browser video, the real admin dashboard,
and a real external node-postgres connection with TLS verification enabled.
`pocketbase-source.mjs` creates its source backup using the pinned PocketBase
executable, in a fresh temporary directory containing fictional data.

`narrate.ps1` creates local voice tracks; `record.mjs` captures scenes;
`encode.mjs` exports the 1080p MP4, burned-in captions, separate SRT, transcript
and coverage/evidence manifest under `dist/demo`. Local recording dependencies
are configured by the workspace scripts; they are not runtime dependencies of
MiraBase. The roadmap graphic is a video asset, not a deployed cloud console.

For this Windows recording, use Node, FFmpeg/ffprobe, Chrome, Windows speech
voices and the Go version required by the root module. Install recording-only
Node dependencies with `npm install --prefix demo`. `MIRABASE_DEMO_CHROME`,
`MIRABASE_DEMO_GO` and `MIRABASE_DEMO_POCKETBASE` select local executables; the
PocketBase source executable must match the pinned audit baseline. Run commands
from the release source root. Recording creates fictional temporary data only.
