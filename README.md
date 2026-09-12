# usb-device-kb

A knowledge base of USB dock and device info for community sharing and app
development. MIT licensed, so it can be embedded in anything.

It records what a USB device or dock cannot tell the operating system about
itself: which logical hubs make up one dock, what its printed ports are,
what a drive's link and throughput ceilings are, and how a USB4 device
identifies itself when it tunnels PCIe. An app that reads the USB tree can
use it to draw a dock as one box, judge a link against what the product can
do, and give a device its real name.

It began as the knowledge base of [Port Authority](https://github.com/harvey-withington/port-authority),
a USB and Thunderbolt insight app for Windows, and is kept separate so the
data stays open and reusable whatever happens to any one app.

## Files

| File | What it holds |
|---|---|
| `docks.json` | Docks: the logical hubs inside each, the strings its USB4 router announces, its uplink, its printed ports and verified port maps. |
| `devices.json` | Devices keyed by USB `vid:pid`: name, kind, the fastest link they can use, claimed throughput, and the USB4 router id they present when tunnelling PCIe. |
| `schema/*.schema.json` | JSON Schema for both files; point your editor at them. |
| `kbdata/` | A Go package that loads and validates the files, and merges a submitted entry. Import it to validate entries in your own app. |
| `cmd/validate` | Validates both files; what CI runs. |
| `cmd/submit` | Turns a submission issue into a change to `docks.json`; what the submission workflow runs. |

## Adding a dock

Every entry in `docks.json` has:

- `id`: a stable slug, `vendor-model` (`caldigit-ts4`).
- `name`: the product name people know.
- `hubs`: every logical hub the dock exposes, as lower-case `vid:pid`. On
  Windows a dock shows as a chain of hubs, often across two controllers,
  and listing all of them is what lets an app fold the chain into one box.
  Never list a chipset hub found inside many docks; those go in
  `generic_internal_hubs` and are folded into whichever dock they sit in.
- `usb4` (Thunderbolt and USB4 docks): the `vendor` and `model` strings the
  dock's own router announces from its DROM. Windows folds them into the
  router's name, `USB4 Router (1.0), <vendor> - <model>`. Record the
  strings, not the router's vid:pid, which is the bridge silicon shared by
  many docks.
- `uplink`: how the dock reaches the computer and the fastest link that
  can carry (`usb4_40` for a Thunderbolt 4 dock). Apps compare the link
  they actually got against this.
- `ports`: the printed ports, grouped by label and position, with each
  kind's maximum link and count.
- `port_map`: which logical hub port is which printed socket. Only entries
  checked on real hardware belong here; a guessed label is worse than none.
- `verified`: `hardware` when checked on a real dock, otherwise where the
  data came from and when.
- `notes`: anything the next person should know.

Link speeds are `low`, `full`, `high`, `ss5`, `ss10`, `ss20`, `usb4_20`,
`usb4_40`, `usb4_80`. Connectors are `usb-c`, `usb-a`, `internal`.

## Adding a device

Entries in `devices.json` are keyed by USB `vid:pid` and hold `name`,
`kind` (`storage`, `display`, `audio`, `video`, `hid`, `network`, …),
`max_link`, the maker's `claimed_read_mbps` and `claimed_write_mbps`, and
for USB4 products the `usb4_id` they present as a router when tunnelling
PCIe.

## Submitting

The easiest way is from Port Authority: set the dock up in the app and
press "Share with the community", which opens a pre-filled issue here. Or
open an issue with the "Add or update a dock" template and paste the
entry as JSON. A workflow validates it and opens a pull request; a
maintainer reviews and merges. Merged entries are published within
minutes (see below).

Submissions carry only the entry: hub and router ids, names, ports. Never
serial numbers.

## Validation

```
go test ./...
go run ./cmd/validate
```

The same checks run on every push and pull request. They cover well-formed
ids, unique dock ids, no hub claimed by two docks, no generic hub inside a
dock, known link speeds and connectors, and port maps that name hubs the
dock actually lists.

## Using the data in your app

Every merge to `main` republishes both files as assets of the rolling
`latest` release:

```
https://github.com/harvey-withington/usb-device-kb/releases/download/latest/docks.json
https://github.com/harvey-withington/usb-device-kb/releases/download/latest/devices.json
```

Fetch with `If-None-Match` and cache; the files change rarely. Ship a copy
inside your app for first run and offline use, and treat a fetched copy as
a newer version of the same thing.

## Sources

Entries marked `verified: hardware` were checked on the real dock. Others
name their source in `verified`: fwupd's dock firmware quirk files
(LGPL-2.1+; the ids are facts), published teardowns with `lsusb` output,
and so on. Please keep that habit.

## Licence

MIT. See `LICENSE`.
