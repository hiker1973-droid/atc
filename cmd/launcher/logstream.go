// Merged live-log stream.
//
// Each tower dashboard serves its own SSE feed at /ws/log, and the page used to
// open one EventSource per tower. That works for a three-tower theatre and
// quietly breaks everywhere else: a browser allows about six concurrent
// connections per origin over HTTP/1.1, and every SSE holds one open for as
// long as the page lives. Syria (8 towers + 2 carrier) or Iraq (9 + 2) saturate
// the pool, after which every ordinary fetch queues forever — the dashboard
// loads once and then sits there stale.
//
// So the fan-in happens here instead. The browser opens exactly one stream; the
// launcher holds the N upstream connections, where there is no such limit, and
// tags each event with the tower it came from.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// logStreamClient has no timeout: these connections are meant to stay open.
var logStreamClient = &http.Client{}

type mergedEvent struct {
	Src  string          `json:"src"`
	Data json.RawMessage `json:"data"`
}

// handleLogStream merges /ws/log from several towers into one SSE response.
//
//	/api/log-stream?ports=6041,6042&rig=foothold
//
// Ports are checked against the same allowlist as the tower proxy, and the rig
// against --fleet, so this cannot be pointed at an arbitrary host.
func handleLogStream(w http.ResponseWriter, r *http.Request) {
	host := "127.0.0.1"
	if name := r.URL.Query().Get("rig"); name != "" {
		rig, ok := rigByName(name)
		if !ok {
			http.Error(w, "unknown rig", http.StatusNotFound)
			return
		}
		if !isSelf(rig) {
			host = rig.Host
		}
	}

	var ports []int
	for _, p := range strings.Split(r.URL.Query().Get("ports"), ",") {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil || !proxyPorts[n] {
			http.Error(w, "tower port not allowed: "+p, http.StatusForbidden)
			return
		}
		ports = append(ports, n)
	}
	if len(ports) == 0 {
		http.Error(w, "no ports given", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx := r.Context()
	events := make(chan mergedEvent, 256)
	var wg sync.WaitGroup
	for _, port := range ports {
		wg.Add(1)
		go func(port int) {
			defer wg.Done()
			followTowerLog(ctx, host, port, events)
		}(port)
	}
	// Close the channel once every upstream has given up, so the writer below
	// ends rather than holding a dead response open.
	go func() { wg.Wait(); close(events) }()

	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		case ev, open := <-events:
			if !open {
				return
			}
			b, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
	}
}

// followTowerLog keeps one tower's SSE feed open, reconnecting while the client
// is still listening. A tower whose role is stopped simply refuses the
// connection; that is a normal state, not an error worth reporting.
func followTowerLog(ctx context.Context, host string, port int, out chan<- mergedEvent) {
	src := strconv.Itoa(port)
	for {
		if ctx.Err() != nil {
			return
		}
		readTowerLog(ctx, host, port, src, out)
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func readTowerLog(ctx context.Context, host string, port int, src string, out chan<- mergedEvent) {
	url := "http://" + net.JoinHostPort(host, strconv.Itoa(port)) + "/ws/log"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := logStreamClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 32*1024), 512*1024)
	for scanner.Scan() {
		line := scanner.Text()
		// SSE: payload lines start with "data: "; ":" lines are keepalive
		// comments and blank lines are record separators.
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok {
			continue
		}
		if !json.Valid([]byte(data)) {
			continue
		}
		select {
		case out <- mergedEvent{Src: src, Data: json.RawMessage(data)}:
		case <-ctx.Done():
			return
		default:
			// Drop rather than block the whole merge on one slow consumer.
		}
	}
}
