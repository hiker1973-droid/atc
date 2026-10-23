package miz

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

const testMission = `mission =
{
	["date"] =
	{
		["Day"] = 25,
		["Month"] = 1,
		["Year"] = 2027,
	}, -- end of ["date"]
	["descriptionText"] = "DictKey_descriptionText_1",
	["descriptionBlueTask"] = "DictKey_descriptionBlueTask_3",
	["sortie"] = "DictKey_sortie_5",
	["theatre"] = "Caucasus",
	["coalition"] =
	{
		["blue"] =
		{
			["country"] =
			{
				[1] =
				{
					["name"] = "USA",
					["plane"] =
					{
						["group"] =
						{
							[1] =
							{
								["name"] = "Venom 1",
								["task"] = "CAP",
								["units"] =
								{
									[1] = { ["skill"] = "Client", ["type"] = "F-16C_50", ["name"] = "Venom 1-1", ["payload"] = { ["pylons"] = { [1] = { ["CLSID"] = "{AIM-9}" }, }, }, },
									[2] = { ["skill"] = "Client", ["type"] = "F-16C_50", ["name"] = "Venom 1-2", },
								},
							},
							[2] =
							{
								["name"] = "Texaco 1-1",
								["task"] = "Refueling",
								["units"] = { [1] = { ["skill"] = "High", ["type"] = "KC130", }, },
							},
						},
					},
				},
			},
		},
	},
	["start_time"] = 32400,
	["weather"] =
	{
		["qnh"] = 760,
		["wind"] = { ["atGround"] = { ["speed"] = 4, ["dir"] = 300, }, },
	},
	["trig"] = { ["func"] = { [1] = "a.b(\"x\") -- not a comment", }, },
	["flag"] = true,
}
`

const testDict = `dictionary =
{
	["DictKey_descriptionText_1"] = "vSFG-7 CAUCASUS TRAINING RANGE\
Line two",
	["DictKey_descriptionBlueTask_3"] = "Fly CAP over Senaki",
	["DictKey_sortie_5"] = "",
}
`

func TestReadBriefing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Training V1.36.miz")
	f, _ := os.Create(path)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("mission")
	w.Write([]byte(testMission))
	w, _ = zw.Create("l10n/DEFAULT/dictionary")
	w.Write([]byte(testDict))
	zw.Close()
	f.Close()

	b, err := ReadBriefing(path)
	if err != nil {
		t.Fatal(err)
	}
	if b.Name != "Training V1.36" || b.Theatre != "Caucasus" || b.Date != "25 Jan 2027" || b.StartTime != "09:00" {
		t.Fatalf("header %+v", b)
	}
	if b.Description != "vSFG-7 CAUCASUS TRAINING RANGE\nLine two" || b.BlueTask != "Fly CAP over Senaki" {
		t.Fatalf("texts %q / %q", b.Description, b.BlueTask)
	}
	if len(b.Flights) != 1 || b.Flights[0].Group != "Venom 1" || b.Flights[0].Slots != 2 || b.Flights[0].Aircraft != "F-16C_50" || b.Flights[0].Task != "CAP" {
		t.Fatalf("flights %+v", b.Flights)
	}
	if b.Weather.WindKts < 7 || b.Weather.WindKts > 8 {
		t.Fatalf("weather %+v", b.Weather)
	}
}
