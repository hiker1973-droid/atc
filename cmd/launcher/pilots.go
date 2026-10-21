package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

// Pilots tab (operator 2026-10-21: "Is there a way to get players stats tab?
// Hours, what they did ... DCSServerBot is running, build the pilots tab").
//
// DCSServerBot already records every slot a player sits in (statistics:
// hop_on / hop_off, kills, deaths, takeoffs, landings, crashes, ejections),
// the mission and server it was on, and - with its greenieboard plugin - each
// carrier trap. Each launcher reads its own DCSServerBot database
// (/api/pilots-raw) and the page's launcher merges every rig's rows
// (/api/pilots), so it works whether the three servers share one database or
// keep one each: rows are de-duplicated on server + player + slot time.
//
// The database is named by SKYEYE_DCSSB_DB, a postgres URL the operator sets
// with setx on each rig (the launcher re-reads SKYEYE_* from the registry on
// every Update). A read-only database user is enough - see pilots.html.
// Columns are looked up first, so a DCSServerBot version without one of them
// reads it as zero instead of failing.

// PilotSession is one DCSServerBot statistics row: a player in one slot.
type PilotSession struct {
	Server    string    `json:"server"`
	UCID      string    `json:"ucid"`
	Name      string    `json:"name"`
	Aircraft  string    `json:"aircraft"`
	Side      int       `json:"side"`
	HopOn     time.Time `json:"hopOn"`
	HopOff    time.Time `json:"hopOff"` // zero = still in the slot
	Kills     int       `json:"kills"`
	KillsAir  int       `json:"killsAir"`
	KillsGnd  int       `json:"killsGround"`
	KillsShip int       `json:"killsShips"`
	KillsSAM  int       `json:"killsSams"`
	Deaths    int       `json:"deaths"`
	Takeoffs  int       `json:"takeoffs"`
	Landings  int       `json:"landings"`
	Crashes   int       `json:"crashes"`
	Ejections int       `json:"ejections"`
	TeamKills int       `json:"teamkills"`
}

// PilotTrap is one greenieboard row: a carrier landing graded by the LSO.
type PilotTrap struct {
	Server   string    `json:"server"`
	UCID     string    `json:"ucid"`
	Name     string    `json:"name"`
	Aircraft string    `json:"aircraft"`
	Grade    string    `json:"grade"`
	Points   float64   `json:"points"`
	Wire     int       `json:"wire"`
	Night    bool      `json:"night"`
	Time     time.Time `json:"time"`
}

// PilotsRaw is one rig's rows.
type PilotsRaw struct {
	Rig        string         `json:"rig,omitempty"`
	Configured bool           `json:"configured"`
	Err        string         `json:"err,omitempty"`
	Sessions   []PilotSession `json:"sessions"`
	Traps      []PilotTrap    `json:"traps"`
}

const pilotsDBEnv = "SKYEYE_DCSSB_DB"

// pilotRange is the time window a request asks for: ?date=YYYY-MM-DD is that
// local calendar day (the nightly report), else ?days=N back from now
// (default 30; 0 = all time). Until zero = open-ended.
type pilotRange struct {
	Since, Until time.Time
	Days         int
	Date         string
}

func pilotsRangeOf(r *http.Request) pilotRange {
	q := r.URL.Query()
	if d := q.Get("date"); d != "" {
		if day, err := time.ParseInLocation("2006-01-02", d, time.Local); err == nil {
			return pilotRange{Since: day, Until: day.AddDate(0, 0, 1), Days: 1, Date: d}
		}
	}
	days := 30
	if v := q.Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			days = n
		}
	}
	if days == 0 {
		return pilotRange{}
	}
	return pilotRange{Since: time.Now().AddDate(0, 0, -days), Days: days}
}

// query is the range as a query string for another rig's /api/pilots-raw.
func (pr pilotRange) query() string {
	if pr.Date != "" {
		return "date=" + pr.Date
	}
	return fmt.Sprintf("days=%d", pr.Days)
}

func handlePilotsRaw(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, readPilotsDB(r.Context(), pilotsRangeOf(r)))
}

// readPilotsDB reads this rig's DCSServerBot rows in the range: slots and
// traps that began inside it.
func readPilotsDB(ctx context.Context, pr pilotRange) PilotsRaw {
	since, until := pr.Since, pr.Until
	if until.IsZero() {
		until = time.Now().AddDate(1, 0, 0)
	}
	url := strings.TrimSpace(os.Getenv(pilotsDBEnv))
	if url == "" {
		return PilotsRaw{Err: pilotsDBEnv + " is not set on this rig"}
	}
	out := PilotsRaw{Configured: true}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		out.Err = "DCSServerBot database: " + redactURL(err.Error(), url)
		return out
	}
	defer conn.Close(context.Background())

	cols, err := tableColumns(ctx, conn)
	if err != nil {
		out.Err = "reading the DCSServerBot schema: " + err.Error()
		return out
	}
	if !cols["statistics"]["player_ucid"] || !cols["missions"]["server_name"] {
		out.Err = "no DCSServerBot statistics / missions tables in this database"
		return out
	}
	if out.Sessions, err = readSessions(ctx, conn, cols, since, until); err != nil {
		out.Err = "statistics: " + err.Error()
		return out
	}
	for _, t := range []string{"traps", "greenieboard"} { // greenieboard plugin: traps
		if cols[t]["player_ucid"] && cols[t]["grade"] {
			if out.Traps, err = readTraps(ctx, conn, t, cols[t], since, until); err != nil {
				out.Err = t + ": " + err.Error()
			}
			break
		}
	}
	return out
}

// redactURL keeps the database password out of an error shown on the page.
func redactURL(msg, url string) string {
	if i := strings.Index(url, "://"); i >= 0 {
		rest := url[i+3:]
		if at := strings.LastIndexByte(rest, '@'); at >= 0 {
			if c := strings.IndexByte(rest[:at], ':'); c >= 0 {
				if pw := rest[c+1 : at]; pw != "" {
					msg = strings.ReplaceAll(msg, pw, "***")
				}
			}
		}
	}
	return msg
}

func tableColumns(ctx context.Context, conn *pgx.Conn) (map[string]map[string]bool, error) {
	rows, err := conn.Query(ctx, `SELECT table_name, column_name FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name IN ('statistics','missions','players','greenieboard','traps')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := map[string]map[string]bool{}
	for rows.Next() {
		var t, c string
		if err := rows.Scan(&t, &c); err != nil {
			return nil, err
		}
		if cols[t] == nil {
			cols[t] = map[string]bool{}
		}
		cols[t][c] = true
	}
	return cols, rows.Err()
}

// colOr is "alias.col" when the column exists, else the fallback literal.
func colOr(have map[string]bool, alias, col, fallback string) string {
	if have[col] {
		return alias + "." + col
	}
	return fallback
}

func readSessions(ctx context.Context, conn *pgx.Conn, cols map[string]map[string]bool, since, until time.Time) ([]PilotSession, error) {
	st := cols["statistics"]
	n := func(c string) string { return "COALESCE(" + colOr(st, "s", c, "0") + ",0)::int" }
	name := "s.player_ucid"
	if cols["players"]["name"] {
		name = "COALESCE(p.name, s.player_ucid)"
	}
	q := `SELECT m.server_name, s.player_ucid, ` + name + `, COALESCE(` + colOr(st, "s", "slot", "''") + `,''),
		COALESCE(` + colOr(st, "s", "side", "0") + `,0)::int, s.hop_on, s.hop_off,
		` + n("kills") + `, ` + n("kills_planes") + `+` + n("kills_helicopters") + `, ` + n("kills_ground") + `,
		` + n("kills_ships") + `, ` + n("kills_sams") + `, ` + n("deaths") + `, ` + n("takeoffs") + `,
		` + n("landings") + `, ` + n("crashes") + `, ` + n("ejections") + `, ` + n("teamkills") + `
		FROM statistics s JOIN missions m ON m.id = s.mission_id`
	if cols["players"]["name"] {
		q += ` LEFT JOIN players p ON p.ucid = s.player_ucid`
	}
	q += ` WHERE s.hop_on >= $1 AND s.hop_on < $2 ORDER BY s.hop_on`
	// DCSServerBot stores UTC wall times in TIMESTAMP (no zone) columns, and
	// pgx sends a time.Time's own wall clock: hand it UTC.
	rows, err := conn.Query(ctx, q, since.UTC(), until.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PilotSession
	for rows.Next() {
		var s PilotSession
		var off *time.Time
		if err := rows.Scan(&s.Server, &s.UCID, &s.Name, &s.Aircraft, &s.Side, &s.HopOn, &off,
			&s.Kills, &s.KillsAir, &s.KillsGnd, &s.KillsShip, &s.KillsSAM, &s.Deaths, &s.Takeoffs,
			&s.Landings, &s.Crashes, &s.Ejections, &s.TeamKills); err != nil {
			return nil, err
		}
		if off != nil {
			s.HopOff = *off
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func readTraps(ctx context.Context, conn *pgx.Conn, table string, have map[string]bool, since, until time.Time) ([]PilotTrap, error) {
	tcol := colOr(have, "g", "time", "NULL::timestamp")
	server := "''"
	if have["mission_id"] {
		server = "COALESCE(m.server_name,'')"
	}
	q := `SELECT ` + server + `, g.player_ucid, COALESCE(p.name, g.player_ucid),
		COALESCE(` + colOr(have, "g", "unit_type", "''") + `,''), COALESCE(g.grade,''),
		COALESCE(` + colOr(have, "g", "points", "0") + `,0)::float8, COALESCE(` + colOr(have, "g", "wire", "0") + `,0)::int,
		COALESCE(` + colOr(have, "g", "night", "false") + `,false), ` + tcol + `
		FROM ` + table + ` g LEFT JOIN players p ON p.ucid = g.player_ucid`
	if have["mission_id"] {
		q += ` LEFT JOIN missions m ON m.id = g.mission_id`
	}
	if have["time"] {
		q += ` WHERE g.time >= $1 AND g.time < $2`
	} else {
		q += ` WHERE $1::timestamp IS NOT NULL AND $2::timestamp IS NOT NULL`
	}
	rows, err := conn.Query(ctx, q, since.UTC(), until.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PilotTrap
	for rows.Next() {
		var t PilotTrap
		var at *time.Time
		if err := rows.Scan(&t.Server, &t.UCID, &t.Name, &t.Aircraft, &t.Grade, &t.Points, &t.Wire, &t.Night, &at); err != nil {
			return nil, err
		}
		if at != nil {
			t.Time = *at
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ── Fleet merge ──────────────────────────────────────────────────────────────

// PilotStats is one pilot across every rig.
type PilotStats struct {
	UCID       string             `json:"ucid"`
	Name       string             `json:"name"`
	Callsign   string             `json:"callsign"` // "Venom 212" from "Venom 212 | Dabs"
	Hours      float64            `json:"hours"`    // in a slot (DCSServerBot hop_on..hop_off)
	Sessions   int                `json:"sessions"`
	Sorties    int                `json:"sorties"` // takeoffs
	Landings   int                `json:"landings"`
	Traps      int                `json:"traps"`
	TrapPoints float64            `json:"trapPoints"` // average greenieboard points, 0 if no traps
	Kills      int                `json:"kills"`
	KillsAir   int                `json:"killsAir"`
	KillsGnd   int                `json:"killsGround"`
	Deaths     int                `json:"deaths"`
	Crashes    int                `json:"crashes"`
	Ejections  int                `json:"ejections"`
	TeamKills  int                `json:"teamkills"`
	Aircraft   map[string]float64 `json:"aircraft"` // hours by type
	Servers    map[string]float64 `json:"servers"`  // hours by server
	FirstSeen  time.Time          `json:"firstSeen"`
	LastSeen   time.Time          `json:"lastSeen"`
	Online     bool               `json:"online"` // in a slot now
	Recent     []PilotSession     `json:"recent"` // newest first, up to 15
	RecentTrap []PilotTrap        `json:"recentTraps"`
}

// PilotsReport is the /api/pilots answer.
type PilotsReport struct {
	Days   int          `json:"days"`
	Date   string       `json:"date,omitempty"`
	Since  time.Time    `json:"since"`
	Until  time.Time    `json:"until"`
	At     time.Time    `json:"at"`
	Pilots []PilotStats `json:"pilots"`
	Rigs   []PilotsRaw  `json:"rigs"` // per rig: configured / error only
}

var (
	pilotsCacheMu sync.Mutex
	pilotsCache   = map[string]*PilotsReport{}
)

func handlePilots(w http.ResponseWriter, r *http.Request) {
	pr := pilotsRangeOf(r)
	pilotsCacheMu.Lock()
	defer pilotsCacheMu.Unlock()
	if c := pilotsCache[pr.query()]; c != nil && time.Since(c.At) < 60*time.Second {
		writeJSON(w, c)
		return
	}
	rep := collectPilots(r.Context(), pr)
	pilotsCache[pr.query()] = rep
	writeJSON(w, rep)
}

func collectPilots(ctx context.Context, pr pilotRange) *PilotsReport {
	parts := make([]PilotsRaw, len(fleetRigs))
	var wg sync.WaitGroup
	for i, rig := range fleetRigs {
		wg.Add(1)
		go func(i int, rig Rig) {
			defer wg.Done()
			var raw PilotsRaw
			if isSelf(rig) {
				raw = readPilotsDB(ctx, pr)
			} else {
				c := &http.Client{Timeout: 20 * time.Second}
				resp, err := c.Get(fmt.Sprintf("http://%s:%d/api/pilots-raw?%s", rig.Host, rig.Port, pr.query()))
				if err != nil {
					raw.Err = err.Error()
				} else {
					defer resp.Body.Close()
					if resp.StatusCode != http.StatusOK {
						raw.Err = fmt.Sprintf("HTTP %d (launcher older than the Pilots tab?)", resp.StatusCode)
					} else if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
						raw.Err = err.Error()
					}
				}
			}
			raw.Rig = rig.Label
			if raw.Rig == "" {
				raw.Rig = rig.Name
			}
			parts[i] = raw
		}(i, rig)
	}
	wg.Wait()
	if len(fleetRigs) == 0 { // no --fleet: this rig alone
		raw := readPilotsDB(ctx, pr)
		raw.Rig = "this rig"
		parts = append(parts, raw)
	}
	rep := mergePilots(parts, time.Now())
	rep.Days, rep.Date, rep.Since, rep.Until = pr.Days, pr.Date, pr.Since, pr.Until
	return rep
}

// mergePilots de-duplicates every rig's rows and adds them up per pilot.
func mergePilots(parts []PilotsRaw, now time.Time) *PilotsReport {
	rep := &PilotsReport{At: now}
	seen := map[string]bool{}
	by := map[string]*PilotStats{}
	get := func(ucid, name string) *PilotStats {
		p := by[ucid]
		if p == nil {
			p = &PilotStats{UCID: ucid, Name: name, Aircraft: map[string]float64{}, Servers: map[string]float64{}}
			by[ucid] = p
		}
		return p
	}
	for _, part := range parts {
		rep.Rigs = append(rep.Rigs, PilotsRaw{Rig: part.Rig, Configured: part.Configured, Err: part.Err})
		for _, s := range part.Sessions {
			key := s.Server + "|" + s.UCID + "|" + s.HopOn.UTC().Format(time.RFC3339Nano)
			if seen[key] {
				continue
			}
			seen[key] = true
			p := get(s.UCID, s.Name)
			end := s.HopOff
			if end.IsZero() {
				end = now
				if now.Sub(s.HopOn) < 12*time.Hour {
					p.Online = true
				} else {
					end = s.HopOn // never closed (server crash): count no time
				}
			}
			h := end.Sub(s.HopOn).Hours()
			if h < 0 {
				h = 0
			}
			p.Hours += h
			p.Sessions++
			if s.Aircraft != "" {
				p.Aircraft[s.Aircraft] += h
			}
			p.Servers[s.Server] += h
			p.Sorties += s.Takeoffs
			p.Landings += s.Landings
			p.Kills += s.Kills
			p.KillsAir += s.KillsAir
			p.KillsGnd += s.KillsGnd
			p.Deaths += s.Deaths
			p.Crashes += s.Crashes
			p.Ejections += s.Ejections
			p.TeamKills += s.TeamKills
			if p.FirstSeen.IsZero() || s.HopOn.Before(p.FirstSeen) {
				p.FirstSeen = s.HopOn
			}
			if end.After(p.LastSeen) {
				p.LastSeen = end
				p.Name = s.Name // the newest name wins
			}
			p.Recent = append(p.Recent, s)
		}
		for _, t := range part.Traps {
			key := "trap|" + t.Server + "|" + t.UCID + "|" + t.Time.UTC().Format(time.RFC3339Nano) + "|" + t.Grade
			if seen[key] {
				continue
			}
			seen[key] = true
			p := get(t.UCID, t.Name)
			p.TrapPoints = (p.TrapPoints*float64(p.Traps) + t.Points) / float64(p.Traps+1)
			p.Traps++
			p.RecentTrap = append(p.RecentTrap, t)
		}
	}
	for _, p := range by {
		p.Callsign = pilotCallsign(p.Name)
		sort.Slice(p.Recent, func(i, j int) bool { return p.Recent[i].HopOn.After(p.Recent[j].HopOn) })
		if len(p.Recent) > 15 {
			p.Recent = p.Recent[:15]
		}
		sort.Slice(p.RecentTrap, func(i, j int) bool { return p.RecentTrap[i].Time.After(p.RecentTrap[j].Time) })
		if len(p.RecentTrap) > 15 {
			p.RecentTrap = p.RecentTrap[:15]
		}
		rep.Pilots = append(rep.Pilots, *p)
	}
	sort.Slice(rep.Pilots, func(i, j int) bool { return rep.Pilots[i].Hours > rep.Pilots[j].Hours })
	return rep
}

// pilotCallsign is the part of a DCS name before "|": "Venom 212 | Dabs" -> "Venom 212".
func pilotCallsign(name string) string {
	if i := strings.IndexByte(name, '|'); i >= 0 {
		return strings.TrimSpace(name[:i])
	}
	return ""
}

func servePilotsUI(w http.ResponseWriter, _ *http.Request) {
	data, err := uiFS.ReadFile("pilots.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}
