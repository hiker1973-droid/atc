"""Generate pkg/airfield/*.go for the Syria (Eastern Med) theatre.

Data provenance, in priority order:

  * TOWER / ATIS frequencies  -- the ratified vSFG-7 "Hornet Radio Presets - Eastern
    Med" card. These are what a pilot has on a preset, so they are what SkyEye must
    transmit on. ⚠ They DIFFER from the DCS terrain's own ATC on 7 of 8 fields (see
    SYRIA_PLAN.md) -- deliberate, not a transcription slip.
  * POSITION / ELEVATION / RUNWAYS / ILS -- CombatWombat's "Airfield Diagrams, Syrian
    Theatre v5.0" AIRFIELD SUMMARY (PDF pages 6-7), which is the surveyed document for
    this map. Cross-checked against the DCS install's beacons.lua.

⚠ THRESHOLDS ARE COMPUTED, NOT SURVEYED -- center +/- half the runway length along the
runway heading, same as the Caucasus and Iraq fields already in this repo. They are good
enough for phraseology and the 3-mile initial call but MUST be verified against DCS
before anyone enables the --position-check hold-short gate.

Run from anywhere:  python tools/gen_syria_airfields.py
"""
import math
import os

# tools/ -> repo root -> pkg/airfield (the script moved into tools/ after it was written).
OUT = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "pkg", "airfield")

MAGVAR = 5.5          # DimOn 2025 value for the Syria map (operator 2026-09-19; was 5.0).
                      # Used for wind -> magnetic, spoken BRC and Command bearings -- NOT
                      # documentation only, whatever older comments say.
PATTERN_ALT = 1500
HANDOFF = ("command", 282.000, "channel four")   # COMMAND is COMM1 P4 on the E-Med card


def dms(d, m, s):
    return d + m / 60.0 + s / 3600.0


# icao: (name, dcs_name, lat, lon, elev_ft, tower, atis, rwy_len_m, pairs, note)
# pairs: [(designator, magnetic heading, reciprocal designator, recip heading)]
F = {
    "LTAG": ("Incirlik", "Incirlik", dms(37, 0, 7), dms(35, 25, 33), 230,
             360.100, 360.200, 2926,
             [("05", 49.0, "23", 229.0)],
             "ILS 05 109.30/055 and 23 111.70/235. TACAN DAN ch21. Tower matches the "
             "DCS terrain exactly (360.10) -- the only one of the eight that does. "
             "2026-09-19: elevation 156 -> 230 ft, headings 055/235 -> 049/229 magnetic "
             "and length 9600 ft per DimOn Aerodrome Data 01 Feb 2026."),
    "LLRD": ("Ramat David", "Ramat David", dms(32, 39, 58.978), dms(35, 11, 2.049), 146,
             251.300, 256.150, 2347,
             [("15", 141.0, "33", 321.0)],
             "Three runway pairs on the field (09/27, 11/29, 15/33); only 15/33 has an ILS "
             "so that is the pair modelled here. ⚠ 2026-09-19 per DimOn Aerodrome Data "
             "01 Feb 2026: the ILS 111.10 serves RUNWAY 33, not 15; centre moved ~420 m "
             "to the aerodrome reference point, elevation 105 -> 146 ft, headings "
             "146/326 -> 141/321 magnetic, TACAN 84X, VOR 113.70, NDB 368. "
             "⚠ DCS terrain tower is 251.05; the card says 251.300 and the card wins."),
    "OJMF": ("Mafraq", "King Hussein Air College", dms(32, 21, 23), dms(36, 15, 33), 2204,
             250.450, 255.550, 3000,
             [("13", 132.0, "31", 312.0)],
             "Called KING HUSSEIN AC on the squadron card; OJMF Mafraq AB on the chart. "
             "VORTAC ABC 115.90 ch106, ILS 111.70 on 13. "
             "⚠ DCS terrain tower is 250.40; the card says 250.450."),
    "LTDA": ("Hatay", "Hatay", dms(36, 21, 37), dms(36, 17, 6), 253,
             250.300, 249.300, 2900,
             [("04", 44.0, "22", 224.0)],
             "ILS 04 108.90/044 and 22 108.15/224. VOR/DME HTY 112.05, NDB HTY 336. "
             "⚠ DCS terrain tower is 250.25; the card says 250.300. "
             "ATIS 249.300 is ASSIGNED -- the card gives Hatay tower-only."),
    "LTAJ": ("Gaziantep", "Gaziantep", dms(36, 56, 52), dms(37, 28, 44), 2305,
             250.100, 249.400, 3000,
             [("10", 106.0, "28", 286.0)],
             "Chart gives RWY 10/28 with the ILS on 28 (109.10/286). The beacons.lua "
             "localizer direction of 106 is the 10 end. VOR/DME GAZ 116.70, NDB 432. "
             "⚠ DCS terrain tower is 250.05; the card says 250.100. "
             "ATIS 249.400 is ASSIGNED -- the card gives Gaziantep tower-only."),
    "LCRA": ("Akrotiri", "Akrotiri", dms(34, 35, 26), dms(32, 59, 17), 69,
             252.000, 249.500, 2743,
             [("10", 106.0, "28", 286.0)],
             "⚠ RUNWAY RESOLVED FROM THE CHART, NOT THE LOCALIZER. beacons.lua gives the "
             "IAK localizer 111 true, which reads as RWY 11; the surveyed chart and the "
             "real RAF Akrotiri are both 10/28, with the ILS on 28 (109.70/291). The "
             "chart wins. TACAN AK ch107, DME 116.00. "
             "⚠ DCS terrain tower is 251.70; the card says 252.000. "
             "ATIS 249.500 is ASSIGNED -- the card gives Akrotiri tower-only. "
             "2026-09-19: elevation 76 -> 69 ft per DimOn Aerodrome Data 01 Feb 2026 "
             "(which also confirms 10/28, ILS 28 109.70 and TACAN 107X)."),
    "LCPH": ("Paphos", "Paphos", dms(34, 43, 7), dms(32, 29, 8), 40,
             249.100, 249.000, 2700,
             [("11", 114.0, "29", 294.0)],
             "ILS on 29 (108.90/294). TACAN PHA ch79, VOR/DME PHA 117.90, NDB 328. "
             "⚠ Tower 249.100 is a vSFG-7 MOVE: the card prints 252.100, which collides "
             "with TEXACO 1. The tanker keeps 252.100 (squadron-wide standard) and "
             "Paphos moved beside its own ATIS on 249.000. "
             "⚠ DCS terrain tower is 251.80."),
    "OJHR": ("H4", "H4", dms(32, 32, 12), dms(38, 12, 22), 2257,
             252.250, 240.850, 2400,
             [("10", 106.0, "28", 286.0)],
             "The squadron card calls this H3; there is NO H3 on the Syria map -- H3 is "
             "an Iraqi field, off-map east. This is OJHR 'H-4 AB' (user ruling "
             "2026-08-29). ⚠ NO ILS, TACAN, VOR or NDB anywhere on the field: "
             "beacons.lua has nothing here, so its ATIS reports no approach aids. "
             "⚠ DCS terrain tower is 250.10; the card says 252.250."),
    # --- Added 2026-09-18 as Foothold's alternate/divert fields (operator ruling:
    # Foothold keeps ATC+ATIS at Akrotiri, Incirlik, Ramat David + these two).
    # Neither is on the Eastern Med card and CombatWombat's summary was not on hand,
    # so EVERYTHING below is read from the DCS Syria beacons.lua: ILS/VOR positions
    # give the runway line, the beacon Y altitude gives elevation. Frequencies are the
    # operator's picks. Localizer course = beacons.lua direction + 180 (checked on
    # Beirut, whose three ILS match the real 16/17/03).
    "OSLK": ("Bassel Al-Assad", "Bassel Al-Assad", dms(35, 24, 6.100), dms(35, 57, 1.023), 94,
             250.600, 249.600, 2713,
             [("17R", 173.0, "35L", 353.0), ("17L", 173.0, "35R", 353.0)],
             "DCS calls the field Latakia (airfield21). TWO PARALLEL RUNWAYS: 17R/35L "
             "8,900 ft carries the ILS 109.10 (IBA, on 17R) and is listed first so it is "
             "the default; 17L/35R 7,900 ft. VOR/DME LTK 114.80 on the field, NDB 414. "
             "Position, elevation, runways and headings (173/353 magnetic) from DimOn "
             "Aerodrome Data 01 Feb 2026 -- 2026-09-19 correction of the single '17/35' "
             "first read off beacons.lua. Both pairs share one computed centre. Tower "
             "250.600 = the DCS terrain's own UHF tower. ATIS 249.600 ASSIGNED."),
    "OLBA": ("Beirut", "Beirut-Rafic Hariri", dms(33, 49, 38.636), dms(35, 29, 15.261), 39,
             250.650, 249.700, 3139,
             [("16", 164.0, "34", 344.0), ("03", 30.0, "21", 210.0)],
             "Three runways (03/21, 16/34, 17/35); 16/34 (10,300 ft) and 03/21 modelled. "
             "ILS 16 IBB 110.10, ILS 17 BIL 109.50, ILS 03 IKK 110.70. VOR/DME KAD "
             "112.60, NDB 351. Position (2026-09-19: moved ~700 m from the beacons.lua "
             "estimate), elevation and runways from DimOn Aerodrome Data 01 Feb 2026. "
             "⚠ DCS terrain tower is UHF 253.200 -- the SHELL 2 tanker's frequency in "
             "Foothold -- so SkyEye uses 250.650 (operator ruling 2026-09-18). ATIS "
             "249.700 ASSIGNED. Both pairs share one computed centre, so thresholds "
             "are rougher than usual; keep --position-check off."),
}


def threshold(lat, lon, hdg_mag, dist_m):
    """Point dist_m from (lat,lon) along a magnetic heading, as [lon, lat]."""
    brg = math.radians(hdg_mag + MAGVAR)          # -> true
    dlat = (dist_m * math.cos(brg)) / 111_320.0
    dlon = (dist_m * math.sin(brg)) / (111_320.0 * math.cos(math.radians(lat)))
    return (round(lon + dlon, 5), round(lat + dlat, 5))


def wrap(text, width, indent):
    out, line = [], ""
    for w in text.split():
        if len(line) + len(w) + 1 > width:
            out.append(indent + line)
            line = w
        else:
            line = (line + " " + w).strip()
    if line:
        out.append(indent + line)
    return "\n".join(out)


# The two Foothold divert fields are not on the Eastern Med card and have no
# CombatWombat cross-check of their own, so their header says so.
PROVENANCE = {
    "OSLK": ("// Foothold alternate/divert field (operator ruling 2026-09-18). Tower/ATIS are\n"
             "// the operator's picks, not the Eastern Med card; position, elevation, runways\n"
             "// and ILS from DimOn Aerodrome Data 01 Feb 2026 and the DCS Syria beacons.lua.\n"),
}
PROVENANCE["OLBA"] = PROVENANCE["OSLK"]
DEFAULT_PROVENANCE = (
    "// Tower/ATIS from the ratified vSFG-7 \"Hornet Radio Presets — Eastern Med\" card;\n"
    "// position, elevation, runways and ILS from CombatWombat's Airfield Diagrams\n"
    "// (Syrian Theatre v5.0) AIRFIELD SUMMARY, cross-checked with the DCS beacons.lua\n"
    "// and (2026-09-19) DimOn Aerodrome Data 01 Feb 2026.\n")

TPL = '''package airfield

import "github.com/paulmach/orb"

// {icao} is {name} — Syria (Eastern Med) theatre.
{provenance}// Thresholds are COMPUTED from center + heading + length — verify against DCS
// before enabling --position-check.
//
{note}
var {icao} = &Airfield{{
\tICAO:            "{icao}",
\tName:            "{name}",
\tDCSName:         "{dcs}", // VERIFY exact ME name on the Syria map
\tCenter:          orb.Point{{{lon:.5f}, {lat:.5f}}}, // [lon, lat]
\tElevationFt:     {elev},
\tMagVar:          {magvar}, // ~+{magvar}°E over the Levant; documentation only
\tPatternAltFt:    {pat},
\tTowerFreqMHz:    {tower:.3f},
\tApproachFreqMHz: {tower:.3f},
\tATISFreqMHz:     {atis:.3f},
\tDepartureDistNm: 7,
\tDepartureAngels: 3,
\tHandoffCallsign: "{hc}",
\tHandoffFreqMHz:  {hf:.3f},
\tHandoffPreset:   "{hp}", // COMMAND is COMM1 P4 on the Eastern Med card
\tBreakDirections: map[string]string{{
{breaks}
\t}},
\tRunwayPairs: []RunwayPair{{
{pairs}
\t}},
}}
'''


def main():
    os.makedirs(OUT, exist_ok=True)
    for icao, (name, dcs, lat, lon, elev, tower, atis, length, pairs, note) in F.items():
        half = length / 2.0
        brk, prs = [], []
        for des, hdg, rdes, rhdg in pairs:
            brk.append(f'\t\t"{des}": "left", // TODO verify pattern side vs ramp in DCS')
            brk.append(f'\t\t"{rdes}": "left",')
            p_lon, p_lat = threshold(lat, lon, rhdg, half)   # threshold is UPWIND of center
            r_lon, r_lat = threshold(lat, lon, hdg, half)
            prs.append(
                "\t\t{\n"
                f"\t\t\tPrimary:    Runway{{Designator: \"{des}\", MagneticHeading: {hdg:.1f}, "
                f"ThresholdLatLon: orb.Point{{{p_lon:.5f}, {p_lat:.5f}}}}},\n"
                f"\t\t\tReciprocal: Runway{{Designator: \"{rdes}\", MagneticHeading: {rhdg:.1f}, "
                f"ThresholdLatLon: orb.Point{{{r_lon:.5f}, {r_lat:.5f}}}}},\n"
                "\t\t},")
        src = TPL.format(icao=icao, name=name, dcs=dcs, lat=lat, lon=lon, elev=elev,
                         provenance=PROVENANCE.get(icao, DEFAULT_PROVENANCE),
                         magvar=MAGVAR, pat=PATTERN_ALT, tower=tower, atis=atis,
                         hc=HANDOFF[0], hf=HANDOFF[1], hp=HANDOFF[2],
                         breaks="\n".join(brk), pairs="\n".join(prs),
                         note=wrap(note, 76, "// "))
        path = os.path.join(OUT, icao.lower() + ".go")
        with open(path, "w", encoding="utf-8", newline="\n") as fh:
            fh.write(src)
        print(f"  wrote {path}")
    print(f"\n{len(F)} airfields generated")


if __name__ == "__main__":
    main()
