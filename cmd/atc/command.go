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
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// commandPick selects one of three strings at random.
func commandPick(opts [3]string) string {
	return opts[rand.Intn(3)]
}

// commandResponse generates an ATC response for a command channel intent.
func commandResponse(text, callsign, channelName string) string {
	lower := strings.ToLower(text)

	// Radio check / comm check
	if containsAny(lower, "radio check", "comm check", "comms check", "how copy") {
		return commandPick([3]string{
			fmt.Sprintf("%s, %s, loud and clear.", callsign, channelName),
			fmt.Sprintf("%s, %s, five by five.", callsign, channelName),
			fmt.Sprintf("%s, %s, reading you loud and clear, go ahead.", callsign, channelName),
		})
	}

	// Check-in
	if containsAny(lower, "check in", "checking in", "check-in") {
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

	// Fence out — leaving combat/training area (check before fence in/check
	// so "fence out" isn't swallowed by a looser "fence" match).
	if containsAny(lower, "fence out", "fence-out", "fenceout") {
		return commandPick([3]string{
			fmt.Sprintf("%s, %s, copy fence out, safe passage.", callsign, channelName),
			fmt.Sprintf("%s, %s, roger fence out, squawk standard, proceed home plate.", callsign, channelName),
			fmt.Sprintf("%s, %s, copy fence out, switch to departure.", callsign, channelName),
		})
	}

	// Fence in / fence check — entering combat/training area, systems hot
	if containsAny(lower, "fence in", "fence-in", "fencein", "fence check", "fence-check") {
		return commandPick([3]string{
			fmt.Sprintf("%s, %s, copy fence in, you are cleared hot.", callsign, channelName),
			fmt.Sprintf("%s, %s, roger fence in, master arm on, cleared hot, good hunting.", callsign, channelName),
			fmt.Sprintf("%s, %s, copy fence check, systems hot, you are cleared into the area.", callsign, channelName),
		})
	}

	return ""
}

// commandLoop connects to SRS on the command frequency and handles pilot calls.
func commandLoop(ctx context.Context, srsAddr string, freqMHz float64, channelName, apiKey, eamPassword, voice, externalAudioPath string) {
	guidLen := 22
	var txCooldown int64
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		log.Info().Str("addr", srsAddr).Msg("Command: connecting to SRS")
		tcpConn, err := net.DialTimeout("tcp", srsAddr, 10*time.Second)
		if err != nil {
			log.Warn().Err(err).Str("addr", srsAddr).Msg("Command: SRS connect failed, retrying in 10s")
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
			}
			continue
		}

		guid := fmt.Sprintf("vsfg7cmd%015d", time.Now().UnixNano()%1000000000000000)
		guid = guid[:guidLen]
		freqHz := freqMHz * 1e6

		srsHost, srsPort, _ := net.SplitHostPort(srsAddr)
		port, _ := strconv.Atoi(srsPort)
		udpConn, err := net.Dial("udp", fmt.Sprintf("%s:%d", srsHost, port))
		if err != nil {
			log.Warn().Err(err).Msg("Command: UDP connect failed")
			tcpConn.Close()
			time.Sleep(5 * time.Second)
			continue
		}

		tcpConn.Write(buildSync(guid, channelName, freqHz))
		time.Sleep(200 * time.Millisecond)
		tcpConn.Write(buildEAM(guid, channelName, freqHz, eamPassword))
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
						mp3, err := synthesizeSpeech(ctx, apiKey, "command test", voice)
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
					if (silent || tooLong) && len(tx.opusFrames) > 3 {
						if until := atomic.LoadInt64(&txCooldown); until > 0 && time.Now().UnixNano() < until {
							delete(transmissions, origin)
							continue
						}
						frames := tx.opusFrames
						delete(transmissions, origin)
						log.Info().Str("origin", origin).Int("frames", len(frames)).Str("channel", channelName).Msg("Command: flushing transmission to Whisper")
						go func(f [][]byte) {
							text, err := transcribeFramesWithPrompt(ctx, apiKey, f, "Command, Raider, Venom")
							if err != nil || text == "" {
								log.Info().Err(err).Str("channel", channelName).Msg("Command: empty/error transcription")
								return
							}
							if isWhisperHallucination(text) {
								log.Info().Str("text", text).Str("channel", channelName).Msg("Command: hallucination filtered")
								return
							}
							log.Info().Str("text", text).Str("channel", channelName).Msg("Command heard")
							cs := extractCallsignSimple(text)
							resp := commandResponse(text, cs, channelName)
							if resp == "" {
								log.Info().Str("text", text).Str("channel", channelName).Msg("Command intent miss")
								return
							}
							log.Info().Str("text", text).Str("callsign", cs).Str("channel", channelName).Msg("Command transcribed")
							broadcastLog("tx", fmt.Sprintf("CMD TX → %q", resp))
							mp3, err := synthesizeSpeech(ctx, apiKey, resp, voice)
							if err != nil {
								log.Error().Err(err).Msg("Command TTS failed")
								return
							}
							atomic.StoreInt64(&txCooldown, time.Now().Add(estimateTTSDuration(resp)).UnixNano())
							log.Info().Str("callsign", cs).Str("text", resp).Float64("freq", freqMHz).Msg("Command TX")
							transmitExternalAudioFile(ctx, mp3, freqMHz, channelName, srsHost, srsPort, externalAudioPath)
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