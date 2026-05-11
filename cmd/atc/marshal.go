package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/vsfg7/atc/pkg/composer"
	"github.com/vsfg7/atc/pkg/controller"
	"github.com/vsfg7/atc/pkg/state"
)

const (
	marshalCallsign     = "Marshal"
	marshalVoice        = "onyx"
	marshalTacanChannel = 72
	// Stack altitude band — Marshal assigns the lowest unoccupied angel in
	// [marshalMinAngels, marshalMaxAngels]. "Unoccupied" considers both stack
	// reservations and any Tacview contact within 50nm of the carrier.
	marshalMinAngels = 2
	marshalMaxAngels = 9
)

// marshalLoop handles the carrier marshal stack on a dedicated SRS frequency.
func marshalLoop(ctx context.Context, srsAddr string, freqMHz float64, apiKey, eamPassword string,
	txCooldown *int64, atcCtrl *controller.ATCController, stack *state.MarshalStack) {

	comp := composer.NewATCComposer(marshalCallsign)

	transmit := func(text string) {
		log.Info().Str("text", text).Msg("Marshal TX")
		atomic.StoreInt64(txCooldown, time.Now().Add(estimateTTSDuration(text)).UnixNano())
		mp3, err := synthesizeSpeech(ctx, apiKey, text, marshalVoice)
		if err != nil {
			log.Error().Err(err).Msg("Marshal TTS failed")
			return
		}
		srsHost, srsPort, _ := net.SplitHostPort(srsAddr)
		transmitExternalAudioFile(mp3, freqMHz, "OMDM-MSH", srsHost, srsPort, flagExternalAudio)
	}

	// DEBUG: with --marshal-test-tx, transmit "test" every 30s to verify the
	// outbound SRS path independently of pilot audio. Leave the flag off in
	// production — every tick blocks the cooldown window for a few seconds.
	if flagMarshalTestTx {
		log.Warn().Msg("Marshal: --marshal-test-tx enabled, transmitting test every 30s")
		go func() {
			tk := time.NewTicker(30 * time.Second)
			defer tk.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-tk.C:
					transmit("test")
				}
			}
		}()
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		tcpConn, err := net.DialTimeout("tcp", srsAddr, 10*time.Second)
		if err != nil {
			log.Debug().Msg("Marshal: SRS connect failed, retrying in 10s")
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
			}
			continue
		}

		guidLen := 22
		guid := "vsfg7msh" + fmt.Sprintf("%014d", time.Now().UnixNano()%100000000000000)
		if len(guid) > guidLen { guid = guid[:guidLen] }
		for len(guid) < guidLen { guid += "0" }
		srsHost, srsPort, _ := net.SplitHostPort(srsAddr)
		port, _ := strconv.Atoi(srsPort)
		udpAddr := fmt.Sprintf("%s:%d", srsHost, port)
		udpConn, err := net.Dial("udp", udpAddr)
		if err != nil {
			tcpConn.Close()
			time.Sleep(5 * time.Second)
			continue
		}

		freqHz := freqMHz * 1e6
		syncMsg := buildSync(guid, marshalCallsign, freqHz)
		tcpConn.Write(syncMsg)
		time.Sleep(200 * time.Millisecond)
		eamMsg := buildEAM(guid, marshalCallsign, freqHz, eamPassword)
		tcpConn.Write(eamMsg)
		log.Info().Float64("freq", freqMHz).Msg("Marshal registered on SRS")

		// UDP keepalive
		keepaliveStop := make(chan struct{})
		go func() {
			defer close(keepaliveStop)
			tk := time.NewTicker(10 * time.Second)
			defer tk.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-tk.C:
					udpConn.Write([]byte(guid))
					tcpConn.Write(buildSync(guid, marshalCallsign, freqMHz*1e6))
				}
			}
		}()

		// TCP reader — responds to server pings and detects disconnect.
		// Without this Marshal silently zombies on a dead socket and the
		// SRS server removes it from the client list after first missed
		// ping, even though the goroutine keeps writing keepalives.
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
				tcpConn.Close()
				udpConn.Close()
				flushTicker.Stop()
				return
			case <-tcpDone:
				log.Warn().Msg("Marshal: SRS disconnected — reconnecting")
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
							// Filter Whisper hallucinations — prompt echo and nonsense
							if isWhisperHallucination(text) {
								log.Debug().Str("text", text).Msg("Marshal: hallucination filtered")
								return
							}
							log.Info().Str("text", text).Msg("Marshal heard")
							cs := extractCallsignSimple(text)
							handleMarshalCall(text, cs, stack, comp, transmit, atcCtrl)
						}(frames)
					}
				}
			default:
				udpConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				n, readErr := udpConn.Read(udpBuf)
				if readErr != nil {
					continue
				}
				// 22-byte packet is a UDP ping (just the GUID), not voice.
				if n == 22 {
					continue
				}
				if n < 7 {
					continue
				}
				// SRS audio packet layout matches Tower's parser in main.go:
				//   [2:4]  audioLen (uint16 LE)
				//   [4:6]  freqSegLen (uint16 LE) — total bytes of trailing freq segments
				//   [6:6+audioLen]  opus audio
				//   [6+audioLen:][n*10]  freq segments (8B freq float64 + 2B trailer each)
				//   [n-22:n]  origin client GUID
				// Marshal previously read audioLen from [4:6] (which is freqSegLen),
				// so every packet either bailed or accumulated garbage — making
				// Marshal deaf to pilots even though TX worked.
				audioLen := binary.LittleEndian.Uint16(udpBuf[2:4])
				if int(audioLen) > n-6 {
					continue
				}
				opusBytes := make([]byte, audioLen)
				copy(opusBytes, udpBuf[6:6+audioLen])

				// SRS broadcasts packets from all radios to all clients; filter
				// to our freq the same way Tower does (500Hz tolerance).
				audioEnd := 6 + int(audioLen)
				freqSegLen := binary.LittleEndian.Uint16(udpBuf[4:6])
				freqMatch := false
				for i := 0; i+10 <= int(freqSegLen) && audioEnd+i+10 <= n; i += 10 {
					pktFreq := math.Float64frombits(binary.LittleEndian.Uint64(udpBuf[audioEnd+i : audioEnd+i+8]))
					if math.Abs(pktFreq-freqHz) < 500 {
						freqMatch = true
						break
					}
				}
				if !freqMatch {
					continue
				}

				origin := extractOriginFromUDP(udpBuf[:n])
				if transmissions[origin] == nil {
					transmissions[origin] = &transmission{}
				}
				transmissions[origin].opusFrames = append(transmissions[origin].opusFrames, opusBytes)
				transmissions[origin].lastPacket = time.Now()
			}
		}

		flushTicker.Stop()
		tcpConn.Close()
		udpConn.Close()
		log.Debug().Msg("Marshal: SRS disconnected, reconnecting in 5s")
		time.Sleep(5 * time.Second)
	}
}

// handleMarshalCall processes a recognized marshal transmission.
func handleMarshalCall(text, callsign string, stack *state.MarshalStack, comp *composer.ATCComposer, transmit func(string), atcCtrl *controller.ATCController) {
	lower := strings.ToLower(text)
	fuelState := extractFuelStateMarshal(lower)
	ceilingFt, altimeter := atcCtrl.GetWeatherState()
	switch {
	case containsAny(lower, "marking mom", "marking moms"):
		pos, _ := stack.Enqueue(callsign, fuelState)
		reserved := stack.ReservedAngels(callsign)
		angels := atcCtrl.AssignMarshalAngels(marshalMinAngels, marshalMaxAngels, reserved)
		stack.SetAngels(callsign, angels)
		brc := atcCtrl.GetCarrierBRC()
		log.Info().Str("callsign", callsign).Int("position", pos).Int("angels", angels).Ints("reserved", reserved).Float64("brc", brc).Msg("Marshal: aircraft checking in")
		// Build stack summary for response
		stackInfo := ""
		all := stack.GetAll()
		if len(all) > 1 {
			stackInfo = fmt.Sprintf(" Stack has %d aircraft.", len(all))
		}
		transmit(comp.MarshalMarkingMom(callsign, pos, angels, altimeter, ceilingFt, brc) + stackInfo)

	case containsAny(lower, "see you at 10", "see you at ten"):
		transmit(comp.MarshalRadarContact(callsign, 10))

	case containsAny(lower, "state") && fuelState > 0:
		transmit(comp.MarshalCopyState(callsign, fuelState))

	case containsAny(lower, "established angels", "established at angels"):
		stack.SetPhase(callsign, "holding")
		angels := 6
		if ac, ok := stack.GetAircraft(callsign); ok {
			angels = ac.Angels
		}
		if atcCtrl.IsDeckClear() {
			stack.SetPhase(callsign, "charlie")
			transmit(comp.MarshalSignalCharlie(callsign))
		} else {
			transmit(comp.MarshalEstablishedAck(callsign, angels))
		}

	case containsAny(lower, "commencing"):
		stack.SetPhase(callsign, "commencing")
		stack.Remove(callsign)
		transmit(comp.MarshalCopyCommencing(callsign, fuelState))
		// Internal stack collapse only — pack remaining aircraft down to fill
		// the vacated slot so the next "marking moms" gets the correct angels.
		// Per 07.png Marshal does not transmit step-down clearances.
		for _, sd := range stack.CollapseStack(marshalMinAngels) {
			log.Info().Str("callsign", sd.Callsign).Int("from", sd.OldAngels).Int("to", sd.NewAngels).Msg("Marshal: stack step-down (internal, no TX)")
		}

	case containsAny(lower, "initial"):
		// 3nm initial — pilot is rolling on the boat, hand off to LSO/Paddles.
		// Marshal's last call before pilot pushes to the LSO freq.
		log.Info().Str("callsign", callsign).Int("button", marshalTacanChannel).Msg("Marshal: 3nm initial, handing off to LSO")
		transmit(comp.MarshalPushButton(callsign, marshalTacanChannel))

	}
}

