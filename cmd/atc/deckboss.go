package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"math/rand"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/vsfg7/atc/pkg/composer"
	"github.com/vsfg7/atc/pkg/controller"
	"github.com/vsfg7/atc/pkg/state"
)

// deckbossLoop handles carrier deck operations (default 128.600 MHz —
// DCS carrier UHF control).
func deckbossLoop(ctx context.Context, srsAddr string, freqMHz float64, apiKey, eamPassword, voice string,
	txCooldown *int64, atcCtrl *controller.ATCController, deck *state.DeckbossState) {

	const deckCallsign = "Deckboss"
	deckVoice := voice
	if deckVoice == "" {
		deckVoice = "ash"
	}
	comp := composer.NewATCComposer(deckCallsign)

	transmit := func(text string) {
		log.Info().Str("text", text).Msg("Deckboss TX")
		atomic.StoreInt64(txCooldown, time.Now().Add(estimateTTSDuration(text)).UnixNano())
		mp3, err := synthesizeSpeech(ctx, apiKey, text, deckVoice)
		if err != nil {
			log.Error().Err(err).Msg("Deckboss TTS failed")
			return
		}
		srsHost, srsPort, _ := net.SplitHostPort(srsAddr)
		transmitExternalAudioFile(ctx, mp3, freqMHz, "OMDM-DKB", srsHost, srsPort, flagExternalAudio)
	}

	// Tacview monitor — free cat if aircraft launches without airborne call
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for _, cat := range deck.Cats {
					if cat.Callsign == "" {
						continue
					}
					// If on cat > 2 min and Tacview shows airborne — free the cat
					if time.Since(cat.UpdatedAt) < 2*time.Minute {
						continue
					}
					if atcCtrl.IsAircraftAirborne(cat.Callsign) {
						log.Info().Str("callsign", cat.Callsign).Msg("Deckboss: Tacview detected launch")
						catNum := deck.FreeCat(cat.Callsign)
						next := deck.DequeueConga()
						if next != "" {
							deck.AssignCat(next)
							go func(cs string, cn int) {
								time.Sleep(3 * time.Second)
								transmit(fmt.Sprintf("%s, %s", cs, comp.DeckbossCatClear(cn)))
							}(next, catNum)
						}
					}
				}
			}
		}
	}()

	handleDeckbossCall := func(text, callsign string) {
		lower := strings.ToLower(text)

		// Address-led guard: pilot calls for §1 / §2 / §6 always lead with
		// the word "Deckboss". Our own TX echoes back through SRS in the
		// shape "<callsign>, Deckboss, ..." (callsign-led), so anything not
		// starting with "deckboss" is treated as self-echo for those cases
		// and dropped. Without this, the §2 response ("under tension, cat
		// X, clear to launch") re-triggers §2 on echo → infinite loop.
		// §3 (silent) and §4 (airborne / clear traffic) skip the guard —
		// pilots typically don't address Deckboss on those quick calls,
		// and the response shapes can't false-fire §4 (verified: no other
		// Deckboss TX contains "airborne" or "clear traffic").
		addressed := strings.HasPrefix(lower, "deckboss") || strings.HasPrefix(lower, "deck boss")

		switch {
		case containsAny(lower, "request taxi", "ready for taxi", "green jet"):
			if !addressed {
				log.Debug().Str("text", text).Msg("Deckboss: §1 dropped — not address-led, likely self-echo")
				return
			}
			// §1 check-in: assign cat or queue in conga
			catNum := deck.AssignCat(callsign)
			if catNum > 0 {
				transmit(comp.DeckbossCatAssignment(callsign, catNum))
			} else {
				pos := deck.EnqueueConga(callsign)
				if pos == -1 {
					transmit(comp.DeckbossDeckFull(callsign))
				} else if pos == -2 {
					transmit(comp.DeckbossStandby(callsign, pos))
				} else {
					transmit(comp.DeckbossCongaLine(callsign))
				}
			}

		case containsAny(lower, "tension", "ready") && containsAny(lower, "cat"):
			if !addressed {
				log.Debug().Str("text", text).Msg("Deckboss: §2 dropped — not address-led, likely self-echo of under-tension response")
				return
			}
			// §2 under tension: pilot reports ready on cat (or shooter under tension)
			catNum := deck.GetCatByCallsign(callsign)
			if catNum > 0 {
				transmit(comp.DeckbossUnderTension(callsign, catNum))
			}

		case containsAny(lower, "tension"):
			// §3 tension-only (no cat word): pilot confirms tension — silent, they go
			log.Debug().Str("callsign", callsign).Msg("Deckboss: tension confirmed, pilot launching")

		case containsAny(lower, "airborne", "clear traffic"):
			catNum := deck.FreeCat(callsign)
			if catNum > 0 {
				log.Info().Str("callsign", callsign).Int("cat", catNum).Msg("Deckboss: cat cleared")
				next := deck.DequeueConga()
				if next != "" {
					deck.AssignCat(next)
					cn, nx := catNum, next
					go func() {
						time.Sleep(3 * time.Second)
						transmit(fmt.Sprintf("%s, %s", nx, comp.DeckbossCatClear(cn)))
					}()
				}
			}

		case containsAny(lower, "radio check", "comm check", "comms check", "how copy", "five by five", "five by"):
			if !addressed {
				log.Debug().Str("text", text).Msg("Deckboss: §6 radio check dropped — not address-led")
				return
			}
			opts := []string{
				fmt.Sprintf("%s, Deckboss, loud and clear.", callsign),
				fmt.Sprintf("%s, Deckboss, five by five.", callsign),
				fmt.Sprintf("%s, Deckboss, read you five by five.", callsign),
			}
			transmit(opts[rand.Intn(len(opts))])
		}
	}

	// SRS receive loop
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		tcpConn, err := net.DialTimeout("tcp", srsAddr, 10*time.Second)
		if err != nil {
			log.Debug().Msg("Deckboss: SRS connect failed, retrying in 10s")
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
			}
			continue
		}

		guidLen := 22
		guid := "vsfg7dkb" + fmt.Sprintf("%014d", time.Now().UnixNano()%100000000000000)
		if len(guid) > guidLen {
			guid = guid[:guidLen]
		}
		for len(guid) < guidLen {
			guid += "0"
		}

		// UDP setup — mirror Tower / Marshal / Command. The previous
		// string-form net.Dial("udp", host:port) path bound the socket via
		// Go's default resolver, which on Windows preferred IPv6 ::1; SRS
		// routes audio to the IPv4 client-list entry, so packets never
		// arrived. ResolveUDPAddr + DialUDP locks the same v4 binding the
		// other roles use. See marshal.go for the full bisect history.
		udpResolved, err := net.ResolveUDPAddr("udp", srsAddr)
		if err != nil {
			tcpConn.Close()
			time.Sleep(5 * time.Second)
			continue
		}
		udpConn, err := net.DialUDP("udp", nil, udpResolved)
		if err != nil {
			tcpConn.Close()
			time.Sleep(5 * time.Second)
			continue
		}

		freqHz := freqMHz * 1e6
		syncMsg := buildSync(guid, deckCallsign, freqHz)
		tcpConn.Write(syncMsg)
		tcpConn.Write(buildEAM(guid, deckCallsign, freqHz, eamPassword))
		// Prime the UDP NAT mapping immediately — without this, SRS doesn't
		// learn our UDP source address until the first 10s keepalive tick.
		udpConn.Write([]byte(guid))
		log.Info().Float64("freq", freqMHz).Msg("Deckboss registered on SRS")

		pingStop := make(chan struct{})
		// UDP-only keepalive — matches Tower's srsLoop. See marshal.go for the
		// full reasoning; re-sending Sync every 10s tears down audio routing.
		go func() {
			tk := time.NewTicker(10 * time.Second)
			defer tk.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-pingStop:
					return
				case <-tk.C:
					udpConn.Write([]byte(guid))
				}
			}
		}()

		// TCP reader — closes tcpDone when SRS hangs up so the inner loop
		// exits and reconnects. Without it a dropped connection leaves
		// deckbossLoop zombied with a dead socket.
		tcpDone := make(chan struct{})
		go func() {
			defer close(tcpDone)
			reader := bufio.NewReader(tcpConn)
			for {
				tcpConn.SetReadDeadline(time.Now().Add(90 * time.Second))
				line, err := reader.ReadBytes('\n')
				if err != nil {
					return
				}
				var msg map[string]interface{}
				if json.Unmarshal(line, &msg) == nil {
					if msgType, ok := msg["MsgType"].(float64); ok && int(msgType) == 1 {
						tcpConn.Write(syncMsg)
					}
				}
			}
		}()

		transmissions := make(map[string]*transmission)
		udpBuf := make([]byte, 4096)
		flushTicker := time.NewTicker(500 * time.Millisecond)

		connDone := false
		for !connDone {
			select {
			case <-ctx.Done():
				close(pingStop)
				tcpConn.Close()
				udpConn.Close()
				flushTicker.Stop()
				return
			case <-tcpDone:
				log.Warn().Msg("Deckboss: SRS disconnected — reconnecting")
				connDone = true
			case <-flushTicker.C:
				now := time.Now()
				for origin, tx := range transmissions {
					if now.Sub(tx.lastPacket) > 400*time.Millisecond && len(tx.opusFrames) > 3 {
						if until := atomic.LoadInt64(txCooldown); until > 0 && time.Now().UnixNano() < until {
							delete(transmissions, origin)
							continue
						}
						frames := tx.opusFrames
						delete(transmissions, origin)
						go func(f [][]byte) {
							text, err := transcribeFrames(ctx, apiKey, f)
							if err != nil || text == "" {
								return
							}
							if isWhisperHallucination(text) {
								return
							}
							log.Info().Str("text", text).Msg("Deckboss heard")
							cs := extractCallsignSkippingAddress(text, "deckboss", "deck boss")
							handleDeckbossCall(text, cs)
						}(frames)
					}
				}
			default:
				udpConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				n, readErr := udpConn.Read(udpBuf)
				if readErr != nil {
					continue
				}
				if n < 6 {
					continue
				}
				// audioLen lives at udpBuf[2:4]; offset 4 is freqSegLen. See marshal.go
				// for the full header layout / regression history.
				audioLen := int(binary.LittleEndian.Uint16(udpBuf[2:4]))
				if audioLen <= 0 || 6+audioLen > n {
					continue
				}
				origin := extractOriginFromUDP(udpBuf[:n])
				opusBytes := make([]byte, audioLen)
				copy(opusBytes, udpBuf[6:6+audioLen])
				if transmissions[origin] == nil {
					transmissions[origin] = &transmission{}
				}
				transmissions[origin].opusFrames = append(transmissions[origin].opusFrames, opusBytes)
				transmissions[origin].lastPacket = time.Now()
			}
		}

		flushTicker.Stop()
		close(pingStop)
		tcpConn.Close()
		udpConn.Close()
		time.Sleep(5 * time.Second)
	}
}

