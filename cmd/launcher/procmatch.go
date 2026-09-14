package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"unicode/utf16"
)

// Role detection by process command line.
//
// Title matching (enumerateCmdWindows) only sees roles whose console window
// carries the bat's `start "Title"`. It misses roles started any other way, and
// a launcher running outside the operator's desktop session reads every title
// as "N/A" — which is how a rig can fly a full mission while its launcher
// reports nothing running. Every role is an atc.exe with a distinctive flag
// set, so matching running atc.exe command lines against each bat's command
// finds them however they were launched.

// atcProc is one running atc.exe and the flags on its command line.
type atcProc struct {
	PID   int
	flags map[string]string
}

var (
	procMu    sync.Mutex
	procKey   string // sorted atc.exe PIDs the cache was built for
	procCache []atcProc
)

// listATCProcs returns every running atc.exe with its flags. The CIM query goes
// through PowerShell, which is too slow to spawn on every dashboard poll, and a
// PID's command line never changes — so it only re-runs when the set of atc.exe
// PIDs (a cheap tasklist call) differs from last time.
func listATCProcs() []atcProc {
	pids := atcPIDs()
	procMu.Lock()
	defer procMu.Unlock()
	if len(pids) == 0 {
		procKey, procCache = "", nil
		return nil
	}
	key := fmt.Sprint(pids)
	if key != procKey {
		procCache = queryATCProcs()
		procKey = key
	}
	return procCache
}

// atcPIDs lists running atc.exe PIDs, sorted.
func atcPIDs() []int {
	out, err := exec.Command("tasklist", "/fi", "imagename eq atc.exe", "/fo", "csv", "/nh").Output()
	if err != nil {
		return nil
	}
	var pids []int
	r := csv.NewReader(bytes.NewReader(out))
	r.FieldsPerRecord = -1
	for {
		rec, err := r.Read()
		if err != nil {
			break
		}
		// No match prints an INFO line, which never parses as an atc.exe row.
		if len(rec) < 2 || !strings.EqualFold(rec[0], "atc.exe") {
			continue
		}
		var pid int
		if _, err := fmt.Sscanf(rec[1], "%d", &pid); err == nil {
			pids = append(pids, pid)
		}
	}
	sort.Ints(pids)
	return pids
}

// queryATCProcs reads every atc.exe command line through CIM. -EncodedCommand
// keeps the script's quotes out of Go's and PowerShell's argument parsing.
func queryATCProcs() []atcProc {
	// Progress off: a first-run PowerShell otherwise emits a "Preparing modules"
	// progress record ahead of the JSON.
	const script = `$ProgressPreference='SilentlyContinue'; Get-CimInstance Win32_Process -Filter "Name='atc.exe'" | Select-Object ProcessId,CommandLine | ConvertTo-Json -Compress`
	u := utf16.Encode([]rune(script))
	b := make([]byte, len(u)*2)
	for i, c := range u {
		binary.LittleEndian.PutUint16(b[i*2:], c)
	}
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive",
		"-EncodedCommand", base64.StdEncoding.EncodeToString(b)).Output()
	if err != nil {
		return nil
	}
	return parseProcJSON(out)
}

// parseProcJSON decodes ConvertTo-Json output, which is a bare object for one
// process and an array for several.
func parseProcJSON(out []byte) []atcProc {
	// Skip anything PowerShell printed ahead of the JSON itself.
	if i := bytes.IndexAny(out, "[{"); i >= 0 {
		out = out[i:]
	} else {
		return nil
	}
	out = bytes.TrimSpace(out)
	type row struct {
		ProcessId   int
		CommandLine string
	}
	var rows []row
	if out[0] == '{' {
		var one row
		if json.Unmarshal(out, &one) != nil {
			return nil
		}
		rows = []row{one}
	} else if json.Unmarshal(out, &rows) != nil {
		return nil
	}
	procs := make([]atcProc, 0, len(rows))
	for _, r := range rows {
		toks := splitCmdLine(r.CommandLine)
		if len(toks) > 0 {
			toks = toks[1:] // the exe path
		}
		flags, _ := flagSet(toks)
		procs = append(procs, atcProc{PID: r.ProcessId, flags: flags})
	}
	return procs
}

// splitCmdLine splits a Windows command line on whitespace, keeping
// double-quoted runs (a "Saved Games" mission path) together.
func splitCmdLine(s string) []string {
	var toks []string
	var cur strings.Builder
	inQuote, have := false, false
	for _, c := range s {
		switch {
		case c == '"':
			inQuote, have = !inQuote, true
		case (c == ' ' || c == '\t') && !inQuote:
			if have {
				toks = append(toks, cur.String())
				cur.Reset()
				have = false
			}
		default:
			cur.WriteRune(c)
			have = true
		}
	}
	if have {
		toks = append(toks, cur.String())
	}
	return toks
}

// flagSet reads "--name value", "--name=value" and bare "--switch" tokens into
// a lowercased name → value map. spliced reports a standalone %VAR% token — a
// bat's %MIZ_FLAG% or %TACVIEW_FLAG% — which expands into flags of its own.
func flagSet(toks []string) (flags map[string]string, spliced bool) {
	flags = map[string]string{}
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if !strings.HasPrefix(t, "--") {
			if strings.Contains(t, "%") {
				spliced = true
			}
			continue
		}
		name, val := t, ""
		if eq := strings.IndexByte(t, '='); eq >= 0 {
			name, val = t[:eq], t[eq+1:]
		} else if i+1 < len(toks) && !strings.HasPrefix(toks[i+1], "--") {
			val = toks[i+1]
			i++
		}
		flags[strings.ToLower(name)] = val
	}
	return flags, spliced
}

// splicable are the flags a bat adds through a standalone %VAR%, so a process
// may carry them without the bat's command naming them.
var splicable = map[string]bool{"--miz-path": true, "--tacview-addr": true}

// roleMatchesProc reports whether a running atc.exe is the one a role's bat
// command starts. The flag sets must agree exactly — every flag the bat names,
// with equal literal values (a %VAR% value matches anything), and nothing extra
// beyond what a spliced %VAR% can add. Subset matching isn't enough: the PG
// ATIS bat's flags are a subset of the Caucasus ATIS process's.
func roleMatchesProc(roleCmd string, p atcProc) bool {
	toks := splitCmdLine(roleCmd)
	if len(toks) > 0 && strings.HasSuffix(strings.ToLower(toks[0]), "atc.exe") {
		toks = toks[1:]
	}
	want, spliced := flagSet(toks)
	if len(want) == 0 {
		return false
	}
	for name, val := range want {
		got, ok := p.flags[name]
		if !ok {
			return false
		}
		if !strings.Contains(val, "%") && !strings.EqualFold(val, got) {
			return false
		}
	}
	for name := range p.flags {
		if _, ok := want[name]; !ok && !(spliced && splicable[name]) {
			return false
		}
	}
	return true
}

// detectRunning returns the PID running each role, or 0. A matching console
// window wins (its PID owns the whole tree, so Stop closes the window too);
// roles without one fall back to the atc.exe whose command line matches their
// bat, each process claimed by at most one role.
func detectRunning(rs []Role) []int {
	pids := make([]int, len(rs))
	wins := enumerateCmdWindows()
	var unmatched []int
	for i := range rs {
		if pid, ok := findWindowPID(wins, rs[i].Name); ok {
			pids[i] = pid
		} else if rs[i].cmd != "" {
			unmatched = append(unmatched, i)
		}
	}
	if len(unmatched) == 0 {
		return pids
	}
	procs := listATCProcs()
	claimed := map[int]bool{}
	for _, i := range unmatched {
		for _, p := range procs {
			if !claimed[p.PID] && roleMatchesProc(rs[i].cmd, p) {
				pids[i] = p.PID
				claimed[p.PID] = true
				break
			}
		}
	}
	return pids
}
