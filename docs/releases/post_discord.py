"""Posts a Readback ATC release's summary to the squadron Discord (#download).

publish_release.bat runs this after the release is up (operator 2026-10-25:
"is there a way with the discord update a summary can be posted at same
time?", "replace it, build the discord summary post"). It replaces GitHub's
own release post, whose webhook on the releases repo is switched off: that
post was GitHub's format, with no say over what it showed.

The post is one embed: the version, what changed (docs/releases/<version>.md,
the same notes the update offer shows) and the download links. Servers that
follow #download as an announcement channel get it too.

    python -I -X utf8 docs/releases/post_discord.py 0.2.4            post it
    python -I -X utf8 docs/releases/post_discord.py 0.2.4 --dry-run  show it

The webhook URL is %USERPROFILE%\\readback-discord-webhook.txt (or
READBACK_DISCORD_WEBHOOK). It is a secret: never printed.
"""
import argparse
import json
import os
import sys
import urllib.error
import urllib.request

REPO = "hiker1973-droid/readback-atc-releases"
HERE = os.path.dirname(os.path.abspath(__file__))
COLOR = 0x2F6FB5  # the Readback blue


def webhook():
    url = os.environ.get("READBACK_DISCORD_WEBHOOK", "").strip()
    if not url:
        path = os.path.join(os.path.expanduser("~"), "readback-discord-webhook.txt")
        try:
            with open(path, encoding="utf-8") as f:
                url = f.read().strip()
        except OSError:
            pass
    return url


def notes(version):
    """What changed: the notes file without its "# Readback ATC x.y.z" heading."""
    with open(os.path.join(HERE, version + ".md"), encoding="utf-8") as f:
        lines = f.read().replace("\r\n", "\n").split("\n")
    if lines and lines[0].startswith("#"):
        lines = lines[1:]
    text = "\n".join(lines).strip()
    if len(text) > 3500:  # an embed takes 4096; leave room for the links
        text = text[:3500].rsplit("\n", 1)[0] + "\n- ...and more in the release notes."
    return text


def payload(version):
    page = "https://github.com/%s/releases/tag/v%s" % (REPO, version)
    base = "https://github.com/%s/releases/download/v%s/" % (REPO, version)
    links = "\n".join([
        "**[Download Readback-Setup.exe](%sReadback-Setup.exe)**" % base,
        "[Installation Guide](%sReadback.ATC.Installation.Guide.pdf) · [User Manual](%sReadback.ATC.User.Manual.pdf)"
        " · [Release page](%s)" % (base, base, page),
        "",
        "Already installed? Readback offers the update by itself within a day, or press **Check now** in Settings.",
    ])
    return {
        "username": "Readback ATC",
        "allowed_mentions": {"parse": []},
        "embeds": [{
            "title": "Readback ATC %s is out" % version,
            "url": page,
            "color": COLOR,
            "description": notes(version) + "\n\n" + links,
        }],
    }


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("version")
    ap.add_argument("--dry-run", action="store_true", help="print the post instead of sending it")
    a = ap.parse_args()
    body = payload(a.version)
    if a.dry_run:
        print(json.dumps(body, indent=2, ensure_ascii=False))
        return 0
    url = webhook()
    if not url:
        print("No Discord webhook (readback-discord-webhook.txt): not posted.")
        return 1
    req = urllib.request.Request(url + "?wait=true", data=json.dumps(body).encode("utf-8"), method="POST",
                                 headers={"Content-Type": "application/json",
                                          "User-Agent": "ReadbackATC-release (https://github.com/%s, 1.0)" % REPO})
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            print("Posted the %s summary to Discord (HTTP %d)." % (a.version, r.status))
    except urllib.error.HTTPError as e:
        print("Discord refused the post: HTTP %d %s" % (e.code, e.read(300).decode("utf-8", "replace")))
        return 1
    except OSError as e:
        print("Discord post failed: %s" % e)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
