package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/DCS-gRPC/go-bindings/dcs/v0/custom"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// The AI a Guard hail reaches is steered through DCS-gRPC's custom.Eval, which
// runs Lua in the mission environment (the DCS-gRPC config must have
// eval_enabled = true; Guard logs at start whether it is). Nothing here has been
// flown: the Lua follows the Scripting Engine's documented route and task
// tables and is checked only by the tests that read it back.

// aiOrders is what a pilot can tell a hailed aircraft.
type aiOrders interface {
	// Turn flies the group on a true heading for nm miles from where it is.
	Turn(ctx context.Context, group string, hdgTrue, nm float64) error
	// Follow has the group form on another group.
	Follow(ctx context.Context, group, leaderGroup string) error
	// Land sends the group to land at a DCS airbase.
	Land(ctx context.Context, group, airbase string) error
}

// luaQuote makes s a Lua string literal.
func luaQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", " ", "\r", " ").Replace(s) + `"`
}

// luaRoutePrelude finds the group and defines the waypoint helper. It returns
// from the chunk with a reason string when the group is gone.
func luaRoutePrelude(group string) string {
	return fmt.Sprintf(`local g = Group.getByName(%s)
if not g then return "no group" end
local u = g:getUnit(1)
if not u then return "no unit" end
local p = u:getPoint()
local v = u:getVelocity()
local spd = math.sqrt(v.x*v.x + v.z*v.z)
if spd < 150 then spd = 150 end
local function wp(x, z, alt, t)
  t = t or {}
  t.x = x; t.y = z; t.alt = alt; t.alt_type = "BARO"; t.speed = spd
  t.type = t.type or "Turning Point"; t.action = t.action or "Turning Point"
  t.task = { id = "ComboTask", params = { tasks = {} } }
  return t
end
`, luaQuote(group))
}

func luaTurn(group string, hdgTrue, nm float64) string {
	return luaRoutePrelude(group) + fmt.Sprintf(`local a = math.rad(%.1f)
local d = %.0f
g:getController():setTask({ id = "Mission", params = { route = { points = {
  wp(p.x, p.z, p.y), wp(p.x + math.cos(a) * d, p.z + math.sin(a) * d, p.y) } } } })
return "ok"
`, hdgTrue, nm*1852)
}

func luaFollow(group, leaderGroup string) string {
	return fmt.Sprintf(`local g = Group.getByName(%s)
local l = Group.getByName(%s)
if not g then return "no group" end
if not l then return "no leader" end
g:getController():setTask({ id = "Follow", params = { groupId = l:getID(), pos = { x = -150, y = 0, z = -150 },
  lastWptIndexFlag = false } })
return "ok"
`, luaQuote(group), luaQuote(leaderGroup))
}

func luaLand(group, airbase string) string {
	return luaRoutePrelude(group) + fmt.Sprintf(`local ab = Airbase.getByName(%s)
if not ab then return "no airbase" end
local q = ab:getPoint()
g:getController():setTask({ id = "Mission", params = { route = { points = {
  wp(p.x, p.z, p.y),
  wp(q.x, q.z, q.y, { type = "Land", action = "Landing", airdromeId = ab:getID() }) } } } })
return "ok"
`, luaQuote(airbase))
}

// grpcAI sends the Lua over DCS-gRPC.
type grpcAI struct {
	addr string
	mu   sync.Mutex
	conn *grpc.ClientConn
}

func (a *grpcAI) client(ctx context.Context) (custom.CustomServiceClient, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.conn == nil {
		c, err := grpc.NewClient(a.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, err
		}
		a.conn = c
	}
	return custom.NewCustomServiceClient(a.conn), nil
}

// eval runs Lua and fails unless it returned "ok".
func (a *grpcAI) eval(ctx context.Context, lua string) error {
	cl, err := a.client(ctx)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	r, err := cl.Eval(ctx, &custom.EvalRequest{Lua: lua})
	if err != nil {
		return err
	}
	if !strings.Contains(r.GetJson(), `"ok"`) {
		return fmt.Errorf("DCS said %s", r.GetJson())
	}
	return nil
}

func (a *grpcAI) Turn(ctx context.Context, group string, hdgTrue, nm float64) error {
	return a.eval(ctx, luaTurn(group, hdgTrue, nm))
}
func (a *grpcAI) Follow(ctx context.Context, group, leader string) error {
	return a.eval(ctx, luaFollow(group, leader))
}
func (a *grpcAI) Land(ctx context.Context, group, airbase string) error {
	return a.eval(ctx, luaLand(group, airbase))
}

// probe logs once whether Eval works on this rig.
func (a *grpcAI) probe(ctx context.Context) {
	cl, err := a.client(ctx)
	if err == nil {
		c, cancel := context.WithTimeout(ctx, 6*time.Second)
		defer cancel()
		_, err = cl.Eval(c, &custom.EvalRequest{Lua: `return "ok"`})
	}
	if err != nil {
		log.Warn().Err(err).Str("addr", a.addr).Msg("Guard: DCS-gRPC Eval is NOT available - hails are answered but orders to AI aircraft will be refused (set eval_enabled = true in the DCS-gRPC config)")
		return
	}
	log.Info().Str("addr", a.addr).Msg("Guard: DCS-gRPC Eval available - AI aircraft can be ordered")
}
