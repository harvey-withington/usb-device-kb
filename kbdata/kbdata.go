// Package kbdata loads, validates and merges the usb-device-kb files.
//
// It is what CI runs on every change, what the submission workflow uses
// to turn an issue into a pull request, and what an app can import to
// validate an entry a user typed before saving or sharing it. It has no
// dependencies, so importing it costs nothing.
package kbdata

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// LinkSpeeds are the link names the files may use, slowest first.
var LinkSpeeds = []string{"unknown", "none", "low", "full", "high", "ss5", "ss10", "ss20", "usb4_20", "usb4_40", "usb4_80"}

// Connectors are the socket kinds a port entry may name.
var Connectors = []string{"usb-c", "usb-a", "internal", "unknown"}

// DeviceKinds are the classes a device entry may claim.
var DeviceKinds = []string{
	"storage", "video", "audio", "hid", "network", "hub", "display", "wireless",
	"printer", "serial", "imaging", "composite", "vendor", "unknown", "billboard", "smartcard",
}

// DocksFile is docks.json.
type DocksFile struct {
	Comment             string            `json:"$comment,omitempty"`
	GenericInternalHubs map[string]string `json:"generic_internal_hubs,omitempty"`
	Docks               []Dock            `json:"docks"`
}

// Dock is one entry in docks.json.
type Dock struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Hubs     []string   `json:"hubs"`
	Uplink   *Uplink    `json:"uplink,omitempty"`
	USB4     *USB4      `json:"usb4,omitempty"`
	Ports    []Port     `json:"ports,omitempty"`
	PortMap  []PortMap  `json:"port_map,omitempty"`
	Verified string     `json:"verified,omitempty"`
	Notes    string     `json:"notes,omitempty"`
}

// Uplink is how a dock reaches the computer.
type Uplink struct {
	Kind    string `json:"kind"`
	MaxLink string `json:"max_link"`
}

// USB4 is the vendor and model strings a dock's own router announces.
type USB4 struct {
	Vendor string `json:"vendor"`
	Model  string `json:"model"`
	Notes  string `json:"notes,omitempty"`
}

// Port is one kind of printed port on a dock.
type Port struct {
	Label     string `json:"label"`
	Position  string `json:"position"`
	Connector string `json:"connector"`
	MaxLink   string `json:"max_link"`
	Count     int    `json:"count"`
	Notes     string `json:"notes,omitempty"`
}

// PortMap pins a logical hub port to a printed socket.
type PortMap struct {
	Hub      string `json:"hub"`
	Port     int    `json:"port"`
	Label    string `json:"label"`
	Position string `json:"position"`
	Verified string `json:"verified,omitempty"`
}

// DevicesFile is devices.json.
type DevicesFile struct {
	Comment string            `json:"$comment,omitempty"`
	Devices map[string]Device `json:"devices"`
}

// Device is one entry in devices.json.
type Device struct {
	Name             string `json:"name"`
	Kind             string `json:"kind"`
	MaxLink          string `json:"max_link"`
	ClaimedReadMBps  int    `json:"claimed_read_mbps,omitempty"`
	ClaimedWriteMBps int    `json:"claimed_write_mbps,omitempty"`
	USB4ID           string `json:"usb4_id,omitempty"`
	Notes            string `json:"notes,omitempty"`
}

var (
	pairRe = regexp.MustCompile(`^[0-9a-f]{4}:[0-9a-f]{4}$`)
	slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

// LoadDocks reads docks.json strictly: an unknown key is an error, since
// a typo that parsed would be a rule that silently never applied.
func LoadDocks(path string) (*DocksFile, error) {
	var f DocksFile
	if err := loadStrict(path, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// LoadDevices reads devices.json strictly.
func LoadDevices(path string) (*DevicesFile, error) {
	var f DevicesFile
	if err := loadStrict(path, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

func loadStrict(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// ValidateDocks reports every problem with the file, in order. An empty
// list means the file is good.
func ValidateDocks(f *DocksFile) []string {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }

	for hub := range f.GenericInternalHubs {
		if !pairRe.MatchString(hub) {
			add("generic_internal_hubs: %q is not lower-case vid:pid", hub)
		}
	}
	ids := map[string]bool{}
	owner := map[string]string{}
	for i, d := range f.Docks {
		where := fmt.Sprintf("docks[%d] (%s)", i, firstNonEmpty(d.ID, d.Name, "unnamed"))
		if ids[d.ID] {
			add("%s: duplicate id %q", where, d.ID)
		}
		ids[d.ID] = true
		for _, p := range ValidateDock(d, f.GenericInternalHubs) {
			add("%s: %s", where, p)
		}
		for _, hub := range d.Hubs {
			if other, taken := owner[hub]; taken && other != d.ID {
				add("%s: hub %s is also listed by %s", where, hub, other)
			}
			owner[hub] = d.ID
		}
	}
	return problems
}

// ValidateDock checks one entry on its own: shape, ids, speeds, and that
// it claims no generic hub. Cross-entry checks are ValidateDocks' job.
func ValidateDock(d Dock, generic map[string]string) []string {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }

	if d.ID == "" {
		add("id is required")
	} else if !slugRe.MatchString(d.ID) && !strings.HasPrefix(d.ID, "local:") {
		add("id %q must be a lower-case slug like vendor-model", d.ID)
	}
	if strings.TrimSpace(d.Name) == "" {
		add("name is required")
	}
	if len(d.Hubs) == 0 {
		add("hubs must list at least one hub")
	}
	seen := map[string]bool{}
	for _, hub := range d.Hubs {
		switch {
		case !pairRe.MatchString(hub):
			add("hub %q is not lower-case vid:pid", hub)
		case seen[hub]:
			add("hub %s is listed twice", hub)
		case generic[hub] != "":
			add("hub %s is a generic chipset hub (%s) and must not be claimed by a dock", hub, generic[hub])
		}
		seen[hub] = true
	}
	if d.Uplink != nil {
		if strings.TrimSpace(d.Uplink.Kind) == "" {
			add("uplink.kind is required when uplink is given")
		}
		if !isLinkSpeed(d.Uplink.MaxLink) {
			add("uplink.max_link %q is not a link speed", d.Uplink.MaxLink)
		}
	}
	if d.USB4 != nil {
		if strings.TrimSpace(d.USB4.Vendor) == "" || strings.TrimSpace(d.USB4.Model) == "" {
			add("usb4 needs both vendor and model")
		}
		if pairRe.MatchString(strings.ToLower(d.USB4.Model)) {
			add("usb4.model %q looks like a vid:pid; record the DROM model string, not the router id", d.USB4.Model)
		}
	}
	for i, p := range d.Ports {
		if strings.TrimSpace(p.Label) == "" {
			add("ports[%d]: label is required", i)
		}
		if !contains(Connectors, p.Connector) {
			add("ports[%d]: connector %q is not one of %s", i, p.Connector, strings.Join(Connectors, ", "))
		}
		if !isLinkSpeed(p.MaxLink) {
			add("ports[%d]: max_link %q is not a link speed", i, p.MaxLink)
		}
		if p.Count < 1 {
			add("ports[%d]: count must be at least 1", i)
		}
	}
	for i, m := range d.PortMap {
		if !seen[m.Hub] {
			add("port_map[%d]: hub %s is not one of the dock's hubs", i, m.Hub)
		}
		if m.Port < 1 {
			add("port_map[%d]: port must be at least 1", i)
		}
		if strings.TrimSpace(m.Label) == "" {
			add("port_map[%d]: label is required", i)
		}
		if strings.TrimSpace(m.Verified) == "" {
			add("port_map[%d]: verified is required; port maps are only accepted from real hardware", i)
		}
	}
	return problems
}

// ValidateDevices reports every problem with devices.json.
func ValidateDevices(f *DevicesFile) []string {
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }
	keys := make([]string, 0, len(f.Devices))
	for k := range f.Devices {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	usb4 := map[string]string{}
	for _, k := range keys {
		d := f.Devices[k]
		if !pairRe.MatchString(k) {
			add("devices: key %q is not lower-case vid:pid", k)
		}
		if strings.TrimSpace(d.Name) == "" {
			add("devices[%s]: name is required", k)
		}
		if !contains(DeviceKinds, d.Kind) {
			add("devices[%s]: kind %q is not one of %s", k, d.Kind, strings.Join(DeviceKinds, ", "))
		}
		if !isLinkSpeed(d.MaxLink) {
			add("devices[%s]: max_link %q is not a link speed", k, d.MaxLink)
		}
		if d.ClaimedReadMBps < 0 || d.ClaimedWriteMBps < 0 {
			add("devices[%s]: claimed throughput cannot be negative", k)
		}
		if d.USB4ID != "" {
			if !pairRe.MatchString(d.USB4ID) {
				add("devices[%s]: usb4_id %q is not lower-case vid:pid", k, d.USB4ID)
			} else if other, taken := usb4[d.USB4ID]; taken {
				add("devices[%s]: usb4_id %s is also used by %s", k, d.USB4ID, other)
			}
			usb4[d.USB4ID] = k
		}
	}
	return problems
}

// ErrConflict is returned by MergeDock when the entry claims a hub that
// another dock lists, which a person has to settle.
var ErrConflict = errors.New("hub conflict")

// MergeDock adds an entry to the file or replaces the one it updates.
// An entry updates an existing dock when it has the same id, or lists
// exactly the same hubs. An entry that shares only some hubs with another
// dock is a conflict and is not merged.
func MergeDock(f *DocksFile, d Dock) (replaced string, err error) {
	wanted := hubSet(d.Hubs)
	for i, existing := range f.Docks {
		if existing.ID == d.ID || sameSet(hubSet(existing.Hubs), wanted) {
			f.Docks[i] = d
			return existing.ID, nil
		}
	}
	for _, existing := range f.Docks {
		for _, hub := range existing.Hubs {
			if wanted[hub] {
				return "", fmt.Errorf("%w: hub %s is listed by %s", ErrConflict, hub, existing.ID)
			}
		}
	}
	f.Docks = append(f.Docks, d)
	return "", nil
}

// StripGenericHubs drops chipset hubs from an entry, since they fold into
// whichever dock they sit in already and must not be claimed by one.
func StripGenericHubs(d *Dock, generic map[string]string) {
	kept := d.Hubs[:0:0]
	for _, hub := range d.Hubs {
		if generic[hub] == "" {
			kept = append(kept, hub)
		}
	}
	d.Hubs = kept
}

// Slug makes an id from a name: "CalDigit TS4" -> "caldigit-ts4".
func Slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		default:
			if b.Len() > 0 && !dash {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

// WriteDocks writes the file the way the repo keeps it: two-space indent,
// trailing newline.
func WriteDocks(path string, f *DocksFile) error {
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func hubSet(hubs []string) map[string]bool {
	out := map[string]bool{}
	for _, h := range hubs {
		out[h] = true
	}
	return out
}

func sameSet(a, b map[string]bool) bool {
	if len(a) != len(b) || len(a) == 0 {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func isLinkSpeed(s string) bool { return contains(LinkSpeeds, s) }

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
