# NIIMBOT N1 — a Label-Printin' Driver fer Linux

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**Other tongues:** [English](README.en.md) · [Русский](README.md) · [Nederlands](README.nl.md) · [简体中文](README.zh-CN.md)

> Avast! This here be a joke translation, matey. The commands, flags, codes an'
> numbers be true as the north star — only the talkin' be pirate. The proper
> README be [README.en.md](README.en.md).

A driver fer the **NIIMBOT N1** thermal-transfer label printer over Bluetooth LE.
Writ in Go, she works from the command line: prints text an' pictures, tells ye
the state o' the printer, an' can draw ye a mock-up without wastin' a single label.

No vendor app, no phone, no account — just Linux, Bluetooth an' one command.

```console
$ niimbot text "Pump" "12A-5"
```

## What She Can Do

| Command | What she does |
|---|---|
| `niimbot info` | the state o' the printer **an' the plunder inside her**: model id, serial number, firmware, battery, label type, how many labels be left on the roll an' how much ribbon |
| `niimbot rfid` | read the RFID tags o' the label roll an' the ribbon |
| `niimbot text "line" "..."` | print text; every line be its own argument, savvy? |
| `niimbot image file.png` | print a picture |
| `niimbot preview "line"` | draw the mock-up **without printin'** — see what would go on the label |
| `niimbot testpage` | the printer's own test page (to check the line be live) |
| `niimbot scan` | hunt fer the printer over the air |

## What Ye Need Aboard

- **Linux** with BlueZ (tried on Debian an' Ubuntu; the talk with BlueZ runs over D-Bus);
- **Go 1.24+** to build her;
- a font with Cyrillic letters (DejaVu, Noto or Liberation — she finds 'em herself);
- a **NIIMBOT N1** (other models want their own printhead numbers — see «The Code o' the Sea»).

## Gettin' Her Aboard

```bash
go install github.com/petrovich811/niimbot@latest
```

Or raise her from the timbers yerself:

```bash
git clone https://github.com/petrovich811/niimbot.git
cd niimbot
go build -o niimbot .
```

## Sailin' Her

```bash
# see what the printer reckons
./niimbot info

# draw the mock-up, waste no label (lands in /tmp/niimbot_label_preview.png)
./niimbot preview "Pump" "12A-5"

# print the words
./niimbot text "Pump" "12A-5"

# a picture
./niimbot image logo.png

# twice as dark, two copies
./niimbot text "GRK" "inv. 4210" --density 3 --copies 2
```

### Riggin'

| Flag | Meanin' | If ye say nothin' |
|---|---|---|
| `--address MAC` | the printer's address; without it she hunts fer the name `N1-` | auto |
| `--length mm` | how long the label be along the feed | `30` |
| `--font points` | the letter size; `0` fits the label herself | `0` |
| `--density 1..3` | how hard she burns | `2` |
| `--label` | the kind o' label; **leave it empty an' she'll read the roll's RFID tag** | auto |
| `--copies N` | how many copies | `1` |
| `--flip` | turn the whole lot 180° | off |
| `-v=false` | hold yer tongue about the protocol | on |

## Labels an' Which Way Up She Prints

### Kinds o' Labels

The NIIMBOT protocol knows eight kinds o' label, but **the N1 carries but five**
([the label types reference](https://printers.niim.blue/other/label-types/)):

| Code | Kind | N1 |
|---|---|---|
| 1 | with gaps — the ordinary ones (`withgaps`) | ✅ |
| 2 | black thermal (`black`) | ❌ |
| 3 | continuous, no gaps (`continuous`) | ✅ |
| 4 | perforated (`perforated`) | ❌ |
| 5 | transparent (`transparent`) | ✅ |
| 6 | PVC tags (`pvctag`) | ❌ |
| 10 | black mark gap (`blackmarkgap`) | ✅ |
| 11 | heat-shrink tube (`heatshrink`) | ✅ |

The kinds `black`, `perforated` an' `pvctag` sail in the protocol, but the N1's
firmware won't have 'em aboard. The driver checks the kind **afore she connects**
an' refuses to print if the model can't carry it.

Labels sold fer the N1 run up to **15 mm wide** (a 12 mm strip down the middle be
what she can actually burn): white matt 14×30, 14×40 an' 14×50 mm, matt silver an'
clear 14×30 mm, an' coloured cable labels 12.5×109 mm. She prints by thermal
transfer, so ye need a ribbon.

### Spyin' the Consumables

Every roll — labels an' ribbon alike — carries an **RFID tag**. The printer reads
it herself, an' the driver can ask:

```console
$ niimbot rfid

=== label roll ===
  UUID:                  881dcce946121080
  barcode:               12242117
  serial:                PC0H902384002378
  label type:            withgaps (1) — supported by N1
  resource:              228, used 43, left 185

=== ribbon ===
  UUID:                  881d8209fa900000
  serial:                PZ1GA06304000391
  resource:              1600, used 901, left 699
```

**She reads the label kind herself.** Leave `--label` off an' the driver reads the
roll's tag an' takes the kind from there: the printer knows what be loaded far
better than any guess. If the tag tells o' a kind the model can't carry, she won't
print at all. If ye name `--label` yerself an' it disagrees with the tag, she'll
warn ye:

```
warning: the printer holds withgaps labels, but printing as continuous
```

The tag keeps the label's size too, but the printer **never tells it to the host**
(the vendor app fetches it from a server by roll number), so ye still set the
length with `--length`.

### Which Way Up

The N1's printhead be **96 dots** wide — that be **12 mm** at 203 dpi. Here we
sail with **EW14×30** labels: 14 mm wide, 30 mm long along the feed. The burnable
strip be 12 mm down the middle.

**The words run along the label.** To make that so, the picture be drawn in
"readin' order": the picture's width be the label's length, the height be the
printhead's width. Then the picture becomes printer rows:

- **she transposes it** — the picture's X axis becomes the feed;
- **the printhead axis runs backwards** — head dot `c` reads picture row
  `across-1-c` (in NiimBlueLib that be `idx = (height-1-col)*width + row`).

Both o' these were tried against the real printer, an' both were the author's own
blunders:

1. draw it "as on the screen" an' the words run **across** the label;
2. leave the printhead axis unreversed an' the label comes out **mirrored**.

If yer label comes out upside down, add `--flip` — that turns the whole lot 180°.

## The Code o' the Sea

A packet:

```
55 55 | command | length | data | XOR checksum | AA AA
```

Numbers o' many bytes be **big-endian**. The checksum be the XOR o' the command,
the length an' every byte o' data.

The printin' run:

```
density (0x21) → label type (0x23) → print start (0x01) →
page start (0x03) → page size (0x13) → bitmap rows (0x85) →
page end (0xE3) → status polling (0xA3) → print end (0xF3)
```

A bitmap row: `position(2) | black-dot counters(3) | repeats(1) | data(12)`, where
bit 7 o' the first byte be the outermost dot o' the head. Rows white as bone sail
with their own command (`0x84`) — shorter an' quicker.

Print status (answer `0xB3`): `page(2) | print progress % | feed progress %`.
The printin' be done when the page be reached an' both progress marks read 100.

### Two Kraken in the Water

**1. The printer has two BLE services, an' only one o' them answers.**

| Service | What she be |
|---|---|
| `e7810a71-73ae-499d-8c15-faa9aef0c3f2` | **the NIIMBOT service herself** — the firmware listens here, characteristic `bef8d6c9-9c21-4c9e-b632-bd58c1009f9f` |
| `49535343-fe7d-4ae5-8fa9-9fafd205e455` | a plain UART — she matches the marks (`notify` + `write`), but **answers no command at all** |

Boardin' the UART looks like a fair wind, but the printer stays dumb as a post:
yer commands sink into the deep. That be where a sailor loses most o' his time.

**2. BlueZ must lay eyes on the device first.**

Without a scan aforehand, boardin' fails with a D-Bus curse:

```
Method "Get" with signature "ss" on interface "org.freedesktop.DBus.Properties" doesn't exist
```

One more thing fer Go sailors: `Adapter.Scan` o' the Linux backend o'
`tinygo.org/x/bluetooth` **blocks until `StopScan`** — run it in a goroutine o' her
own, or the ship gets stuck in the doldrums.

### Other Models

The numbers be the N1's own: model id `3586`, head `96` dots, density `1..3`.
Fer another model, change 'em in `protocol.go` (the «model → numbers» chart sails in
[NiimBlueLib](https://github.com/MultiMote/niimbluelib/blob/master/src/printer_models.ts)).

## Testin' Her

```bash
go test ./...
```

The tests check the buildin' o' packets **against bytes taken off the live
printer** (handshake `5555c10101c1aaaa`, model query `555540010849aaaa`), the
readin' o' answers, the splicin' o' half-arrived packets, the dot counters, the
way rows lie an' the white canvas behind 'em.

## What's Been Tried in Open Water

**Tried on a live N1 (GA24110447):**

- huntin', boardin' an' the handshake;
- model id `3586`, serial number, firmware `3.13`, battery;
- **printin' text** — the whole voyage: `page 1, print 100%, feed 100%`;
- **printin' a picture** — frame, words an' corner mark landed just as the mock-up said;
- **which way up** — the printout reads true: words along the label, not mirrored.
  Confirmed on paper after the two repairs told above;
- both blunders only showed their faces on paper — there be no other way to catch 'em.

**Not yet tried:**

- printin' on label kinds other than `withgaps`;
- other NIIMBOT models (they want their own printhead numbers).

## Thanks

The protocol were plundered from the open-source library
**[NiimBlueLib](https://github.com/MultiMote/niimbluelib)** (MIT) — the truest open
chart o' the NIIMBOT protocol. From her came the packet shape, the command codes,
the splittin' o' the dot counters an' the model numbers. The code o' this driver be
writ from bare timbers in Go.

## Fair Warnin'

This project sails under no flag o' NIIMBOT an' be not blessed by the makers.
«NIIMBOT» an' the model names belong to their rightful owners. The printer be
steered by a plundered protocol — sail at yer own risk.

## Licence

[MIT](LICENSE).
