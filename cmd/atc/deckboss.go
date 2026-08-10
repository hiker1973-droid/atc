package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"math/rand"
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/vsfg7/atc/pkg/composer"
	"github.com/vsfg7/atc/pkg/controller"
	"github.com/vsfg7/atc/pkg/state"
)

// catNumRe captures the cat number from "cat 1" / "CAT 2" / "cat3".
var catNumRe = regexp.MustCompile(`\bcat\s*(\d)\b`)

// assignCatForCaller picks a catapult for a §1 check-in.
//
// With deckPositionCheck off this is byte-for-byte the historical behaviour:
// first free cat, lowest number first. With it on, Deckboss consults Tacview
// for where the caller is actually parked and prefers the pair of cats nearest
// that spot — bow cats 1/2 for aircraft forward of the island, waist cats 3/4
// for aircraft aft — so the assignment matches the deck the pilot is looking at
// instead of always starting at cat one.
//
// Fails open by design. If Tacview has no carrier, no contact for the callsign,
// or shows them off the boat, we log why and fall back to the plain first-free
// assignment rather than leaving a pilot without a cat mid-mission.
func assignCatForCaller(callsign string, deckPositionCheck bool,
	atcCtrl *controller.ATCController, deck *state.DeckbossState) int {

	if !deckPositionCheck {
		return deck.AssignCat(callsign)
	}
	foreAft, rangeNm, onDeck := atcCtrl.DeckSpotNm(callsign)
	if !onDeck {
		log.Warn().Str("callsign", callsign).Float64("rangeNm", rangeNm).
			Msg("Deckboss: deck position check — no on-deck Tacview contact, falling back to first free cat")
		return deck.AssignCat(callsign)
	}
	preferBow := foreAft >= 0
	catNum := deck.AssignCatPreferred(callsign, preferBow)
	log.Info().Str("callsign", callsign).Float64("foreAftNm", foreAft).
		Bool("preferBow", preferBow).Int("cat", catNum).
		Msg("Deckboss: cat assigned from Tacview deck position")
	return catNum
}

// deckbossLoop handles carrier deck operations (default 128.600 MHz —
// DCS carrier UHF control).
func deckbossLoop(ctx context.Context, srsAddr string, freqMHz float64, apiKey, eamPassword, voice string,
	txCooldown *int64, atcCtrl *controller.ATCController, deck *state.DeckbossState, deckPositionCheck bool) {

	const deckCallsign = "Deckboss"
	deckVoice := voice
	if deckVoice == "" {
		// Keep in sync with the --deckboss-voice default in main.go.
		deckVoice = "shimmer"
	}
	comp := composer.NewATCComposer(deckCallsign)

	transmit := func(text string) {
		log.Info().Str("text", text).Msg("Deckboss TX")
		atomic.StoreInt64(txCooldown, time.Now().Add(estimateTTSDuration(text)).UnixNano())
		mp3, err := synthesizeSpeech(ctx, apiKey, text, deckbossVoice(deckVoice))
		if err != nil {
			log.Error().Err(err).Msg("Deckboss TTS failed")
			return
		}
		srsHost, srsPort, _ := net.SplitHostPort(srsAddr)
		transmitExternalAudioFile(ctx, mp3, freqMHz, "OMDM-DKB", srsHost, srsPort, flagExternalAudio)
	}

	// releaseCat frees every slot held by cs and, when someone is waiting,
	// pulls the head of the conga onto a cat and tells them which one.
	//
	// The cat-clear names the cat next-up was actually assigned rather than the
	// one that just freed. On a partially spotted deck those differ — free cat
	// 1 with cat 3 just released and next-up is assigned 1 but was being sent
	// to 3, a cat another aircraft is sitting on.
	//
	// txDelay staggers the transmission so the cat-clear doesn't step on the
	// ack that triggered the release. Returns the freed cat number, 0 if cs
	// held no slot (so a double-fire from §2a and §4 is a no-op).
	releaseCat := func(cs string, txDelay time.Duration) int {
		freed, next, nextCat := deck.ReleaseAndPullNext(cs)
		if next != "" {
			go func() {
				if txDelay > 0 {
					select {
					case <-ctx.Done():
						return
					case <-time.After(txDelay):
					}
				}
				transmit(fmt.Sprintf("%s, %s", next, comp.DeckbossCatClear(nextCat)))
			}()
		}
		return freed
	}

	// Tacview monitor — reclaims cat slots the radio flow never released.
	go func() {
		const (
			// minTimeOnCat is how long a slot must be held before the monitor
			// considers it at all: a pilot who just got the cat is still
			// taxiing to it and has not launched.
			minTimeOnCat = 2 * time.Minute
			// staleSlotTimeout is how long a slot may be held by an aircraft
			// Tacview no longer sees before Deckboss reclaims it. Covers the
			// pilot who disconnects, respawns, or dies on the deck — none of
			// which produce the climb-out IsAircraftAirborne looks for, so
			// without this the slot was held for the life of the process and
			// the deck slowly read full with nothing on it.
			staleSlotTimeout = 5 * time.Minute
			// congaStaleTimeout is the same idea for the conga line. A queued
			// pilot who leaves keeps their place and gets handed a cat they
			// will never taxi to, which re-fills the deck with a ghost.
			congaStaleTimeout = 5 * time.Minute
			// contactMaxAge is how fresh a Tacview contact must be to count as
			// "still on the server".
			contactMaxAge = 60 * time.Second
		)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Every reclaim below needs a live feed to distinguish "gone"
				// from "not being tracked". With Tacview down every callsign
				// reads as absent, which would clear the whole deck.
				tacviewUp := atcCtrl.IsTacviewActive()

				for n := 1; n <= 4; n++ {
					cat := deck.GetCat(n)
					if cat.Callsign == "" || time.Since(cat.UpdatedAt) < minTimeOnCat {
						continue
					}
					switch {
					case atcCtrl.IsAircraftAirborne(cat.Callsign):
						log.Info().Str("callsign", cat.Callsign).Int("cat", cat.Number).
							Msg("Deckboss: Tacview detected launch")
						releaseCat(cat.Callsign, 3*time.Second)
					case tacviewUp && time.Since(cat.UpdatedAt) > staleSlotTimeout &&
						!atcCtrl.HasRecentContact(cat.Callsign, contactMaxAge):
						log.Warn().Str("callsign", cat.Callsign).Int("cat", cat.Number).
							Dur("heldFor", time.Since(cat.UpdatedAt)).
							Msg("Deckboss: reclaiming stale cat — no Tacview contact, aircraft gone")
						releaseCat(cat.Callsign, 3*time.Second)
					}
				}

				if !tacviewUp {
					continue
				}
				// Candidates are collected first so the deck lock is never
				// held across a controller call.
				for _, cs := range deck.CongaWaitingSince(congaStaleTimeout) {
					if atcCtrl.HasRecentContact(cs, contactMaxAge) {
						continue
					}
					if deck.RemoveFromConga(cs) {
						log.Warn().Str("callsign", cs).
							Msg("Deckboss: dropped stale conga entry — no Tacview contact, aircraft gone")
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
		// Defensive prefix list — Whisper variants of "Deck boss" we've seen
		// in the wild: "duck boss", "tech boss", "dec boss". Add as observed.
		addrPrefixes := []string{"deckboss", "deck boss", "duck boss", "duckboss", "tech boss", "techboss", "dec boss", "decboss", "check boss", "checkboss", "yak boss", "yakboss", "dakwas", "dak was"}
		addressed := false
		for _, p := range addrPrefixes {
			if strings.HasPrefix(lower, p) {
				addressed = true
				break
			}
		}

		switch {
		case containsAny(lower, "request taxi", "requesting taxi", "ready for taxi", "ready to taxi", "green jet", "greenjet"):
			if !addressed {
				log.Debug().Str("text", text).Msg("Deckboss: §1 dropped — not address-led, likely self-echo")
				return
			}
			// §1 check-in: assign cat or queue in conga
			catNum := assignCatForCaller(callsign, deckPositionCheck, atcCtrl, deck)
			if catNum > 0 {
				transmit(comp.DeckbossCatAssignment(callsign, catNum))
			} else {
				// §1b/§1c/§1d. The result codes decide which response the
				// caller gets: a pilot re-checking in from the line hears
				// their position (§1c), not another "join the conga" (§1b).
				pos, res := deck.EnqueueConga(callsign)
				switch res {
				case state.CongaFull:
					transmit(comp.DeckbossDeckFull(callsign))
				case state.CongaAlready:
					transmit(comp.DeckbossStandby(callsign, pos))
				default:
					transmit(comp.DeckbossCongaLine(callsign))
				}
			}

		case (containsAny(lower, "tension", "ready") && containsAny(lower, "cat")) || containsAny(lower, "shoot"):
			if !addressed {
				log.Debug().Str("text", text).Msg("Deckboss: §2 dropped — not address-led, likely self-echo of under-tension response")
				return
			}
			// §2 under tension / shoot shortcut: pilot says either
			// "under tension cat X" / "ready cat X" OR the shortcut "shoot"
			// (with the address). Fires the under-tension ack and starts the
			// 10s cat-clear timer below. Cat number is sourced from state
			// (§1 assignment), then from text regex, then a generic ack as
			// last resort.
			//
			// Note the address guard above is load-bearing for a second
			// reason now: the ack ends in "shooter's discretion", which
			// contains the bare "shoot" trigger. The echo comes back
			// callsign-led, so it is dropped before it can re-fire §2.
			catNum := deck.GetCatByCallsign(callsign)
			if catNum == 0 {
				// Pilot called §2 without prior §1 — parse cat number directly
				// from the transmission so we still ack ("Cat 1 under tension").
				if m := catNumRe.FindStringSubmatch(lower); m != nil {
					var n int
					fmt.Sscanf(m[1], "%d", &n)
					if n >= 1 && n <= 4 {
						catNum = n
					}
				}
			}
			if catNum > 0 {
				transmit(comp.DeckbossUnderTension(callsign, catNum))
				// §2a slot management. Deckboss no longer calls the shot —
				// the under-tension ack ends with "shooter's discretion" and
				// the shooter fires the jet off the deck, so there is no
				// "Cat X, shoot, shoot, shoot" TX (removed 2026-08-06). The
				// timer still runs the slot handoff: 10 seconds after the
				// under-tension ack the aircraft is off the cat, so free it
				// and pull the next conga aircraft onto it. Next-up hears
				// their slot without waiting on the launching pilot's
				// airborne call; §4 still covers the generic-ack edge case.
				go func(cs string) {
					select {
					case <-ctx.Done():
						return
					case <-time.After(10 * time.Second):
					}
					if freed := releaseCat(cs, 0); freed > 0 {
						log.Info().Str("callsign", cs).Int("cat", freed).Msg("Deckboss: cat cleared (post-launch)")
					}
				}(callsign)
			} else {
				// Couldn't parse a cat number from the call — generic ack
				transmit(fmt.Sprintf("%s, Deckboss, copy under tension.", callsign))
			}

		case containsAny(lower, "say brc", "request brc", "brc check", "check brc", "what's brc", "what is brc", "say bearing", "current brc", "current bearing", "current vrc", "say vrc", "check vrc"):
			if !addressed {
				log.Debug().Str("text", text).Msg("Deckboss: BRC dropped — not address-led")
				return
			}
			// §7 BRC request: pilot asks mother's bow heading
			brc := atcCtrl.GetCarrierBRC()
			log.Info().Str("callsign", callsign).Float64("brc", brc).Msg("Deckboss: BRC request")
			transmit(comp.MarshalSayBRC(callsign, brc))

		case containsAny(lower, "tension"):
			// §3 tension-only (no cat word): pilot confirms tension — silent, they go
			log.Debug().Str("callsign", callsign).Msg("Deckboss: tension confirmed, pilot launching")

		case containsAny(lower, "airborne", "clear traffic"):
			// §4: pilot's optional airborne callout. In the standard flow
			// the cat was already cleared by §2a on the 10s post-launch
			// timer, so this is just an ack. The slot-management block below
			// only fires as a fallback when §2a was skipped (generic "copy
			// under tension" in §2 because no cat number was parseable).
			// FreeCat returns 0 if the slot is already cleared, so the
			// double-fire guard is free.
			//
			// The ack is now "copy airborne" (2026-08-06) — it repeats the
			// trigger word, so the old "omit the word" self-echo protection
			// is gone and the mid-string check below replaces it: our own TX
			// comes back callsign-led with "Deckboss" in the middle
			// ("Raider 032, Deckboss, copy airborne."), which no pilot call
			// looks like. Pilots may or may not lead with "Deckboss" on this
			// call, so both of their shapes still get through — address-led
			// ("Deckboss, Raider 032, airborne") and bare ("Raider 032,
			// airborne").
			if !addressed && containsAny(lower, addrPrefixes...) {
				log.Debug().Str("text", text).Msg("Deckboss: §4 dropped — Deckboss mid-string, self-echo of airborne ack")
				return
			}
			transmit(fmt.Sprintf("%s, Deckboss, copy airborne.", callsign))
			// Also drops them from the conga if they somehow launched while
			// still queued — otherwise the line hands them a cat later.
			deck.RemoveFromConga(callsign)
			if catNum := releaseCat(callsign, 3*time.Second); catNum > 0 {
				log.Info().Str("callsign", callsign).Int("cat", catNum).Msg("Deckboss: cat cleared (airborne fallback)")
			}

		case containsAny(lower, "pushing command", "switching command", "push command", "switch command",
			"pushing strike", "switching strike", "pushing departure", "switching departure"):
			if !addressed {
				log.Debug().Str("text", text).Msg("Deckboss: handoff ack dropped — not address-led, likely self-echo")
				return
			}
			// Pilot's courtesy call on the departure handoff issued with the
			// airborne ack. Short ack only. Address-led because our own
			// handoff TX contains "switch to <command name>".
			log.Info().Str("callsign", callsign).Msg("Deckboss: pilot-initiated command handoff ack")
			transmit(comp.HandoffAck(callsign, flagHandoffCommandName))

		case containsAny(lower, "bolter",
			"remain in bold", "remaining in bold", "staying in bold", "in the bold", "bold pattern"):
			if !addressed {
				log.Debug().Str("text", text).Msg("Deckboss: §8 bolter dropped — not address-led, likely self-echo")
				return
			}
			// §8 bolter pattern: pilot doing trap practice, staying in the
			// touch-and-go pattern. Deckboss acks and hands off to the LSO
			// (who owns the recovery pattern); no deck state change. Address-led
			// to avoid self-echo since our response also contains "bolter".
			// Matches the bare word "bolter" (rare enough it won't false-fire,
			// and the address guard covers self-echo) so every inflection lands
			// — "remain in bolter", "remaining in bolter", "in the bolter", etc.
			// The earlier enumerated list missed "remaining in bolter" (the -ing
			// form is not a superstring of "remain in bolter"); collapsed to the
			// bare token 2026-07-05.
			// "bold" phrase variants also 2026-07-05 — Whisper renders "bolter"
			// as "bold" ("remaining in bold" from Raider 311 got no response).
			// Scoped to phrases, never a bare "bold", so ball-flying chatter
			// ("looking good") can't false-fire it.
			transmit(comp.DeckbossBolterPattern(callsign))

		case containsAny(lower, "radio check", "comm check", "comms check", "com check", "com-check", "comm-check", "how copy", "five by five", "five by"):
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

		tcpConn, err := dialSRSKeepAlive(ctx, srsAddr)
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
					if now.Sub(tx.lastPacket) > 400*time.Millisecond && len(tx.opusFrames) > 9 {
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
							cs := extractCallsignSkippingAddress(text, "deckboss", "deck boss", "duck boss", "tech boss", "dec boss", "check boss")
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

