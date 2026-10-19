package miz

// A DCS cloud preset (cloud_presets.go, generated from DCS's clouds.lua by
// tools/gen_cloud_presets.py): its layers, bottom up, as they sit at the
// preset's own base.
type cloudPreset struct {
	Name   string       // readable name, e.g. "Scattered Showers 3 [ATMOS-X]"
	First  int          // the preset's first layer altitude, m MSL
	Layers []cloudLayer // visible layers, bottom up
}

type cloudLayer struct {
	AltM  int    // bottom of the layer at the preset's own base, m MSL
	Cover string // FEW / SCT / BKN / OVC
}

// ceilingM is the bottom of the lowest broken or overcast layer, in metres
// MSL, when the mission sets the preset's base to baseM: the mission editor's
// base slider moves every layer by the same amount. false = no ceiling.
func (p cloudPreset) ceilingM(baseM float64) (float64, bool) {
	shift := baseM - float64(p.First)
	for _, l := range p.Layers {
		if l.Cover == "BKN" || l.Cover == "OVC" {
			return float64(l.AltM) + shift, true
		}
	}
	return 0, false
}
