// Command logtail tails one or more log files and streams them as SSE
// in the same JSON format as the vSFG-7 ATC dashboard.
// Handles file rotation (DCS.log gets overwritten on each session).
//
// Usage:
//   logtail.exe --port 6010 --file "C:\path\to\dcs.log:DCS" --file "C:\path\to\skyeye.log:SkyEye"
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

var (
	flagPort  int
	flagFiles []string
)

type logEvent struct {
	Level   string `json:"level"`
	Msg     string `json:"msg"`
	Source  string `json:"source"`
	Time    string `json:"time"`
}

type broker struct {
	mu      sync.RWMutex
	clients map[chan string]struct{}
}

func newBroker() *broker {
	return &broker{clients: make(map[chan string]struct{})}
}

func (b *broker) subscribe() chan string {
	ch := make(chan string, 64)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *broker) unsubscribe(ch chan string) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
	close(ch)
}

func (b *broker) publish(msg string) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

func main() {
	root := &cobra.Command{
		Use:   "logtail",
		Short: "Tail log files and serve as SSE",
		RunE:  run,
	}
	root.Flags().IntVar(&flagPort, "port", 6010, "HTTP port to serve SSE on")
	root.Flags().StringArrayVar(&flagFiles, "file", nil, "Log file to tail in format path:Label (repeatable)")
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	if len(flagFiles) == 0 {
		return fmt.Errorf("at least one --file required")
	}

	b := newBroker()

	// Start a tailer goroutine for each file
	for _, spec := range flagFiles {
		parts := strings.SplitN(spec, ":", 2)
		path := parts[0]
		label := filepath.Base(path)
		if len(parts) == 2 && parts[1] != "" {
			label = parts[1]
		}
		go tailFile(path, label, b)
	}

	// Serve SSE
	http.HandleFunc("/ws/log", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		ch := b.subscribe()
		defer b.unsubscribe(ch)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		// Send a hello event
		hello, _ := json.Marshal(logEvent{Level: "info", Msg: "logtail connected", Source: "logtail", Time: time.Now().Format(time.RFC3339)})
		fmt.Fprintf(w, "data: %s\n\n", hello)
		flusher.Flush()

		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", msg)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})

	// Health check
	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","files":%d}`, len(flagFiles))
	})

	fmt.Printf("logtail listening on :%d — tailing %d file(s)\n", flagPort, len(flagFiles))
	for _, f := range flagFiles {
		fmt.Printf("  %s\n", f)
	}
	return http.ListenAndServe(fmt.Sprintf(":%d", flagPort), nil)
}

// tailFile follows a file, handling rotation (file truncated/recreated).
func tailFile(path, label string, b *broker) {
	var (
		file   *os.File
		reader *bufio.Reader
		offset int64
		err    error
	)

	classify := func(line string) string {
		l := strings.ToLower(line)
		if strings.Contains(l, "error") || strings.Contains(l, "err ") || strings.Contains(l, "critical") || strings.Contains(l, "fatal") {
			return "error"
		}
		if strings.Contains(l, "warn") {
			return "warn"
		}
		return "info"
	}

	publish := func(line string) {
		line = strings.TrimSpace(line)
		if line == "" {
			return
		}
		ev := logEvent{
			Level:  classify(line),
			Msg:    fmt.Sprintf("[%s] %s", label, line),
			Source: label,
			Time:   time.Now().Format(time.RFC3339),
		}
		data, _ := json.Marshal(ev)
		b.publish(string(data))
	}

	openFile := func() {
		if file != nil {
			file.Close()
			file = nil
		}
		for {
			file, err = os.Open(path)
			if err == nil {
				// Seek to end on first open so we don't replay entire log
				offset, _ = file.Seek(0, io.SeekEnd)
				reader = bufio.NewReaderSize(file, 64*1024)
				ev := logEvent{Level: "info", Msg: fmt.Sprintf("[%s] watching %s", label, filepath.Base(path)), Source: label, Time: time.Now().Format(time.RFC3339)}
				data, _ := json.Marshal(ev)
				b.publish(string(data))
				return
			}
			ev := logEvent{Level: "warn", Msg: fmt.Sprintf("[%s] waiting for %s", label, filepath.Base(path)), Source: label, Time: time.Now().Format(time.RFC3339)}
			data, _ := json.Marshal(ev)
			b.publish(string(data))
			time.Sleep(5 * time.Second)
		}
	}

	openFile()

	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			publish(line)
			offset += int64(len(line))
		}
		if err != nil {
			if err != io.EOF {
				openFile()
				continue
			}
			// Check for rotation — file replaced or truncated
			time.Sleep(250 * time.Millisecond)
			info, statErr := os.Stat(path)
			if statErr != nil {
				// File gone — wait for it
				openFile()
				continue
			}
			if info.Size() < offset {
				// File truncated/rotated
				openFile()
				continue
			}
		}
	}
}
