package terrain

import (
	"embed"
	"fmt"
	"strings"
	"sync"
)

// The grids built by tools/terrain-grid/build_grid.py, embedded so a rig needs
// nothing but atc.exe. Caucasus 2026-10-21 (1.4 MB, 0.01 deg); Syria and Kola
// to follow.
//
//go:embed grids/*.grid.gz
var grids embed.FS

var (
	cacheMu sync.Mutex
	cache   = map[string]*Grid{}
)

// ForMap returns the grid for a --map name ("caucasus"), loaded once.
// An error means there is no grid for that map (terrain masking stays off).
func ForMap(name string) (*Grid, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if g, ok := cache[name]; ok {
		return g, nil
	}
	gz, err := grids.ReadFile("grids/" + name + ".grid.gz")
	if err != nil {
		return nil, fmt.Errorf("terrain: no grid for map %q", name)
	}
	g, err := Load(gz)
	if err != nil {
		return nil, err
	}
	cache[name] = g
	return g, nil
}
