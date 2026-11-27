"""Tests for hotseat_format: the same inputs as
macos/Tests/HotseatKitTests/FormatTests.swift must give the same strings.

Run: python3 -m unittest discover -s linux -p 'test_*.py'
"""
import datetime as dt
import json
import locale
import os
import stat
import tempfile
import unittest
from zoneinfo import ZoneInfo

import hotseat_format as f

SHANGHAI = ZoneInfo("Asia/Shanghai")


def ts(s):
    """ "yyyy-MM-dd HH:mm" in Asia/Shanghai, as a Unix time."""
    return int(dt.datetime.strptime(s, "%Y-%m-%d %H:%M").replace(tzinfo=SHANGHAI).timestamp())


NOW = ts("2026-10-30 10:00")


def window(pct, resets=None, reset=False):
    return {"usedPercent": pct, "resetsAt": ts(resets) if resets else 0,
            "observedAt": ts("2026-10-30 09:50"), "source": "statusline", "reset": reset}


# The fixture of macos/Tests/HotseatKitTests/Fixture.swift.
PANEL = json.loads("""
{"generatedAt": 1793328100, "current": "claude3", "recommend": "claude2",
 "desktop": {"managed": true, "active": "claude3", "running": true, "runningKnown": true, "pendingRecovery": false},
 "accounts": [
  {"account": "claude", "current": false, "fiveHour": null, "sevenDay": null,
   "limit": {"message": "You've hit your weekly limit", "resetsAt": 1793527200, "observedAt": 1793253315},
   "limitActive": true, "desktop": "parked"},
  {"account": "claude2", "current": false,
   "fiveHour": {"usedPercent": 0, "resetsAt": 1793309400, "observedAt": 1793293372, "source": "statusline", "reset": true},
   "sevenDay": {"usedPercent": 64, "resetsAt": 1793732400, "observedAt": 1793293401, "source": "statusline", "reset": false},
   "limit": null, "limitActive": false, "desktop": "parked", "desktopProblem": "not logged in yet"},
  {"account": "claude3", "current": true,
   "fiveHour": {"usedPercent": 20, "resetsAt": 1793345400, "observedAt": 1793328072, "source": "statusline", "reset": false},
   "sevenDay": {"usedPercent": 79, "resetsAt": 1793617200, "observedAt": 1793328046, "source": "desktop", "reset": false},
   "limit": null, "limitActive": false, "desktop": "active"}
 ]}
""")


class FormatTests(unittest.TestCase):
    def test_when_text(self):
        self.assertEqual(f.fmt_when(ts("2026-10-30 14:30"), NOW, SHANGHAI), "14:30")
        self.assertEqual(f.fmt_when(ts("2026-11-02 19:00"), NOW, SHANGHAI), "Mon 19:00")
        self.assertEqual(f.fmt_when(ts("2026-11-09 10:00"), NOW, SHANGHAI), "Nov 9 10:00")

    def test_when_text_ignores_the_locale(self):
        # The Mac app pins en_US_POSIX; GTK sets the desktop's locale on
        # import, so a non-English LC_TIME must not change the names.
        old = locale.setlocale(locale.LC_TIME)
        for name in ("zh_CN.UTF-8", "de_DE.UTF-8", "fr_FR.UTF-8", "ja_JP.UTF-8"):
            try:
                locale.setlocale(locale.LC_TIME, name)
                break
            except locale.Error:
                continue
        else:
            self.skipTest("no non-English locale installed")
        try:
            self.assertEqual(f.fmt_when(ts("2026-11-02 19:00"), NOW, SHANGHAI), "Mon 19:00")
            self.assertEqual(f.fmt_when(ts("2026-11-09 10:00"), NOW, SHANGHAI), "Nov 9 10:00")
            self.assertEqual(f.fmt_limit({"resetsAt": ts("2026-11-01 18:00")}, NOW, SHANGHAI), "Limit until Sun 18:00")
        finally:
            locale.setlocale(locale.LC_TIME, old)

    def test_age_text(self):
        self.assertEqual(f.fmt_age(None, NOW), "no data yet")
        self.assertEqual(f.fmt_age(0, NOW), "no data yet")
        self.assertEqual(f.fmt_age(NOW - 59, NOW), "updated just now")
        self.assertEqual(f.fmt_age(NOW - 59 * 60, NOW), "updated 59 min ago")
        self.assertEqual(f.fmt_age(NOW - 47 * 3600, NOW), "updated 47 h ago")
        self.assertEqual(f.fmt_age(NOW - 3 * 86400, NOW), "updated 3 d ago")

    def test_account_age_uses_newest_window(self):
        self.assertEqual(f.observed_at(PANEL["accounts"][2]), 1793328072)
        self.assertEqual(f.observed_at(PANEL["accounts"][0]), 0)

    def test_title_text(self):
        generated = PANEL["generatedAt"]
        self.assertEqual(f.title_text(None, True, generated), "!")
        self.assertEqual(f.title_text(None, False, generated), "")
        self.assertEqual(f.title_text(PANEL, False, generated), "20%")
        self.assertEqual(f.title_text(PANEL, True, generated), "!")
        self.assertEqual(f.title_text(dict(PANEL, current="claude2"), False, generated), "0%")
        self.assertEqual(f.title_text(dict(PANEL, current="claude"), False, generated), "LIMIT")
        self.assertEqual(f.title_text({"current": "", "accounts": []}, False, generated), "")

    def test_window_value_text(self):
        v = lambda w: f.fmt_value(w, NOW, SHANGHAI)
        self.assertEqual(v(None), "?")
        self.assertEqual(v(window(0, "2026-10-30 08:00", reset=True)), "0% (reset)")
        self.assertEqual(v(window(12.4, "2026-10-30 14:30")), "12% · in 4h 30m")
        self.assertEqual(v(window(12, "2026-10-30 18:00")), "12% · in 8h")
        self.assertEqual(v(window(12, "2026-10-30 10:42")), "12% · in 42m")
        self.assertEqual(v(window(65, "2026-11-04 03:00")), "65% · Wed 03:00")
        self.assertEqual(v(window(79)), "79%")

    def test_limit_text(self):
        lim = lambda resets: {"message": "", "resetsAt": resets, "observedAt": 0}
        self.assertEqual(f.fmt_limit(lim(ts("2026-10-30 15:30")), NOW, SHANGHAI), "Limit · resets in 5h 30m")
        self.assertEqual(f.fmt_limit(lim(ts("2026-11-01 18:00")), NOW, SHANGHAI), "Limit until Sun 18:00")
        self.assertEqual(f.fmt_limit(lim(0), NOW, SHANGHAI), "Limit reached")
        self.assertEqual(f.fmt_limit(None, NOW, SHANGHAI), "Limit reached")

    def test_model_limit_text(self):
        lim = lambda resets, model: {"message": "", "resetsAt": resets, "observedAt": 0, "model": model}
        self.assertEqual(f.fmt_model_limit(lim(ts("2026-10-30 15:30"), "Fable"), NOW, SHANGHAI),
                         "Fable limit · resets in 5h 30m")
        self.assertEqual(f.fmt_model_limit(lim(ts("2026-11-01 18:00"), "Fable"), NOW, SHANGHAI),
                         "Fable limit until Sun 18:00")
        self.assertEqual(f.fmt_model_limit(lim(0, "Opus"), NOW, SHANGHAI), "Opus limit reached")
        self.assertEqual(f.fmt_model_limit({"resetsAt": 0}, NOW, SHANGHAI), "Model limit reached")

    def test_model_limit_does_not_limit_the_account(self):
        a = {"account": "work", "current": True, "limit": None, "limitActive": False,
             "modelLimit": {"message": "", "resetsAt": 2000, "observedAt": 1000, "model": "Fable"},
             "modelLimitActive": True, "desktop": "none"}
        self.assertTrue(f.is_model_limited(a, 1500))
        self.assertFalse(f.is_model_limited(a, 2500))
        self.assertFalse(f.is_limited(a, 1500))
        self.assertEqual(f.title_text({"current": "work", "accounts": [a]}, False, 1500), "")
        self.assertFalse(f.is_model_limited(dict(a, modelLimitActive=False), 1500))
        self.assertFalse(f.is_model_limited({"account": "old-cli"}, 1500))

    def test_display_names(self):
        self.assertEqual(f.display_name({"account": "claude2", "note": "Work", "email": "sam.w@example.com"}), "Work")
        self.assertEqual(f.display_name({"account": "claude2", "note": "Work"}), "Work")
        self.assertEqual(f.display_name({"account": "claude2", "email": "sam.w@example.com"}), "sam.w")
        self.assertEqual(f.display_name({"account": "claude2"}), "claude2")
        self.assertEqual(f.display_name({"account": "claude2", "note": "  ", "email": ""}), "claude2")
        # A non-blank note or email is used as typed, as on the Mac.
        self.assertEqual(f.display_name({"account": "claude2", "note": " Work "}), " Work ")

    def test_stale_window_past_reset(self):
        # staleWindowPastReset in FormatTests.swift.
        self.assertEqual(f.fmt_value(window(80, "2026-10-30 09:00"), NOW, SHANGHAI), "0% (reset)")
        self.assertEqual(f.win_percent(window(80, "2026-10-30 09:00"), NOW), 0)
        self.assertEqual(f.win_percent(window(80, "2026-10-30 11:00"), NOW), 80)
        limited = PANEL["accounts"][0]  # limit resets 1793527200
        self.assertTrue(f.is_limited(limited, 1793527100))
        self.assertFalse(f.is_limited(limited, 1793527300))
        later = 1793345500  # after claude3's 5h reset
        self.assertEqual(f.title_text(PANEL, False, later), "0%")

    def test_a_window_past_its_reset_is_not_used_or_limited(self):
        limited = PANEL["accounts"][0]  # limit resets 1793527200
        self.assertTrue(f.is_limited(limited, 1793527100))
        self.assertFalse(f.is_limited(limited, 1793527300))
        w = PANEL["accounts"][2]["fiveHour"]  # resets 1793345400
        self.assertEqual(f.win_percent(w, 1793345300), 20)
        self.assertEqual(f.win_percent(w, 1793345500), 0)

    def test_avatar_text(self):
        self.assertEqual(f.avatar_text({"account": "a", "note": "work"}), "W")
        self.assertEqual(f.avatar_text({"account": "a", "icon": "briefcase.fill"}), "💼")
        self.assertEqual(f.avatar_text({"account": "a", "icon": "🎯"}), "🎯")
        # An SF Symbol name with no emoji counterpart falls back to the initial.
        self.assertEqual(f.avatar_text({"account": "a", "note": "work", "icon": "tortoise.fill"}), "W")


class NamesAndEntries(unittest.TestCase):
    def test_valid_name(self):
        for ok in ("work", "claude2", "a.b_c-d", "9lives"):
            self.assertTrue(f.valid_name(ok), ok)
        for bad in ("", "-x", ".x", "a b", "a;b", "a/b", "$(x)", "a\nb"):
            self.assertFalse(f.valid_name(bad), repr(bad))

    def test_autostart_entry_quotes_the_path(self):
        plain = f.autostart_entry("/home/u/.local/bin/hotseat-indicator")
        self.assertIn("\nExec=/usr/bin/python3 -I /home/u/.local/bin/hotseat-indicator\n", plain)
        spaced = f.autostart_entry('/home/u/my apps/a\\b"c$d`e%')
        # Desktop Entry spec: %% for %; quote the argument and put \ before
        # " ` $ \; then every backslash is written twice, because the value
        # is unescaped as a string first: a literal backslash becomes four.
        self.assertIn('\nExec=/usr/bin/python3 -I "/home/u/my apps/a\\\\\\\\b\\\\"c\\\\$d\\\\`e%%"\n', spaced)
        self.assertTrue(plain.startswith("[Desktop Entry]\nType=Application\n"))

    def test_error_text(self):
        # HotseatError.message.
        self.assertEqual(f.error_text(1, "boom"), "hotseat exited with 1: boom")
        self.assertEqual(f.error_text(2, ""), "hotseat exited with 2")
        self.assertEqual(f.error_text(None, "hotseat did not answer within 30 s"), "hotseat did not answer within 30 s")
        self.assertEqual(f.bad_output("x" * 300), "hotseat printed something unexpected: " + "x" * 200)

    def test_tilde(self):
        self.assertEqual(f.tilde("/home/u/.claude-work", "/home/u"), "~/.claude-work")
        self.assertEqual(f.tilde("/home/u", "/home/u"), "~")
        self.assertEqual(f.tilde("/home/u2/x", "/home/u"), "/home/u2/x")
        self.assertEqual(f.tilde("/home/u/x", "/home/u/"), "~/x")
        self.assertEqual(f.tilde(None, "/home/u"), "—")
        self.assertEqual(f.tilde("", "/home/u"), "—")

    def test_parse_object(self):
        self.assertEqual(f.parse_object('{"a": 1}'), {"a": 1})
        for bad in ("", "nope", "[]", "1", '"s"', "null", None):
            self.assertIsNone(f.parse_object(bad), repr(bad))

    def test_removal_result(self):
        ok = json.dumps({"removed": "work", "loggedOut": True, "backup": "/b"})
        self.assertIsNone(f.removal_problem("work", ok))
        out = json.dumps({"removed": "work", "loggedOut": False, "backup": "/b/1"})
        self.assertEqual(f.removal_problem("work", out),
                         "Hotseat removed work, but Claude Code could not sign out. The account's files are in /b/1.")
        for bad in ("", "not json", json.dumps({"removed": "other", "loggedOut": True, "backup": ""}), "[]"):
            self.assertEqual(f.removal_problem("work", bad),
                             "hotseat printed something unexpected: invalid account removal result")


class SetupSteps(unittest.TestCase):
    STATUS = {"version": "0.1.1", "accounts": 2, "loginsFound": 2, "shareActions": 0, "shareConflicts": 0,
              "shim": True, "statusline": True, "hook": True, "desktop": "adopted"}

    def ids_done(self, status, version="0.1.1"):
        return [(s[0], s[2]) for s in f.setup_steps(status, version)]

    def test_all_done(self):
        self.assertEqual(self.ids_done(self.STATUS), [("cli", True), ("accounts", True), ("share", True),
                                                      ("shim", True), ("tracking", True), ("desktop", True)])

    def test_cli_missing_shows_only_the_cli_step(self):
        steps = f.setup_steps(None, None)
        self.assertEqual([(s[0], s[2]) for s in steps], [("cli", False)])
        self.assertIsNone(steps[0][4])

    def test_pending_steps_and_details(self):
        status = dict(self.STATUS, accounts=0, loginsFound=0, shim=False, hook=False, desktop="available")
        steps = {s[0]: s for s in f.setup_steps(status, "0.1.1")}
        self.assertFalse(steps["accounts"][2])
        self.assertIn("No Claude Code login found yet", steps["accounts"][3])
        self.assertFalse(steps["share"][2])  # nothing to share without accounts
        self.assertFalse(steps["shim"][2])
        self.assertFalse(steps["tracking"][2])  # needs both status line and hook
        self.assertFalse(steps["desktop"][2])
        self.assertEqual(steps["tracking"][4], [["statusline", "install"], ["hook", "install"]])
        self.assertEqual(steps["desktop"][4], [["desktop", "adopt"]])

    def test_share_conflicts_and_absent_desktop(self):
        steps = {s[0]: s for s in f.setup_steps(dict(self.STATUS, shareConflicts=3, shareActions=3, desktop="absent"), "0.1.1")}
        self.assertIn("3 files differ between accounts", steps["share"][3])
        self.assertFalse(steps["share"][2])
        self.assertTrue(steps["desktop"][2])  # nothing to adopt
        self.assertEqual(steps["desktop"][3], "Claude Desktop is not installed or never opened")

    def test_share_conflicts_do_not_block(self):
        # shareConflictsDoNotBlock in SetupTests.swift: conflicts with no
        # share actions left leave the step done, and say so.
        share = f.setup_steps(dict(self.STATUS, shareConflicts=2, shareActions=0), "0.1.1")[2]
        self.assertTrue(share[2])
        self.assertIn("2 files differ", share[3])

    def test_missing_fields_default(self):
        self.assertEqual(self.ids_done({}), [("cli", True), ("accounts", False), ("share", False),
                                             ("shim", False), ("tracking", False), ("desktop", True)])


class RunCli(unittest.TestCase):
    def fake_cli(self, body):
        d = tempfile.mkdtemp()
        path = os.path.join(d, "hotseat")
        with open(path, "w") as fh:
            fh.write("#!/bin/sh\n" + body + "\n")
        os.chmod(path, os.stat(path).st_mode | stat.S_IXUSR)
        self.addCleanup(lambda: (os.remove(path), os.rmdir(d)))
        return path

    def test_output_that_is_not_utf8_does_not_raise(self):
        path = self.fake_cli(r"printf '\377\376{}'; printf 'bad \377' >&2; exit 4")
        status, out, err = f.run_cli(path, ["panel", "--json"], 10)
        self.assertEqual(status, 4)
        self.assertIn("\ufffd", out)
        self.assertEqual(err, "bad \ufffd")
        self.assertEqual(f.error_text(status, err), "hotseat exited with 4: bad \ufffd")

    def test_exit_code_and_output(self):
        path = self.fake_cli('echo "$1 $2"; echo oops >&2; exit 3')
        self.assertEqual(f.run_cli(path, ["use", "work"], 10), (3, "use work\n", "oops"))

    def test_missing_cli_and_timeout(self):
        status, out, err = f.run_cli("/nonexistent/hotseat", ["version"], 10)
        self.assertEqual((status, out), (None, ""))
        self.assertEqual(err, "hotseat CLI not found; tried: /nonexistent/hotseat")
        slow = self.fake_cli("sleep 5")
        self.assertEqual(f.run_cli(slow, [], 0.3), (None, "", "hotseat did not answer within 0 s"))

    def test_guarded_always_returns(self):
        self.assertEqual(f.guarded(lambda: 1, lambda e: 2), 1)
        self.assertEqual(f.guarded(lambda: 1 / 0, lambda e: ("err", type(e).__name__)), ("err", "ZeroDivisionError"))


if __name__ == "__main__":
    unittest.main()
