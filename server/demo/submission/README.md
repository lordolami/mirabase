# Submission film source

This directory owns a narrative re-edit of existing actual recordings. It does not start a server, create databases, change release qualification or replace the original film.

- `story.json`: exact 180-second allocation and narration.
- `render.py`: local Pillow/FFmpeg composition, captions, chapters and evidence manifest. Windows fonts and the existing workspace recordings/audio are required.
- `SUBMISSION.md`: reusable written submission with feature breadth, attribution, AI tooling and limitations.

Render narration using `../narrate.ps1` into the workspace `_build/submission-edit/audio` directory, then run `render.py` using Python with Pillow and FFmpeg on PATH. Outputs are under `server/dist/submission`. Visual intermediates are cached locally; invalidate affected visual files before changing their composition. The evidence manifest records source hashes and distinguishes actual dashboard footage, connected example footage and explanatory graphics.

Do not run the fixture or bulk-creation tools to re-edit this film. Do not claim exhaustive verification, production readiness, millions of populated databases, full AgentFS parity or deployed cloud features.
