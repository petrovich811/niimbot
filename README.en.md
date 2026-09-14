# NIIMBOT N1 — label printer driver for Linux

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**Other languages:** [Русский](README.md) · [Nederlands](README.nl.md) · [简体中文](README.zh-CN.md) · [Pirate 🏴‍☠️](README.pirate.md)

A Bluetooth LE driver for the **NIIMBOT N1** thermal-transfer label printer,
written in Go. Prints text and images from the command line, reports printer
status, and can render a preview without wasting a label.

No vendor app, no phone, no account — just Linux, Bluetooth and one command.

```console
$ niimbot text "1 gulden" "1734"
```

## Features

| Command | Description |
|---|---|
| `niimbot info` | printer status **and installed consumables**: model id, serial number, firmware, battery, label type, labels left in the roll, ribbon left |
| `niimbot rfid` | read the RFID tags of the label roll and the ribbon |
| `niimbot text "line" "..."` | print text (each argument is a separate line) |
| `niimbot image file.png` | print an image |
| `niimbot preview "line"` | render the label **without printing** |
| `niimbot testpage` | built-in printer test page (connection check) |
| `niimbot gui` | browser UI: **label series**, templates, live preview |
| `niimbot scan` | discover the printer over Bluetooth |

## Requirements

- **Linux** with BlueZ (tested on Debian/Ubuntu; uses the BlueZ D-Bus API);
- **Go 1.24+** to build;
- a font with Cyrillic support (DejaVu, Noto or Liberation — auto-detected);
- a **NIIMBOT N1** printer (other models need their own printhead parameters — see "Protocol").

## Install

```bash
go install github.com/petrovich811/niimbot@latest
```

Or from source:

```bash
git clone https://github.com/petrovich811/niimbot.git
cd niimbot
go build -o niimbot .
```

## Usage

```bash
# printer status
./niimbot info

# render only (saved to /tmp/niimbot_label_preview.png)
./niimbot preview "1 gulden" "1734"

# print text
./niimbot text "1 gulden" "1734"

# print an image
./niimbot image logo.png

# darker, two copies
./niimbot text "TAG" "inv. 4210" --density 3 --copies 2
```

### Flags

| Flag | Meaning | Default |
|---|---|---|
| `--address MAC` | printer address; otherwise discovers by name `N1-` | auto |
| `--length mm` | label length along the feed direction | `30` |
| `--font px` | font size; `0` fits the label automatically | `0` |
| `--density 1..3` | print density | `2` |
| `--label` | label type; **empty means auto-detect from the roll's RFID tag** | auto |
| `--copies N` | number of copies | `1` |
| `--flip` | rotate content by 180° | off |
| `-v=false` | quiet protocol log | on |

## Graphical interface

```bash
./niimbot gui              # serves http://127.0.0.1:8765 and opens the browser
./niimbot gui --port 9000  # another port
./niimbot gui --no-browser # server only
```

The interface is **embedded in the binary** (`embed` + `net/http`) — no GUI
framework, no extra dependencies. Printing logic is not duplicated: the UI calls
the same functions the command line does.

| What it offers | Why |
|---|---|
| **"Series" tab** | a list of labels, one per line; printed as a single job, back to back |
| **CSV upload** | takes the first column (save Excel as CSV) |
| **Preview** | the label mock-up before printing |
| **Templates** | saved setting sets for typical jobs: cable labels, equipment tags |
| **Status** | model, serial, battery, label type, labels and ribbon left |
| **Progress** | "printed N of M" while a series runs |

A `|` in a line starts a new line inside one label: `1 gulden|1734` puts two lines
on the label. Templates live in `~/.config/niimbot/templates.json`.

### A series of multi-line labels: template + data

When every label in a series needs several lines, use a **label template** and a
**data table** — the way real label software does it.

**Template** (the "Label template" field):

```
{1}
{2}
{3}
```

**Data** (one record per line, columns separated by `;`):

```
West Frisia;1 gulden 1734;XF
Holland;1 gulden 1762;VF
Utrecht;2 gulden 1785;UNC
```

That prints **three labels with three lines each**: `{1}`, `{2}`, `{3}` are the
record's columns.

| Rule | How it works |
|---|---|
| Template lines | every template line becomes one line on the label; trailing empty lines are dropped |
| Columns | separated by `;`, `,` or a tab — whichever comes first |
| Reusing a column | `TAG {2}` puts a column anywhere in the line |
| Without a template | one data line is one label, and `\|` inside it breaks the line |

A CSV exported from Excel therefore works as-is: upload it, set the template, and
the whole table prints as a series.

### Printing a series from Excel

1. Build a table in Excel: **one row is one label**, the columns are what lands on it.
2. Save it as **CSV**: File → Save As → CSV.
3. In the UI open the "Series" tab and press **Load CSV** — the rows land in the list.
4. Set a **label template** with `{1}`, `{2}`… — that is what the label looks like.
5. Check the **mock-up** and press **Print**.

The "first row is a header" checkbox drops the table's header. Quotes protect a
separator inside a field: `"KM# 90; Verkade 45.2";XF` is two columns, not three.

**About the columns.** There must be **at least as many** as the highest number in
the template: a template using `{3}` needs three columns. Excel pads empty cells
itself, so `West Frisia;;XF` is three columns, and the label keeps an **empty
line** — every label in the series then has the same layout. Extra columns are
ignored. If there are fewer, the driver **refuses to print** and says which one is
missing, instead of printing a literal `{3}` on the label.

**Column order.** If the table's columns are not in the order you want on the label,
there is no need to rearrange Excel — just list the column numbers top to bottom,
comma-separated:

```
4, 1, 2, 5
```

That means: the label's first line is column 4, the second is 1, then 2 and 5. A
column you leave out (the third here) never reaches the label, and the same column
may be listed twice. The template field fills itself in — `{4}`, `{1}`, `{2}`, `{5}` —
and you can then add to it, e.g. `TAG {2}`.

## Labels and orientation

### Label types

The NIIMBOT protocol knows eight label types, but the **N1 supports five**
([label types reference](https://printers.niim.blue/other/label-types/)):

| ID | Type | N1 |
|---|---|---|
| 1 | with gaps — the usual labels (`withgaps`) | ✅ |
| 2 | black thermal (`black`) | ❌ |
| 3 | continuous, no gaps (`continuous`) | ✅ |
| 4 | perforated (`perforated`) | ❌ |
| 5 | transparent (`transparent`) | ✅ |
| 6 | PVC tags (`pvctag`) | ❌ |
| 10 | black mark gap (`blackmarkgap`) | ✅ |
| 11 | heat-shrink tube (`heatshrink`) | ✅ |

The types `black`, `perforated` and `pvctag` exist in the protocol, but the N1
firmware does not support them. The driver checks the type **before connecting**
and refuses to print if the model does not support it.

Labels sold for the N1 are up to **15 mm wide** (12 mm printable strip down the
centre): white matt 14×30, 14×40 and 14×50 mm, matt silver and clear 14×30 mm,
and coloured cable labels 12.5×109 mm. Printing is thermal transfer, so a ribbon
is required.

### Consumable auto-detection

Every roll — labels and ribbon alike — carries an **RFID tag**. The printer reads
it on its own, and the driver can ask:

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

**The label type is detected automatically.** When `--label` is omitted, the
driver reads the roll's tag and takes the type from there: the printer knows
what is loaded far better than any guess. If the tag reports a type the model
does not support, printing does not start. If `--label` is given explicitly and
disagrees with the tag, the driver warns:

```
warning: the printer holds withgaps labels, but printing as continuous
```

The tag also stores the label dimensions, but the printer **never sends them to
the host** (the vendor app fetches them from a server by roll number), so the
length is still set with the `--length` flag.

### Orientation

The N1 printhead is **96 dots** wide — **12 mm** at 203 dpi. This project uses
**EW14×30** labels: 14 mm wide, 30 mm long along the feed direction. The
printable area is a 12 mm strip down the centre of the label.

**Text runs along the label.** To get that, the content is drawn in "reading
orientation": the image width is the label length, the height is the printhead
width. That image is then turned into printer rows:

- **transposed** — the image's X axis becomes the feed direction;
- **the printhead axis is reversed** — head dot `c` reads image row `across-1-c`
  (in NiimBlueLib that is `idx = (height-1-col)*width + row`).

Both details were confirmed by printing on real hardware, and both were the
author's mistakes:

1. drawing the content "as on screen" makes the text run **across** the label;
2. without reversing the printhead axis the label comes out **mirrored**.

If the label prints upside down, add `--flip` — that rotates the content 180°.

## Protocol

Packet layout:

```
55 55 | command | length | data | XOR checksum | AA AA
```

Multi-byte integers are **big-endian**. The checksum is the XOR of the command,
the length and all data bytes.

Print sequence:

```
density (0x21) → label type (0x23) → print start (0x01) →
page start (0x03) → page size (0x13) → bitmap rows (0x85) →
page end (0xE3) → status polling (0xA3) → print end (0xF3)
```

A bitmap row is `position(2) | black-pixel counters(3) | repeats(1) | data(12)`,
where bit 7 of the first byte is the leftmost printhead dot. Fully blank rows
are sent with a dedicated command (`0x84`) — shorter and faster.

Print status (response `0xB3`): `page(2) | print progress % | feed progress %`.
Printing is done when both progress values are 100.

### Two subtleties of series printing

Both were found on real hardware, and both cost wasted labels:

**1. Bitmap rows need a pause between them** (4 ms here). `writeWithoutResponse`
pushes data faster than the printer can take it, and it answers with a data error:
response `0xDB`, code `6`. Without the pause the second label of a series already fails.

**2. Each label must wait for the previous page to finish printing.** While the
printer is still printing, the next label counts as extra data and it answers
`0xDB`/`6` again. The driver polls the status and waits for two consecutive
readings of "100% print and feed" before sending the next label.

It also helps that **error `0xDB` is readable**: the driver decodes the code
(cover open, no paper, overheat, no ribbon and so on) and stops the job instead
of silently waiting for a timeout.

### Two traps worth knowing

**1. The printer exposes two BLE services, and only one of them works.**

| Service | What it is |
|---|---|
| `e7810a71-73ae-499d-8c15-faa9aef0c3f2` | **native NIIMBOT service** — the firmware listens here; characteristic `bef8d6c9-9c21-4c9e-b632-bd58c1009f9f` |
| `49535343-fe7d-4ae5-8fa9-9fafd205e455` | a transparent UART — matches the expected properties (`notify` + `write`) but **never answers any command** |

Connecting to the UART looks successful while the printer stays completely
silent. This is where most debugging time goes.

**2. BlueZ must have seen the device first.**

Without a prior scan, connecting fails with a D-Bus error:

```
Method "Get" with signature "ss" on interface "org.freedesktop.DBus.Properties" doesn't exist
```

Also note: with the Linux backend of `tinygo.org/x/bluetooth`,
`Adapter.Scan` **blocks until `StopScan`** — run it in a separate goroutine or
the program will hang.

### Other models

Parameters are hard-coded for the N1: model id `3586`, printhead `96` dots,
density `1..3`. For another model, change these in `protocol.go` — the
model-to-parameters table lives in
[NiimBlueLib](https://github.com/MultiMote/niimbluelib/blob/master/src/printer_models.ts).

## Tests

```bash
go test ./...
```

The tests verify packet construction **against bytes captured from a real
printer** (handshake `5555c10101c1aaaa`, model query `555540010849aaaa`),
response parsing, partial-packet reassembly, pixel counters, row orientation
and the white canvas background.

## Verified vs. not verified

**Verified on a real N1 (GA24110447):**

- discovery, connection and handshake;
- model id `3586`, serial number, firmware `3.13`, battery;
- **text printing** — full cycle: `page 1, print 100%, feed 100%`;
- **image printing** — a frame, text and a corner marker landed exactly as
  in the preview;
- **orientation** — the printout reads correctly: text along the label, not
  mirrored. Confirmed on paper after the two fixes described above;
- both orientation bugs showed up only on paper — there is no other way to
  catch them.

**Not verified yet:**

- printing on label types other than `withgaps`;
- other NIIMBOT models (they need their own printhead parameters).

## Building for other systems

The driver is written in Go, so it builds for Linux, Windows and macOS:

```bash
go build -o niimbot .                                  # your own system
GOOS=windows GOARCH=amd64 go build -o niimbot.exe .    # Windows, from Linux too
GOOS=darwin  GOARCH=arm64 go build -o niimbot-mac .    # macOS — ON a Mac only
```

| System | State |
|---|---|
| **Linux** | works, verified on real hardware |
| **Windows** | cross-compiles from Linux unchanged; not yet tried on real hardware |
| **macOS** | **must be built on a Mac**: Bluetooth there is CoreBluetooth, and its binding needs CGO and Apple frameworks. Cross-building from Linux is impossible |

**A macOS quirk:** the system never reveals real Bluetooth addresses — it hands out
a UUID instead of a MAC. So on a Mac the printer is found by scanning, and
`--address` takes a UUID, not a MAC.

**The font** is found automatically: DejaVu/Noto/Liberation on Linux, Arial/Segoe UI
on Windows, Arial/Helvetica on macOS. Set `NIIMBOT_FONT` to use your own.

## Related projects

If there is no Linux box next to the printer, there are other roads:

| Project | What it gives |
|---|---|
| [NiimBlueLib](https://github.com/MultiMote/niimbluelib) | the open NIIMBOT protocol library (JS, MIT) — this driver is built on the knowledge from it |
| [NiimBlue](https://niim.blue/) | a ready web client: print from the browser over Web Bluetooth, **no server**. Works in Chrome on Android; on iPhone you need the [Bluefy](https://apps.apple.com/us/app/bluefy-web-ble-browser/id1492822055) browser |
| [NiimPrintX](https://github.com/labbots/NiimPrintX) | a Python client with a GUI; models D11, B21, B1, D110, B18 — the N1 is not on its list |

This driver stands apart by working from the Linux command line and by printing
**series of labels from a CSV with a template**.

## Acknowledgements

The protocol was reverse-engineered from the open-source
**[NiimBlueLib](https://github.com/MultiMote/niimbluelib)** (MIT) — the most
accurate open implementation of the NIIMBOT protocol. Packet layout, command
codes, pixel-counter splitting and model parameters come from there.
The code in this driver was written from scratch in Go.

## Disclaimer

This project is not affiliated with or endorsed by NIIMBOT. "NIIMBOT" and model
names belong to their respective owners. The printer is driven over a
reverse-engineered protocol — use at your own risk.

## License

[MIT](LICENSE).
