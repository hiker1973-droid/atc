"""Copy SkyEye's GCI brain (github.com/dharmab/skyeye, MIT) into pkg/gci.

atc.exe runs it as the Darkstar GCI role with our own ears (SRS + Whisper API)
and mouth (ElevenLabs / OpenAI TTS, radio effect, ExternalAudio). SkyEye's own
SRS client, whisper.cpp recognizer and Piper speaker need cgo (Opus,
whisper.cpp, soxr), which the rigs' plain `go build` cannot do, so they stay
behind; the four SRS facts the controller and radar ask for come from our
SRS reader through pkg/gci/srsinfo.

Re-run to take a newer SkyEye:

    git clone https://github.com/dharmab/skyeye <dir>
    python -I tools/gci-vendor/vendor_skyeye.py <dir>      (from the repo root)

It deletes and rewrites pkg/gci (except srsinfo and conf, which are ours) and
fails loudly if an upstream edit no longer matches one of the patches below.
"""
import os
import re
import shutil
import subprocess
import sys

SRC = os.path.abspath(sys.argv[1])
ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
DST = os.path.join(ROOT, "pkg", "gci")
UP = "github.com/dharmab/skyeye"
ME = "github.com/vsfg7/atc/pkg/gci"

PKGS = ["bearings", "brevity", "callsigns", "coalitions", "composer", "controller",
        "encyclopedia", "locations", "parser", "radar", "sim", "spatial", "telemetry",
        "traces", "trackfiles"]
INTERNAL = ["normalize", "parser"]          # -> pkg/gci/internal/...
OURS = {"srsinfo", "conf", "LICENSE", "VENDORED.md"}
DROP = {os.path.join("traces", "discord.go")}  # discordgo webhook - not used

# dharmab/collections (MIT) declares go 1.26.2 for its lint tooling only; the
# Day rig builds with go 1.26.0, so the two small packages are copied too.
COLLECTIONS = ("github.com/dharmab/collections", "v1.0.0", ["sets", "deques"])

IMPORTS = [
    (COLLECTIONS[0] + "/", ME + "/internal/collections/"),
    (UP + "/internal/conf", ME + "/conf"),
    (UP + "/internal/", ME + "/internal/"),
    (UP + "/pkg/simpleradio", ME + "/srsinfo"),
    (UP + "/pkg/", ME + "/"),
]

# (file under pkg/gci, old, new): the controller and radar take our SRS facts.
PATCHES = [
    ("controller/controller.go", "srsClient *simpleradio.Client", "srsClient srsinfo.Client"),
    ("controller/controller.go", "srsClient *simpleradio.Client,", "srsClient srsinfo.Client,"),
    ("radar/radar.go", "srsClient *simpleradio.Client", "srsClient srsinfo.Client"),
    ("radar/radar.go", "srsClient *simpleradio.Client) *Radar", "srsClient srsinfo.Client) *Radar"),
    # A paused DCS server (Foothold waits for a player) refuses Tacview, and
    # SkyEye logged an ERROR every 10 s - a red alert on the dashboard all day
    # (operator 2026-10-21: "make the GCI quieter about telemetry when
    # paused"). One info line per outage, the retries at debug; a session
    # that lasted 30 s or more counts as connected and re-arms the line.
    ("telemetry/telemetry.go",
     """func (c *RealTimeClient) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			nextAttempt := time.Now().Add(10 * time.Second)
			if err := c.read(ctx); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				log.Error().Err(err).Msg("error reading telemetry, retrying")
				time.Sleep(time.Until(nextAttempt))
			}
		}
	}
}""",
     """func (c *RealTimeClient) Run(ctx context.Context) error {
	quiet := false // vsfg7: one line per outage (tools/gci-vendor/vendor_skyeye.py)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			start := time.Now()
			nextAttempt := start.Add(10 * time.Second)
			if err := c.read(ctx); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				if time.Since(start) >= 30*time.Second {
					quiet = false // it was connected: say so when it drops
				}
				if !quiet {
					log.Info().Err(err).Msg("telemetry unavailable (DCS paused or restarting?) - retrying every 10 s quietly")
					quiet = true
				} else {
					log.Debug().Err(err).Msg("error reading telemetry, retrying")
				}
				time.Sleep(time.Until(nextAttempt))
			}
		}
	}
}"""),
]


def _writable_retry(func, path, _exc):
    """rmtree helper: the collections copy came from the read-only module cache."""
    os.chmod(path, 0o777)
    parent = os.path.dirname(path)
    if parent:
        os.chmod(parent, 0o777)
    func(path)


def main():
    head = subprocess.run(["git", "-C", SRC, "log", "-1", "--format=%H %cs %s"],
                          capture_output=True, text=True, check=True).stdout.strip()
    for name in os.listdir(DST) if os.path.isdir(DST) else []:
        if name not in OURS:
            p = os.path.join(DST, name)
            shutil.rmtree(p, onexc=_writable_retry) if os.path.isdir(p) else os.remove(p)
    os.makedirs(DST, exist_ok=True)
    for p in PKGS:
        shutil.copytree(os.path.join(SRC, "pkg", p), os.path.join(DST, p))
    for p in INTERNAL:
        shutil.copytree(os.path.join(SRC, "internal", p), os.path.join(DST, "internal", p))
    for d in DROP:
        os.remove(os.path.join(DST, d))
    shutil.copy(os.path.join(SRC, "LICENSE"), os.path.join(DST, "LICENSE"))
    mod, ver, subs = COLLECTIONS
    cache = subprocess.run(["go", "env", "GOMODCACHE"], capture_output=True, text=True,
                           check=True).stdout.strip()
    csrc = os.path.join(cache, *mod.split("/")) + "@" + ver
    if not os.path.isdir(csrc):  # outside this module, so its go line is not ours
        subprocess.run(["go", "mod", "download", mod + "@" + ver], cwd=SRC, check=True,
                       env=dict(os.environ, GOTOOLCHAIN="auto"))
    for sub in subs:
        d = os.path.join(DST, "internal", "collections", sub)
        shutil.copytree(os.path.join(csrc, sub), d)
        for base, dirs, files in os.walk(d):  # the module cache is read-only
            for x in dirs:
                os.chmod(os.path.join(base, x), 0o755)
            for f in files:
                os.chmod(os.path.join(base, f), 0o644)
        os.chmod(d, 0o755)
    shutil.copy(os.path.join(csrc, "LICENSE.md"), os.path.join(DST, "internal", "collections", "LICENSE.md"))

    n = 0
    for base, _, files in os.walk(DST):
        if os.path.relpath(base, DST).split(os.sep)[0] in OURS:
            continue
        for f in files:
            if not f.endswith(".go"):
                continue
            p = os.path.join(base, f)
            s = open(p, encoding="utf-8", newline="").read()
            t = s
            for a, b in IMPORTS:
                t = t.replace('"' + a, '"' + b)
            # the package is still called simpleradio at its call sites
            t = t.replace('"' + ME + '/srsinfo"', 'srsinfo "' + ME + '/srsinfo"')
            if t != s:
                open(p, "w", encoding="utf-8", newline="").write(t)
                n += 1
    for rel, old, new in PATCHES:
        p = os.path.join(DST, rel)
        s = open(p, encoding="utf-8", newline="").read()
        if old not in s:
            if new in s:
                continue
            sys.exit("patch no longer matches %s: %r" % (rel, old))
        open(p, "w", encoding="utf-8", newline="").write(s.replace(old, new, 1))
    left = subprocess.run(["git", "grep", "-l", "simpleradio\\.", "--", "pkg/gci"], cwd=ROOT,
                          capture_output=True, text=True).stdout.split()
    left = [x for x in left if "srsinfo" not in x]
    if left:
        sys.exit("still uses simpleradio: %s" % left)
    with open(os.path.join(DST, "VENDORED.md"), "w", encoding="utf-8", newline="\n") as fh:
        fh.write("# pkg/gci - vendored SkyEye\n\n"
                 "Copied from https://github.com/dharmab/skyeye (MIT, see LICENSE) by\n"
                 "`tools/gci-vendor/vendor_skyeye.py`. Do not edit the copied packages by hand:\n"
                 "re-run the script. `srsinfo/` and `conf/` are ours.\n\n"
                 "Upstream: `%s`\n\nPackages: %s; internal/%s.\n"
                 % (head, ", ".join(PKGS), ", internal/".join(INTERNAL)))
    print("vendored", head, "-", n, "files rewritten")


if __name__ == "__main__":
    main()
