# Pilot documentation

Two generated PDFs for pilots, kept here so they version with the code they describe
(operator 2026-10-14: "Make sure we push documentation to git").

| Document | Folder | Format | Builder |
|---|---|---|---|
| **ATC Phraseology Reference** | `phraseology/` | portrait kneeboard pages | `phraseology/build_phraseology.py` |
| **SkyEye ATC Comms Packet** | `comms-packet/` | 16:9 slides | `comms-packet/_tools/build_skyeye_packet.py` |
| **Hitman JTAC Comms Flow** | `hitman-flow/` | one-page sheet (ReportLab) | `hitman-flow/build_hitman_flow.py` |
| **Paddles LSO Comms Flow** | `paddles-flow/` | one-page sheet (ReportLab) | `paddles-flow/build_paddles_flow.py` |
| **Paddles Daily Report** | `paddles-report/` | nightly PDF of traps and grades, all rigs (Task Scheduler "SkyEye Paddles Report", 00:05 on the dev box; data from the launcher's `/api/lso-log`) | `paddles-report/run_nightly.py` |

Both need the squadron card kit in
`C:\Users\afhin\Saved Games\DCS\Missions\_MISSION_TOOLKIT\briefing` (missioncard,
newpilot_kit, newpilot_slides) and PyMuPDF (`pip install pymupdf`). Each builder also
copies its PDF to the matching folder under `OneDrive\Desktop\SKYEYE ATC Dev\`.
Older versions are in git history, not in `Old\` folders.

## Updating after a SkyEye change

1. **Frequencies changed** (a start script, an airfield, an ATIS station):
   `go test ./cmd/atc -run TestDocFacts` fails. Refresh the facts both documents read:

       UPDATE_DOC_FACTS=1 go test ./cmd/atc -run TestDocFacts

2. **A call or reply changed:** edit the tables at the top of
   `phraseology/build_phraseology.py` (and the matching card in the Comms Packet builder).
3. Rebuild, from the repo root:

       python -I docs/phraseology/build_phraseology.py
       python -I docs/comms-packet/_tools/build_skyeye_packet.py
       python -I docs/paddles-flow/build_paddles_flow.py
       python -I docs/hitman-flow/build_hitman_flow.py

4. `go test ./cmd/atc -run TestDocPhrases` runs every pilot call the Phraseology
   Reference prints through SkyEye's real parsers (tower, Command, Hitman, Marshal).
   Deckboss parses inline in `deckboss.go`, so its calls are checked by hand.
5. Bump `VER` / `DATE` in the builder when the content changes, commit the PDF with the
   code, and update `comms-packet/NOTES.md` and `DISCORD_POST.md` for a packet release.

Both builders stop on any text overflow and list every overflowing row.
