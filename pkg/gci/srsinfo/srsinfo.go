// Package srsinfo answers the four questions SkyEye's GCI controller and radar
// ask their SRS client: who is on the GCI frequency, how many humans, how many
// clients, and which frequencies the GCI is on. atc.exe fills it from the SRS
// server's client list (sync and update messages) on the GCI's own connection.
// Ours, not vendored: SkyEye's SRS client needs cgo (Opus).
package srsinfo

import (
	"encoding/json"
	"math"
	"strings"
	"sync"

	"github.com/martinlindhe/unit"
)

// Client is what the controller and radar use (SkyEye's *simpleradio.Client).
type Client interface {
	IsOnFrequency(name string) bool
	HumansOnFrequency() int
	ClientsOnFrequency() int
	Frequencies() []RadioFrequency
}

// RadioFrequency is one frequency the GCI is on.
type RadioFrequency struct {
	Frequency  unit.Frequency
	Modulation int
}

type radio struct {
	Freq       float64 `json:"freq"`
	Modulation int     `json:"modulation"`
	Enc        bool    `json:"enc"`
}

type client struct {
	ClientGuid string
	Name       string
	RadioInfo  struct {
		Radios []radio `json:"radios"`
	}
}

// Book is the SRS client list as the GCI's connection last saw it.
type Book struct {
	ownName string
	freqs   []RadioFrequency

	mu      sync.RWMutex
	clients map[string]client // GUID -> client
}

// NewBook watches freqsHz (AM) for the GCI. ownName is the GCI's own SRS
// name: its receiver and every ExternalAudio transmission connect under it,
// and none of them is a pilot.
func NewBook(ownName string, freqsHz ...float64) *Book {
	b := &Book{ownName: ownName, clients: map[string]client{}}
	for _, f := range freqsHz {
		b.freqs = append(b.freqs, RadioFrequency{Frequency: unit.Frequency(f) * unit.Hertz})
	}
	return b
}

// Note reads one line from the SRS TCP stream. A sync carries every client
// ("Clients"), an update one ("Client"); a client disconnect (MsgType 5) drops it.
func (b *Book) Note(line []byte) {
	var msg struct {
		MsgType int
		Client  *client
		Clients []client
	}
	if json.Unmarshal(line, &msg) != nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if msg.Clients != nil && msg.MsgType == 2 { // full sync: start over
		b.clients = map[string]client{}
	}
	put := func(c client) {
		if c.ClientGuid == "" {
			return
		}
		if old, ok := b.clients[c.ClientGuid]; ok {
			if c.Name == "" {
				c.Name = old.Name
			}
			if c.RadioInfo.Radios == nil {
				c.RadioInfo = old.RadioInfo
			}
		}
		if c.Name == b.ownName {
			delete(b.clients, c.ClientGuid)
			return
		}
		b.clients[c.ClientGuid] = c
	}
	if msg.Client != nil {
		if msg.MsgType == 5 {
			delete(b.clients, msg.Client.ClientGuid)
		} else {
			put(*msg.Client)
		}
	}
	for _, c := range msg.Clients {
		put(c)
	}
}

func (b *Book) onFrequency(c client) bool {
	for _, r := range c.RadioInfo.Radios {
		if r.Enc {
			continue
		}
		for _, f := range b.freqs {
			if math.Abs(r.Freq-f.Frequency.Hertz()) <= 500 && r.Modulation == f.Modulation {
				return true
			}
		}
	}
	return false
}

func isBot(c client) bool { return strings.HasSuffix(c.Name, "[BOT]") }

// IsOnFrequency: a client named name (the DCS pilot name, as Tacview gives it)
// is tuned to the GCI.
func (b *Book) IsOnFrequency(name string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, c := range b.clients {
		if c.Name == name && b.onFrequency(c) {
			return true
		}
	}
	return false
}

func (b *Book) count(humansOnly bool) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	n := 0
	for _, c := range b.clients {
		if b.onFrequency(c) && (!humansOnly || !isBot(c)) {
			n++
		}
	}
	return n
}

// HumansOnFrequency counts clients on the GCI frequency whose name does not end "[BOT]".
func (b *Book) HumansOnFrequency() int { return b.count(true) }

// ClientsOnFrequency counts every client on the GCI frequency.
func (b *Book) ClientsOnFrequency() int { return b.count(false) }

// Frequencies the GCI is on.
func (b *Book) Frequencies() []RadioFrequency { return b.freqs }
