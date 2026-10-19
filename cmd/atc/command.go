package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/vsfg7/atc/pkg/composer"
)

// commandPick selects one of three strings at random.
func commandPick(opts [3]string) string {
	return opts[rand.Intn(3)]
}

// radioCheckPhrases are the radio-check calls Command and Hitman answer. The
// towers already took "com check" / "comcheck" / "comp check"; Command and
// Hitman missed them on DCS3 2026-10-14 (Raider 302).
var radioCheckPhrases = []string{"radio check", "comm check", "comms check", "com check", "coms check",
	"comcheck", "commcheck", "comp check", "how copy"}

// commandAddresses open a call to Command, plus the other roles' addresses so
// a pilot on the wrong radio still gets his own callsign back. Longest first:
// extractCallsignSkippingAddress takes the first exact prefix followed by a
// space or comma, so "union marshal" would not match "Union Marshall".
var commandAddresses = []string{"vsfg-7-command", "vsfg 7 command", "command",
	"union marshall", "union marshal", "unit marshall", "marshall", "marshal",
	"deck boss", "deckboss", "hitman 1-1", "hitman one one", "hitman",
	"overlord actual", "overlord"}

// commandResponse generates an ATC response for a command channel intent.
func commandResponse(text, callsign, channelName string) string {
	lower := strings.ToLower(text)

	// Radio check / comm check
	if containsAny(lower, radioCheckPhrases...) {
		return commandPick([3]string{
			fmt.Sprintf("%s, %s, loud and clear.", callsign, channelName),
			fmt.Sprintf("%s, %s, five by five.", callsign, channelName),
			fmt.Sprintf("%s, %s, reading you loud and clear, go ahead.", callsign, channelName),
		})
	}

	// Check-in. A departure handed over by the tower often says "entering
	// airspace" or "airborne" instead; Venom 021's three such calls went
	// unanswered on Day 2026-10-16 21:55 until he said "checking in".
	if containsAny(lower, "check in", "checking in", "check-in", "entering airspace", "entering the airspace",
		"entering your airspace", "airborne", "with you") {
		return commandPick([3]string{
			fmt.Sprintf("%s, %s, read you loud and clear, proceed mission.", callsign, channelName),
			fmt.Sprintf("%s, %s, loud and clear, you are cleared to proceed, good hunting.", callsign, channelName),
			fmt.Sprintf("%s, %s, copy check-in, read you five by five, proceed with mission.", callsign, channelName),
		})
	}

	// On station
	if containsAny(lower, "on station", "on-station", "onstation") {
		return commandPick([3]string{
			fmt.Sprintf("%s, %s, affirmative, good hunting.", callsign, channelName),
			fmt.Sprintf("%s, %s, copy on station, good hunting.", callsign, channelName),
			fmt.Sprintf("%s, %s, roger on station, you are cleared hot, good hunting.", callsign, channelName),
		})
	}

	// Off station
	if containsAny(lower, "off station", "off-station", "offstation", "departing station") {
		return commandPick([3]string{
			fmt.Sprintf("%s, %s, copy, proceed to assigned pattern.", callsign, channelName),
			fmt.Sprintf("%s, %s, roger off station, proceed to assigned pattern, good work.", callsign, channelName),
			fmt.Sprintf("%s, %s, copy off station, return to pattern, well done.", callsign, channelName),
		})
	}

	// RTB / recovery request — pilot is heading home and wants the handoff.
	// Checked before "fence out" so an "RTB, fence out" call reads as the
	// recovery request rather than the area-exit ack.
	//
	// Command can't name the destination here: commandResponse is a pure text
	// function with no position feed. The Tacview watcher in
	// command_handoff.go issues the real "contact <facility> on <freq>" TX
	// when the pilot closes through handoffThresholdNm, so this ack tells them
	// what to expect rather than guessing a field.
	if containsAny(lower, rtbPhrases...) {
		return commandPick([3]string{
			fmt.Sprintf("%s, %s, copy RTB, expect recovery handoff at three zero miles, continue.", callsign, channelName),
			fmt.Sprintf("%s, %s, roger returning to base, stand by for handoff inside three zero miles.", callsign, channelName),
			fmt.Sprintf("%s, %s, copy RTB, remain this frequency, I'll pass you to recovery at three zero miles.", callsign, channelName),
		})
	}

	// Fence out — leaving combat/training area (check before fence in/check
	// so "fence out" isn't swallowed by a looser "fence" match).
	if containsAny(lower, "fence out", "fence-out", "fenceout", "fencing out") {
		if fuel := extractFuelStateMarshal(lower); fuel > 0 {
			return composer.NewATCComposer(channelName).CommandFenceOut(callsign, fuel)
		}
		return commandPick([3]string{
			fmt.Sprintf("%s, %s, copy fence out, safe passage.", callsign, channelName),
			fmt.Sprintf("%s, %s, roger fence out, squawk standard, proceed home plate.", callsign, channelName),
			fmt.Sprintf("%s, %s, copy fence out, switch to departure.", callsign, channelName),
		})
	}

	// Fence in / fence check — entering combat/training area, systems hot
	// "fencing in" flown on DCS3 2026-10-15 14:18:55 and missed (pending since
	// 2026-07-02). Whisper hears "fence in" as "fence again" (Foothold
	// 2026-10-18 11:36:40 "Raider 303, fence again, air to ground", missed twice).
	if containsAny(lower, "fence in", "fence-in", "fencein", "fence check", "fence-check", "fencing in",
		"fence again", "fenced in") {
		if fuel := extractFuelStateMarshal(lower); fuel > 0 {
			return composer.NewATCComposer(channelName).CommandFenceIn(callsign, fuel)
		}
		return commandPick([3]string{
			fmt.Sprintf("%s, %s, copy fence in, you are cleared hot.", callsign, channelName),
			fmt.Sprintf("%s, %s, roger fence in, master arm on, cleared hot, good hunting.", callsign, channelName),
			fmt.Sprintf("%s, %s, copy fence check, systems hot, you are cleared into the area.", callsign, channelName),
		})
	}

	// Bare fuel state report ("Command, Raider 11, state 1.8"). Last so a state
	// given alongside any call above gets that call's reply; below 2.0 the
	// composer answers bingo.
	if fuel := extractFuelStateMarshal(lower); fuel > 0 {
		return composer.NewATCComposer(channelName).CommandFuelState(callsign, fuel)
	}

	return ""
}

// commandLoop connects to SRS on the command frequency and handles pilot calls.
// tracker may be nil; when set, every successfully-transcribed pilot callsign
// is recorded so the Tacview-driven handoff watcher knows who to monitor.
func commandLoop(ctx context.Context, srsAddr string, freqMHz float64, channelName, apiKey, eamPassword, voice, externalAudioPath string, tracker *pilotTracker, store *tacviewPositions) {
	footholdTaskingBoard() // on Syria: log now whether the Foothold save file is readable
	var inFlight *atomic.Int32
	if tracker != nil {
		inFlight = &tracker.inFlight
	}
	if flagOverlordName != "" {
		srsHost, srsPort, _ := net.SplitHostPort(srsAddr)
		commandRangeAreaBoard().overlordTX = func(text string) {
			mp3, err := synthesizeSpeech(ctx, apiKey, text, overlordVoice())
			if err != nil {
				log.Error().Err(err).Msg("Overlord TTS failed")
				return
			}
			broadcastLog("tx", fmt.Sprintf("Overlord TX → %q", text))
			log.Info().Str("text", text).Float64("freq", freqMHz).Msg("Command TX")
			noteOwnTX(freqMHz, text)
			transmitExternalAudioFile(ctx, mp3, freqMHz, flagOverlordName, srsHost, srsPort, externalAudioPath)
		}
	}
	voiceNetLoop(ctx, srsAddr, freqMHz, channelName, apiKey, eamPassword, commandVoice(voice), externalAudioPath,
		"Command, Raider, Venom, Viper, request tasking, fence in, fence out, air to ground, copy, IP Falcon, Granite, Hammer, Indigo, Javelin, "+rangeAreaWhisperWords,
		inFlight,
		func(text string) (string, string) {
			// Another role's address is skipped too: on 2026-10-15 a pilot on the
			// wrong radio called "Union Marshall, Raider 302, comcheck" on 282 and
			// was answered "Union Marshall, vSFG-7-Command, loud and clear".
			cs := joinCallsignDigits(extractCallsignSkippingAddress(text, commandAddresses...))
			if store != nil {
				cs = undoubleCallsign(cs, func(c string) bool { _, ok := store.Get(c); return ok })
			}
			if tracker != nil {
				tracker.Note(cs, text)
			}
			return cs, commandReply(text, cs, channelName, store)
		})
}

// voiceNetLoop is one controller on one SRS frequency: it hears each pilot
// transmission, has Whisper transcribe it with prompt, and speaks whatever
// handle answers (callsign, reply; "" reply = stay silent). Command and the
// JTAC both run on it. inFlight, when set, counts the transmissions between
// the flush to Whisper and the end of the reply, so the handoff watcher can
// hold off while a pilot's call is still being answered. May be nil.
func voiceNetLoop(ctx context.Context, srsAddr string, freqMHz float64, channelName, apiKey, eamPassword string, vp voiceProfile, externalAudioPath, prompt string, inFlight *atomic.Int32, handle func(text string) (string, string)) {
	guidLen := 22
	var txCooldown int64
	// Pilots already understood on this net, by SRS origin. While we transmit
	// the net used to drop everything it heard, and Paddles' groove calls
	// came every 2 s: Raider 302's ball call (Day 2026-10-18 16:08) was lost
	// inside one. ExternalAudio connects with a new origin each time, so a
	// known origin is a pilot, not us, and is heard through our own
	// transmission (the own-echo text check still applies).
	pilots := newKnownOrigins()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		log.Info().Str("addr", srsAddr).Msg("Command: connecting to SRS")
		tcpConn, err := dialSRSKeepAlive(ctx, srsAddr)
		if err != nil {
			log.Warn().Err(err).Str("addr", srsAddr).Msg("Command: SRS connect failed, retrying in 10s")
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
			}
			continue
		}

		guid := srsClientGUID("vsfg7cmd", 15)
		guid = guid[:guidLen]
		freqHz := freqMHz * 1e6

		srsHost, srsPort, _ := net.SplitHostPort(srsAddr)
		// UDP setup — mirror Tower's ResolveUDPAddr + DialUDP path. The
		// previous string-form net.Dial("udp", "localhost:5008") bound the
		// socket via Go's default resolver, which on Windows preferred IPv6
		// ::1. SRS routed audio to the IPv4 source-port and our IPv6-bound
		// socket never received voice. Same root cause that Marshal had —
		// fix verified there 2026-05-16, replicating here for Command.
		udpResolved, err := net.ResolveUDPAddr("udp", srsAddr)
		if err != nil {
			log.Warn().Err(err).Msg("Command: UDP resolve failed")
			tcpConn.Close()
			time.Sleep(5 * time.Second)
			continue
		}
		udpConn, err := net.DialUDP("udp", nil, udpResolved)
		if err != nil {
			log.Warn().Err(err).Msg("Command: UDP connect failed")
			tcpConn.Close()
			time.Sleep(5 * time.Second)
			continue
		}

		tcpConn.Write(buildSync(guid, channelName, freqHz))
		time.Sleep(200 * time.Millisecond)
		tcpConn.Write(buildEAM(guid, channelName, freqHz, eamPassword))
		// Prime the UDP NAT mapping so SRS learns our source port immediately,
		// rather than waiting 10s for the first keepalive tick. Mirrors
		// SkyEye's initialize() → SendPing() pattern. See marshal.go for the
		// full reasoning.
		udpConn.Write([]byte(guid))
		log.Info().Float64("freq", freqMHz).Str("name", channelName).Msg("Command channel registered on SRS")

		// DEBUG: with --command-test-tx, transmit "command test" every 30s to
		// verify the outbound SRS path independently of pilot audio.
		if flagCommandTestTx {
			log.Warn().Msg("Command: --command-test-tx enabled, transmitting test every 30s")
			go func() {
				tk := time.NewTicker(30 * time.Second)
				defer tk.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-tk.C:
						if until := atomic.LoadInt64(&txCooldown); until > 0 && time.Now().UnixNano() < until {
							continue
						}
						atomic.StoreInt64(&txCooldown, time.Now().Add(estimateTTSDuration("command test")).UnixNano())
						mp3, err := synthesizeSpeech(ctx, apiKey, "command test", vp)
						if err != nil {
							log.Error().Err(err).Msg("Command test TTS failed")
							continue
						}
						log.Info().Float64("freq", freqMHz).Msg("Command test TX")
						transmitExternalAudioFile(ctx, mp3, freqMHz, channelName, srsHost, srsPort, externalAudioPath)
					}
				}
			}()
		}

		log.Info().Int("goroutines", runtime.NumGoroutine()).Msg("Command: spawning keepalive goroutine")
		pingStop := make(chan struct{})
		// UDP-only keepalive — matches Tower's srsLoop. See marshal.go for the
		// full reasoning; sending Sync+EAM every 10s was tearing down audio
		// routing on each cycle.
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

		// TCP reader — closes tcpDone when the server hangs up, so the
		// inner loop below exits and we reconnect. Without this, a silently
		// dropped SRS connection leaves commandLoop zombied: the 10s keepalive
		// keeps writing to a dead socket and no voice is ever processed.
		log.Info().Int("goroutines", runtime.NumGoroutine()).Msg("Command: spawning TCP reader goroutine")
		tcpDone := make(chan struct{})
		syncMsg := buildSync(guid, channelName, freqHz)
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

		log.Info().Int("goroutines", runtime.NumGoroutine()).Msg("Command: entering main UDP read loop")
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
				log.Warn().Str("name", channelName).Msg("Command: SRS disconnected — reconnecting")
				connDone = true
			case <-flushTicker.C:
				now := time.Now()
				for origin, tx := range transmissions {
					silent := now.Sub(tx.lastPacket) > 400*time.Millisecond
					tooLong := !tx.firstPacket.IsZero() && now.Sub(tx.firstPacket) > 20*time.Second
					if (silent || tooLong) && len(tx.opusFrames) > 9 {
						if rxMuted(&txCooldown, freqMHz) && !pilots.known(origin) {
							delete(transmissions, origin)
							continue
						}
						frames := tx.opusFrames
						delete(transmissions, origin)
						log.Info().Str("origin", origin).Int("frames", len(frames)).Str("channel", channelName).Msg("Command: flushing transmission to Whisper")
						if inFlight != nil {
							inFlight.Add(1)
						}
						go func(f [][]byte, origin string) {
							if inFlight != nil {
								defer inFlight.Add(-1)
							}
							text, err := transcribeFramesWithPrompt(ctx, apiKey, f, prompt)
							if err != nil || text == "" {
								log.Info().Err(err).Str("channel", channelName).Msg("Command: empty/error transcription")
								return
							}
							if isWhisperHallucination(text) || isPromptEcho(text, prompt) {
								log.Info().Str("text", text).Str("channel", channelName).Msg("Command: hallucination filtered")
								return
							}
							if isOwnEcho(freqMHz, text) {
								log.Info().Str("text", text).Str("channel", channelName).Msg("Command: own transmission heard back - dropped")
								return
							}
							log.Info().Str("text", text).Str("channel", channelName).Msg("Command heard")
							pilots.add(origin)
							cs, resp := handle(text)
							if resp == netHandled {
								return
							}
							if resp == "" {
								log.Info().Str("text", text).Str("channel", channelName).Msg("Command intent miss")
								return
							}
							log.Info().Str("text", text).Str("callsign", cs).Str("channel", channelName).Msg("Command transcribed")
							broadcastLog("tx", fmt.Sprintf("CMD TX → %q", resp))
							mp3, err := synthesizeSpeech(ctx, apiKey, resp, vp)
							if err != nil {
								log.Error().Err(err).Msg("Command TTS failed")
								return
							}
							atomic.StoreInt64(&txCooldown, time.Now().Add(estimateTTSDuration(resp)).UnixNano())
							log.Info().Str("callsign", cs).Str("text", resp).Float64("freq", freqMHz).Msg("Command TX")
							noteOwnTX(freqMHz, resp)
							transmitExternalAudioFile(ctx, mp3, freqMHz, channelName, srsHost, srsPort, externalAudioPath)
						}(frames, origin)
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
				if transmissions[origin].firstPacket.IsZero() {
					transmissions[origin].firstPacket = time.Now()
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
