package kbdata

import (
	"errors"
	"testing"
)

func TestShippedFilesAreValid(t *testing.T) {
	docks, err := LoadDocks("../docks.json")
	if err != nil {
		t.Fatal(err)
	}
	if p := ValidateDocks(docks); len(p) > 0 {
		t.Errorf("docks.json:\n  %v", p)
	}
	devices, err := LoadDevices("../devices.json")
	if err != nil {
		t.Fatal(err)
	}
	if p := ValidateDevices(devices); len(p) > 0 {
		t.Errorf("devices.json:\n  %v", p)
	}
	if _, ok := docks.GenericInternalHubs["8087:0b40"]; !ok {
		t.Error("the Goshen Ridge hub should be listed as generic")
	}
}

func TestValidateDockCatchesTheUsualMistakes(t *testing.T) {
	generic := map[string]string{"8087:0b40": "Goshen Ridge hub"}
	bad := Dock{
		ID: "Bad Id", Name: " ", Hubs: []string{"2188:5500", "2188:5500", "8087:0b40", "ZZZZ:0001"},
		Uplink:  &Uplink{Kind: "", MaxLink: "fast"},
		USB4:    &USB4{Vendor: "Intel", Model: "8087:0b26"},
		Ports:   []Port{{Label: "", Connector: "hdmi", MaxLink: "ss10", Count: 0}},
		PortMap: []PortMap{{Hub: "1234:0001", Port: 0, Label: ""}},
	}
	problems := ValidateDock(bad, generic)
	want := []string{
		"must be a lower-case slug", "name is required", "listed twice", "generic chipset hub",
		"not lower-case vid:pid", "uplink.kind is required", "not a link speed", "looks like a vid:pid",
		"label is required", "connector \"hdmi\"", "count must be", "not one of the dock's hubs",
		"port must be at least 1", "verified is required",
	}
	for _, w := range want {
		if !anyContains(problems, w) {
			t.Errorf("no problem mentions %q in:\n  %v", w, problems)
		}
	}
	good := Dock{ID: "acme-dock-9", Name: "Acme Dock 9", Hubs: []string{"1234:0001"}}
	if p := ValidateDock(good, generic); len(p) > 0 {
		t.Errorf("a minimal good entry was rejected: %v", p)
	}
}

func TestValidateDocksCatchesCrossEntryProblems(t *testing.T) {
	f := &DocksFile{Docks: []Dock{
		{ID: "a", Name: "A", Hubs: []string{"1234:0001"}},
		{ID: "a", Name: "A again", Hubs: []string{"1234:0002"}},
		{ID: "b", Name: "B", Hubs: []string{"1234:0001"}},
	}}
	problems := ValidateDocks(f)
	if !anyContains(problems, "duplicate id") || !anyContains(problems, "also listed by") {
		t.Errorf("problems = %v", problems)
	}
}

func TestMergeDockUpdatesReplacesOrRefuses(t *testing.T) {
	f := &DocksFile{Docks: []Dock{{ID: "acme", Name: "Acme", Hubs: []string{"1234:0001", "1234:0002"}}}}

	if replaced, err := MergeDock(f, Dock{ID: "acme", Name: "Acme (fixed)", Hubs: []string{"1234:0001", "1234:0002", "1234:0003"}}); err != nil || replaced != "acme" {
		t.Fatalf("same id: replaced=%q err=%v", replaced, err)
	}
	if f.Docks[0].Name != "Acme (fixed)" || len(f.Docks) != 1 {
		t.Errorf("same-id update did not replace in place: %+v", f.Docks)
	}
	if replaced, err := MergeDock(f, Dock{ID: "acme-renamed", Name: "Acme", Hubs: []string{"1234:0003", "1234:0002", "1234:0001"}}); err != nil || replaced != "acme" {
		t.Errorf("same hubs, other id: replaced=%q err=%v", replaced, err)
	}
	if _, err := MergeDock(f, Dock{ID: "other", Name: "Other", Hubs: []string{"1234:0003", "5678:0001"}}); !errors.Is(err, ErrConflict) {
		t.Errorf("partial overlap: err = %v, want a conflict", err)
	}
	if _, err := MergeDock(f, Dock{ID: "new", Name: "New", Hubs: []string{"5678:0001"}}); err != nil || len(f.Docks) != 2 {
		t.Errorf("new dock: err=%v docks=%d", err, len(f.Docks))
	}
}

func TestSlugAndStripGeneric(t *testing.T) {
	if got := Slug("CalDigit, Inc. TS4"); got != "caldigit-inc-ts4" {
		t.Errorf("Slug = %q", got)
	}
	d := Dock{Hubs: []string{"8087:0b40", "1234:0001"}}
	StripGenericHubs(&d, map[string]string{"8087:0b40": "x"})
	if len(d.Hubs) != 1 || d.Hubs[0] != "1234:0001" {
		t.Errorf("hubs after strip = %v", d.Hubs)
	}
}

func anyContains(list []string, sub string) bool {
	for _, s := range list {
		if contains1(s, sub) {
			return true
		}
	}
	return false
}

func contains1(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
