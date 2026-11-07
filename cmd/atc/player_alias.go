package main

// Who is really in each jet. Tacview's pilot name is the one a slot was first flown under: on Day 2026-11-06 Bacon
// (Raider 031) flew the jet Tacview called "Raider 306 | Turbo", so Marshal found no radar contact for 031 and Paddles
// carded his bolter and trap as Turbo's (confirmed in that night's Tacview file: one object, 103, Pilot Turbo all
// evening). DCS-gRPC knows the player in every unit, with its position, so this polls it and tells the controller
// which Tacview jet each player is in (ATCController.RefreshPlayerAliases).

import (
	"context"
	"strings"
	"time"

	"github.com/DCS-gRPC/go-bindings/dcs/v0/coalition"
	"github.com/DCS-gRPC/go-bindings/dcs/v0/common"
	"github.com/DCS-gRPC/go-bindings/dcs/v0/group"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/vsfg7/atc/pkg/controller"
)

const playerAliasEvery = 15 * time.Second

// playerCallsign is the part of a DCS player name before the pipe: "RAIDER 031 | BACON" -> "Raider 031".
func playerCallsign(name string) string {
	if i := strings.Index(name, "|"); i >= 0 {
		name = name[:i]
	}
	words := strings.Fields(name)
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
	}
	return strings.Join(words, " ")
}

// watchPlayerAliases polls DCS-gRPC for the players in blue aircraft. "" (no address) turns it off; a DCS that is
// down is retried quietly.
func watchPlayerAliases(ctx context.Context, addr string, ctrl *controller.ATCController) {
	if addr == "" || ctrl == nil {
		return
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Warn().Err(err).Msg("Player names: DCS-gRPC client failed")
		return
	}
	defer conn.Close()
	co, gr := coalition.NewCoalitionServiceClient(conn), group.NewGroupServiceClient(conn)
	tk := time.NewTicker(playerAliasEvery)
	defer tk.Stop()
	logged := map[string]string{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
		}
		cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		groups, err := co.GetGroups(cctx, &coalition.GetGroupsRequest{Coalition: common.Coalition_COALITION_BLUE, Category: common.GroupCategory_GROUP_CATEGORY_AIRPLANE})
		var players []controller.PlayerFix
		if err == nil {
			for _, g := range groups.GetGroups() {
				us, uerr := gr.GetUnits(cctx, &group.GetUnitsRequest{GroupName: g.GetName()})
				if uerr != nil {
					continue
				}
				for _, u := range us.GetUnits() {
					if u.PlayerName == nil || *u.PlayerName == "" || u.GetPosition() == nil {
						continue
					}
					players = append(players, controller.PlayerFix{Callsign: playerCallsign(*u.PlayerName), Lat: u.GetPosition().GetLat(), Lon: u.GetPosition().GetLon()})
				}
			}
		}
		cancel()
		if err != nil {
			continue
		}
		ctrl.RefreshPlayerAliases(players)
		for _, p := range players {
			if k := ctrl.PlayerAlias(p.Callsign); k != logged[p.Callsign] {
				logged[p.Callsign] = k
				if k != "" {
					log.Info().Str("player", p.Callsign).Str("tacviewName", k).Msg("Player names: Tacview shows this jet under another pilot's name")
				}
			}
		}
	}
}
