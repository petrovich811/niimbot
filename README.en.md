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
| `niimbot preview --file img.png` | mock-up from a picture: what survives fitting and thresholding |
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

# darker, two copies — one for the flip, one for the catalogue
./niimbot text "2 gulden 1785" "Utrecht" --density 3 --copies 2
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

## Label designer

Open it from the **"Label designer"** link in the UI header, or directly:
`http://127.0.0.1:8765/designer.html`.

A label is built from **elements** — text and pictures, each with its own
coordinates in millimetres. Coordinates follow the reading orientation:

- **X** — along the label (the feed), 0 on the left;
- **Y** — across the label (the printhead), 0 on top, 12 mm in total.

| What you can do | How |
|---|---|
| Move elements | drag them; coordinates snap to 0.5 mm |
| Set exact values | the X, Y, width and height fields on the right |
| Text | text with `{1}`, `{2}`… fields, size in mm, left/centre/right alignment |
| A picture | "+ Picture": the file is embedded into the template |
| See the result | the "Refresh mock-up" button — a real print-out sits beside the canvas |
| Print a series | a list of labels (pasted or from CSV) and "Print series" |
| Save | "Save" in the "Template" section |

**The template stays a single file**: pictures are embedded as `data:URL`, so it can
be carried to another computer whole.

The canvas is ruled in millimetres: thin lines are 1 mm, bold ones 5 mm, with a
ruler on top. The "1 mm grid" checkbox hides the ruling.

Remember that **1 mm is 8 dots**. A 14×30 label holds only 240×96 dots, so four
lines larger than 2.5 mm will not fit.

### Font and line height

Every text element has its own font: **family**, **weight** and **line height**.

| Field | What it sets |
|---|---|
| **Font** | a system family: "Arial", "DejaVu Sans", "Times New Roman", "Comic Sans MS"… |
| **Size, mm** | letter height |
| **Line height, mm** | spacing between lines inside one element; `0` follows the font metrics |
| **Bold** | a bold face, when the family has one |

The font list comes **from the system**: Linux (`/usr/share/fonts`, `~/.fonts`,
`~/.local/share/fonts`) and Windows (`C:\Windows\Fonts`, plus the current user's
fonts). The drop-down lists only families **with Cyrillic**, since labels are almost
always Russian text. `/api/fonts` returns the full list.

A template stores the **family name**, not a file path: it survives being carried to
another machine where the same font lives elsewhere.

If the named font is missing, the driver **refuses to print** and says which one is
absent. Silently substituting another is not allowed: the label would come out
different from the intent, and only a human eye on paper would notice.

### Lines, frames and rotation

| Element | What it gives |
|---|---|
| **Line** | a filled bar: a separator between lines, or a solid block |
| **Frame** | an outline around the edges, with a settable thickness (0.3 mm by default) |
| **Text rotation** | `0°`, `90°`, `180°`, `270°` — for cable labels where text runs across |

For a rotated element the width (`w`) is the **length of the text**, not a box width:
after rotation it becomes the block's height. The designer's canvas shows the
rotation honestly: at 90° and 270° the width and height swap.

**Edge snapping.** While dragging, an element is pulled not only to the 0.5 mm grid
but also to the label's **edges and centre** — red guide lines appear on the canvas.
Hold **Alt** to switch edge snapping off when you want to place something by eye.

### A template as data

For those working through the API, a template reads plainly without the designer:

```json
{
  "length": 30,
  "elements": [
    {"kind": "text", "x": 1, "y": 0.5, "w": 13, "text": "lot {4}", "font": 2.2, "align": "left"},
    {"kind": "text", "x": 16, "y": 0.5, "w": 13, "text": "{5}", "font": 2.2, "align": "right"},
    {"kind": "text", "x": 1, "y": 3.2, "w": 28, "text": "{1}", "font": 2.6},
    {"kind": "image", "x": 20, "y": 3, "w": 8, "h": 8, "image": "data:image/png;base64,…"}
  ]
}
```

`/api/preview` and `/api/print` accept the same list in the `elements` field.

## Labels with a picture

```bash
# see what would print, without wasting a label
./niimbot preview --file sketch.png

# print it
./niimbot image sketch.png
```

Formats: **PNG, JPEG, GIF**. An image of any size is **fitted** into the label area
whole, keeping its proportions, and centred on a white field — there is no need to
match pixels by hand.

### Draw at the label's own size

The printer runs at **203 dpi**, which is **8 dots per millimetre**. The label area:

| Label | Image size |
|---|---|
| 14×30 mm | **240×96** dots |
| 14×40 mm | 320×96 |
| 14×50 mm | 400×96 |

The height is always **96** dots — that is the printhead's width (12 mm) — and the
width is the label length in millimetres times 8.

**Fitting works, but drawing at that size is better.** Draw 900×360 and the text
shrinks almost fourfold and turns woolly: the label's resolution is only 240×96,
and there is no headroom.

### What to draw in

| Program | When it suits |
|---|---|
| **Inkscape** | best for labels: a 240×96 pixel document, shapes and text, export to PNG |
| **GIMP** | when you need a photograph: scale to 8 pixels per millimetre, greyscale |
| **LibreOffice Draw** | when an office editor feels more familiar and simple shapes suffice |
| **ImageMagick** | from the command line: `convert source.png -resize 240x96! label.png` |
| anything else | one thing matters: a PNG of the right size |

Printing is **black and white**: grey becomes black or white by a threshold, so
halftones and thin lines vanish. Check with `preview --file` — it shows exactly what
the printer will get.

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
- **a series of several labels as one job** — three labels printed back to back,
  `print finished (page 3, 100%)`;
- **column order** — on paper the lines follow exactly the order asked for
  (`4, 1, 2, 5`), and a column left out never reaches the label;
- **an empty column keeps an empty line** — every label in the series has the same
  number of lines and the layout does not shift;
- **the label designer** — a series built from placed elements printed: text lands at
  its coordinates and alignment works;
- **font and line height** — four fonts (Arial, Times New Roman, Comic Sans MS,
  DejaVu Sans) are distinguishable on paper, and a 4 mm line height visibly spreads
  the lines;
- **a frame, a divider and a 90° text rotation** — printed together on one label:
  the frame is whole on all four sides, the divider is straight, and the rotated text
  reads when the head is tilted;
- **print density** — all three levels (1, 2 and 3) give a usable print, including a
  0.2 mm hairline and 1.6 mm small text; **2** was judged the best, which is why it is
  the default;
- **a picture drawn at the label's own size is crisper** than the same picture
  shrunk from a large canvas. Compared on paper: 240×96 against 900×360 — the
  first is clearly better. So "draw at that size" is not fussiness, it follows
  from 203 dpi;
- the orientation and connection bugs surfaced only on real hardware: neither the
  tests nor the mock-up showed them.

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
