package main

import "testing"

// Bat commands exactly as discoverRoles captures them from the repo's bats.
const (
	batMarshalPG     = `%~dp0atc.exe --marshal-only --airfield OMDM --marshal-freq 306.3 --marshal-voice coral --srs-addr %SRS% --tacview-addr %TACVIEW% --eam-password %EAM% --dashboard-port 6004 %MIZ_FLAG% --log-level %LOG%`
	batMarshalSyria  = `%~dp0atc.exe --marshal-only --airfield LCRA --marshal-freq 306.3 --marshal-voice coral --srs-addr %SRS% --tacview-addr %TACVIEW% --eam-password %EAM% --dashboard-port 6004 %MIZ_FLAG% --log-level %LOG%`
	batATISPG        = `%~dp0atc.exe --atis-only --srs-addr %SRS% --eam-password %EAM% %MIZ_FLAG% --log-level %LOG%`
	batATISCaucasus  = `%~dp0atc.exe --atis-only --map caucasus --srs-addr %SRS% --eam-password %EAM% %MIZ_FLAG% --log-level %LOG%`
	batCommandPG     = `%~dp0atc.exe --command-only --command-freq 282.0 --command-name vSFG-7-Command --command-voice sage --srs-addr %SRS% --eam-password %EAM% %TACVIEW_FLAG% %MIZ_FLAG% --pprof-port 7770 --log-level %LOG%`
	batCommandCauc   = `%~dp0atc.exe --command-only --map caucasus --command-freq 282.0 --command-name vSFG-7-Command --command-voice sage --srs-addr %SRS% --eam-password %EAM% %TACVIEW_FLAG% %MIZ_FLAG% --pprof-port 7770 --log-level %LOG%`
	batBatumiTower   = `%~dp0atc.exe --airfield UGSB --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice nova --dashboard-port 6011 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%`
	batKobuletiTower = `%~dp0atc.exe --airfield UG5X --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --tts-voice shimmer --dashboard-port 6012 --runway-rotation=false %MIZ_FLAG% --log-level %LOG%`
	batMarshalCauc   = `%~dp0atc.exe --marshal-only --airfield UGSB --marshal-freq 306.2 --marshal-voice coral --srs-addr %SRS% --tacview-addr %TACVIEW% --eam-password %EAM% --dashboard-port 6004 %MIZ_FLAG% --log-level %LOG%`
	batDeckbossCauc  = `%~dp0atc.exe --airfield UGSB --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --deckboss-freq 128.6 --deckboss-voice shimmer --handoff-marshal-freq 306.2 --no-atis --dashboard-port 6005 %MIZ_FLAG% --log-level %LOG%`
)

// Caucasus carrier roles share --airfield UGSB with Batumi Tower and dashboard
// ports with the PG/Syria carrier roles; only the exact flag set tells them apart.
func TestRoleMatchesProcCaucasusCarrier(t *testing.T) {
	marshalCauc := proc(1, `C:\SkyeyeATC\atc.exe --marshal-only --airfield UGSB --marshal-freq 306.2 --marshal-voice coral --srs-addr localhost:5008 --tacview-addr localhost:42676 --eam-password hunter2 --dashboard-port 6004 --log-level info`)
	deckCauc := proc(2, `C:\SkyeyeATC\atc.exe --airfield UGSB --srs-addr localhost:5008 --eam-password hunter2 --tacview-addr localhost:42676 --deckboss-freq 128.6 --deckboss-voice shimmer --handoff-marshal-freq 306.2 --no-atis --dashboard-port 6005 --log-level info`)
	batumi := proc(3, `C:\SkyeyeATC\atc.exe --airfield UGSB --srs-addr localhost:5008 --eam-password hunter2 --tacview-addr localhost:42676 --tts-voice nova --dashboard-port 6011 --runway-rotation=false --log-level info`)
	cases := []struct {
		name string
		bat  string
		p    atcProc
		want bool
	}{
		{"caucasus marshal matches its bat", batMarshalCauc, marshalCauc, true},
		{"pg marshal bat is not the caucasus marshal", batMarshalPG, marshalCauc, false},
		{"syria marshal bat is not the caucasus marshal", batMarshalSyria, marshalCauc, false},
		{"caucasus deckboss matches its bat", batDeckbossCauc, deckCauc, true},
		{"batumi tower bat is not the deckboss on the same airfield", batBatumiTower, deckCauc, false},
		{"caucasus deckboss bat is not batumi tower", batDeckbossCauc, batumi, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := roleMatchesProc(tc.bat, tc.p); got != tc.want {
				t.Fatalf("got %v, want %v (proc flags %v)", got, tc.want, tc.p.flags)
			}
		})
	}
}

// proc builds a running process the way CIM reports it: %VAR%s expanded, the
// exe path quoted, a mission path with a space in it.
func proc(pid int, cmdline string) atcProc {
	toks := splitCmdLine(cmdline)
	flags, _ := flagSet(toks[1:])
	return atcProc{PID: pid, flags: flags}
}

func TestRoleMatchesProc(t *testing.T) {
	const miz = `--miz-path "C:\Users\Administrator\Saved Games\DCS\Missions\Night Training.miz"`
	marshalPG := proc(1, `"C:\SkyeyeATC\atc.exe" --marshal-only --airfield OMDM --marshal-freq 306.3 --marshal-voice coral --srs-addr localhost:5004 --tacview-addr localhost:42676 --eam-password hunter2 --dashboard-port 6004 `+miz+` --log-level info`)
	atisCauc := proc(2, `"C:\SkyeyeATC\atc.exe" --atis-only --map caucasus --srs-addr localhost:5004 --eam-password hunter2 --log-level info`)
	cmdCauc := proc(3, `"C:\SkyeyeATC\atc.exe" --command-only --map caucasus --command-freq 282.0 --command-name vSFG-7-Command --command-voice sage --srs-addr localhost:5004 --eam-password hunter2 --tacview-addr localhost:42676 `+miz+` --pprof-port 7770 --log-level info`)
	batumi := proc(4, `C:\SkyeyeATC\atc.exe --airfield UGSB --srs-addr localhost:5004 --eam-password hunter2 --tacview-addr localhost:42676 --tts-voice nova --dashboard-port 6011 --runway-rotation=false --log-level info`)

	cases := []struct {
		name string
		bat  string
		p    atcProc
		want bool
	}{
		{"marshal matches its own bat, mission path with a space", batMarshalPG, marshalPG, true},
		{"syria marshal differs only by airfield and freq", batMarshalSyria, marshalPG, false},
		{"caucasus atis matches caucasus bat without the optional miz", batATISCaucasus, atisCauc, true},
		{"pg atis flags are a subset of caucasus atis — must not match", batATISPG, atisCauc, false},
		{"caucasus command with spliced tacview + miz", batCommandCauc, cmdCauc, true},
		{"pg command shares the pprof port but not --map", batCommandPG, cmdCauc, false},
		{"tower with --flag=value form", batBatumiTower, batumi, true},
		{"different tower on another port", batKobuletiTower, batumi, false},
		{"marshal bat never matches a tower", batMarshalPG, batumi, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := roleMatchesProc(tc.bat, tc.p); got != tc.want {
				t.Fatalf("got %v, want %v (proc flags %v)", got, tc.want, tc.p.flags)
			}
		})
	}
}

func TestParseProcJSON(t *testing.T) {
	one := parseProcJSON([]byte(`{"ProcessId":42,"CommandLine":"\"C:\\SkyeyeATC\\atc.exe\" --atis-only --map caucasus"}`))
	if len(one) != 1 || one[0].PID != 42 || one[0].flags["--map"] != "caucasus" {
		t.Fatalf("single object: %+v", one)
	}
	many := parseProcJSON([]byte(`[{"ProcessId":1,"CommandLine":"atc.exe --airfield UGSB"},{"ProcessId":2,"CommandLine":null}]`))
	if len(many) != 2 || many[0].flags["--airfield"] != "UGSB" || len(many[1].flags) != 0 {
		t.Fatalf("array: %+v", many)
	}
	if parseProcJSON([]byte("  ")) != nil {
		t.Fatal("empty output should parse to nil")
	}
	noisy := parseProcJSON([]byte("#< CLIXML\r\n<Objs Version=\"1.1.0.1\"></Objs>\r\n{\"ProcessId\":7,\"CommandLine\":\"atc.exe --marshal-only\"}\r\n"))
	if len(noisy) != 1 || noisy[0].PID != 7 {
		t.Fatalf("output with a leading progress record: %+v", noisy)
	}
}
