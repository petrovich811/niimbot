# NIIMBOT N1 — labelprinter-driver voor Linux

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**Andere talen:** [English](README.en.md) · [Русский](README.md) · [简体中文](README.zh-CN.md) · [Pirate 🏴‍☠️](README.pirate.md)

Een driver voor de thermotransfer-labelprinter **NIIMBOT N1** via Bluetooth LE.
Geschreven in Go, werkt vanaf de opdrachtregel: tekst en afbeeldingen printen,
de status van de printer tonen, en een proefontwerp maken zonder te printen.

Geen app van de fabrikant, geen telefoon, geen account — alleen Linux, Bluetooth
en één commando.

```console
$ niimbot text "Pomp" "12A-5"
```

## Mogelijkheden

| Commando | Wat het doet |
|---|---|
| `niimbot info` | status van de printer **en de geplaatste verbruiksartikelen**: model-id, serienummer, firmware, accu, labeltype, resterend aantal etiketten en lint |
| `niimbot rfid` | de RFID-tags van de etikettenrol en het lint uitlezen |
| `niimbot text "regel" "..."` | tekst printen; elke regel tekst is een apart argument |
| `niimbot image bestand.png` | een afbeelding printen |
| `niimbot preview "regel"` | een proefontwerp maken **zonder te printen** |
| `niimbot testpage` | de ingebouwde testpagina van de printer |
| `niimbot gui` | grafische interface in de browser: **reeksen etiketten**, sjablonen, proefontwerp |
| `niimbot scan` | de printer in de buurt zoeken |

## Vereisten

- **Linux** met BlueZ (getest op Debian en Ubuntu; de communicatie met BlueZ loopt via D-Bus);
- **Go 1.24+** om te bouwen;
- een lettertype met Cyrillische tekens (DejaVu, Noto of Liberation — wordt automatisch gezocht);
- een **NIIMBOT N1** (voor andere modellen zijn andere printkopgegevens nodig — zie «Protocol»).

## Installatie

```bash
go install github.com/petrovich811/niimbot@latest
```

Of vanuit de broncode:

```bash
git clone https://github.com/petrovich811/niimbot.git
cd niimbot
go build -o niimbot .
```

## Gebruik

```bash
# status van de printer
./niimbot info

# proefontwerp zonder printen (komt in /tmp/niimbot_label_preview.png)
./niimbot preview "Pomp" "12A-5"

# tekst printen
./niimbot text "Pomp" "12A-5"

# afbeelding
./niimbot image logo.png

# tweemaal zo donker, twee exemplaren
./niimbot text "GRK" "inv. 4210" --density 3 --copies 2
```

### Opties

| Optie | Betekenis | Standaard |
|---|---|---|
| `--address MAC` | adres van de printer; zonder dit wordt op naam `N1-` gezocht | auto |
| `--length mm` | lengte van het etiket in de doorvoerrichting | `30` |
| `--font punten` | korps; `0` past zich aan het etiket aan | `0` |
| `--density 1..3` | printdichtheid | `2` |
| `--label` | labeltype; **leeg betekent: bepalen via de RFID-tag van de rol** | auto |
| `--copies N` | aantal exemplaren | `1` |
| `--flip` | inhoud 180° draaien | uit |
| `-v=false` | geen uitgebreide protocoluitvoer | aan |

## Grafische interface

```bash
./niimbot gui              # serveert http://127.0.0.1:8765 en opent de browser
./niimbot gui --port 9000  # een andere poort
./niimbot gui --no-browser # alleen de server
```

De interface zit **in de binary zelf** (`embed` + `net/http`) — geen GUI-framework,
geen extra afhankelijkheden. De printlogica wordt niet gedupliceerd: de interface
roept dezelfde functies aan als de opdrachtregel.

| Wat er is | Waarvoor |
|---|---|
| **Tab «Reeks»** | een lijst etiketten, één per regel; als één opdracht achter elkaar geprint |
| **CSV laden** | de eerste kolom wordt gebruikt (sla Excel op als CSV) |
| **Proefontwerp** | het ontwerp van het etiket vóór het printen |
| **Sjablonen** | bewaarde sets instellingen voor vaste taken: kabel-etiketten, apparatuurplaatjes |
| **Status** | model, serienummer, accu, labeltype, rest van rol en lint |
| **Voortgang** | «N van M geprint» tijdens een reeks |

Een `|` in een regel begint een nieuwe regel binnen één etiket: `Pomp|12A-5` zet
twee regels op het etiket. Sjablonen staan in `~/.config/niimbot/templates.json`.

### Een reeks etiketten met meerdere regels: sjabloon + gegevens

Als elk etiket in een reeks uit meerdere regels moet bestaan, gebruik dan een
**etikeksjabloon** en een **gegevenstabel** — zoals echte etiketsoftware werkt.

**Sjabloon:**

```
{1}
{2}
{3}
```

**Gegevens** (één record per regel, kolommen met `;`):

```
Centrifugaalpomp;12A-5;14.09.2026
Schuifafsluiter;12B-1;14.09.2026
Regelklep;12B-2;14.09.2026
```

Dat print **drie etiketten van elk drie regels**: `{1}`, `{2}`, `{3}` zijn de
kolommen van het record.

| Regel | Hoe het werkt |
|---|---|
| Sjabloonregels | elke regel van het sjabloon wordt één regel op het etiket; lege regels vervallen |
| Kolommen | gescheiden door `;`, `,` of een tab — wat het eerst komt |
| Kolom hergebruiken | `TAG {2}` zet een kolom waar dan ook in de regel |
| Zonder sjabloon | één dataregel is één etiket, en `\|` erin breekt de regel |

Een CSV uit Excel werkt dus direct: bestand laden, sjabloon instellen, en de hele
tabel gaat als reeks naar de printer.

## Etiketten en oriëntatie

### Labeltypen

Het NIIMBOT-protocol kent acht labeltypen, maar **de N1 ondersteunt er vijf**
([overzicht van de typen](https://printers.niim.blue/other/label-types/)):

| Code | Type | N1 |
|---|---|---|
| 1 | met tussenruimte — de gewone etiketten (`withgaps`) | ✅ |
| 2 | zwart, thermisch (`black`) | ❌ |
| 3 | doorlopende band (`continuous`) | ✅ |
| 4 | geperforeerd (`perforated`) | ❌ |
| 5 | transparant (`transparent`) | ✅ |
| 6 | pvc-labels (`pvctag`) | ❌ |
| 10 | met zwarte markering (`blackmarkgap`) | ✅ |
| 11 | krimpkous (`heatshrink`) | ✅ |

De typen `black`, `perforated` en `pvctag` bestaan in het protocol, maar de
firmware van de N1 ondersteunt ze niet. De driver controleert het type **vóór het
verbinden** en weigert te printen als het model het niet ondersteunt.

Voor de N1 zijn etiketten tot **15 mm breed** te koop (het bedrukbare deel is
12 mm in het midden): wit mat 14×30, 14×40 en 14×50 mm, mat zilver en transparant
14×30 mm, en gekleurde kabel-etiketten van 12,5×109 mm. Het printen gebeurt
thermisch met overdracht, dus er is een lint (ribbon) nodig.

### Verbruiksartikelen automatisch herkennen

Elke rol — etiketten én lint — heeft een **RFID-tag**. De printer leest die zelf,
en de driver kan het vragen:

```console
$ niimbot rfid

=== etikettenrol ===
  UUID:                  881dcce946121080
  barcode:               12242117
  serienummer:           PC0H902384002378
  labeltype:             withgaps (1) — ondersteund door de N1
  voorraad:              228, gebruikt 43, over 185

=== lint (ribbon) ===
  UUID:                  881d8209fa900000
  serienummer:           PZ1GA06304000391
  voorraad:              1600, gebruikt 901, over 699
```

**Het labeltype wordt automatisch bepaald.** Als `--label` niet is opgegeven,
leest de driver de tag van de rol en neemt het type daaruit: de printer weet beter
dan welke gok dan ook wat erin zit. Ondersteunt het model het type uit de tag niet,
dan begint het printen niet. Geeft u `--label` wel op en wijkt het af van de tag,
dan waarschuwt de driver:

```
let op: de printer bevat etiketten van het type withgaps, maar er wordt geprint als continuous
```

De tag bewaart ook de afmetingen van het etiket, maar de printer **stuurt die niet
naar de host** (de app van de fabrikant haalt de maten op bij een server aan de hand
van het rolletnummer). Daarom stelt u de lengte nog steeds in met `--length`.

### Oriëntatie

De printkop van de N1 is **96 punten** breed — dat is **12 mm** bij 203 dpi.
Hier worden etiketten **EW14×30** gebruikt: 14 mm breed en 30 mm lang in de
doorvoerrichting. Het bedrukbare deel is 12 mm in het midden van het etiket.

**De tekst loopt langs het etiket.** Daarvoor wordt de inhoud in
«leesoriëntatie» getekend: de breedte van het beeld is de lengte van het etiket,
de hoogte is de breedte van de printkop. Daarna wordt het beeld omgezet in
printerregels:

- **het beeld wordt getransponeerd** — de X-as wordt de doorvoerrichting;
- **de as van de printkop loopt omgekeerd** — printkoppunt `c` leest beeldregel
  `across-1-c` (in NiimBlueLib is dat `idx = (height-1-col)*width + row`).

Beide details zijn met proefprints op de echte printer bevestigd, en beide waren
fouten van de auteur:

1. tekent u de inhoud «zoals op het scherm», dan loopt de tekst **dwars** op het etiket;
2. zonder de as van de printkop om te keren komt het etiket er **gespiegeld** uit.

Komt het etiket ondersteboven uit, voeg dan `--flip` toe — dat is een draaiing
van 180°.

## Protocol

Een pakket:

```
55 55 | commando | lengte | gegevens | XOR-controle | AA AA
```

Getallen van meerdere bytes zijn **big-endian**. De controlesom is de XOR van het
commando, de lengte en alle gegevensbytes.

De printcyclus:

```
dichtheid (0x21) → labeltype (0x23) → start printen (0x01) →
start pagina (0x03) → paginagrootte (0x13) → bitmapregels (0x85) →
einde pagina (0xE3) → status opvragen (0xA3) → einde printen (0xF3)
```

Een bitmapregel: `positie(2) | tellers van zwarte punten(3) | herhalingen(1) | gegevens(12)`,
waarbij bit 7 van de eerste byte de uiterste punt van de printkop is. Volledig
witte regels gaan met een apart commando (`0x84`) — dat is korter en sneller.

Printstatus (antwoord `0xB3`): `pagina(2) | voortgang printen % | voortgang doorvoer %`.
Het printen is klaar als beide percentages 100 zijn.

### Twee subtiliteiten bij het printen van reeksen

Beide gevonden op echte hardware, en beide kostten verspilde etiketten:

**1. Tussen bitmapregels is een pauze nodig** (hier 4 ms). `writeWithoutResponse`
duwt gegevens sneller dan de printer ze kan verwerken, en hij antwoordt met een
gegevensfout: antwoord `0xDB`, code `6`. Zonder de pauze mislukt het tweede etiket
van een reeks al.

**2. Elk etiket moet wachten tot de vorige pagina klaar is met printen.** Terwijl
de printer nog print, geldt het volgende etiket als extra gegevens en antwoordt
hij weer `0xDB`/`6`. De driver pollt de status en wacht op twee opeenvolgende
metingen van «100 % printen en doorvoer» voordat het volgende etiket gaat.

Het helpt ook dat **fout `0xDB` leesbaar is**: de driver vertaalt de code (klep
open, geen papier, oververhitting, geen lint enzovoort) en stopt de opdracht in
plaats van stil op een timeout te wachten.

### Twee valkuilen

**1. De printer heeft twee BLE-services, en er werkt er maar één.**

| Service | Wat het is |
|---|---|
| `e7810a71-73ae-499d-8c15-faa9aef0c3f2` | **de eigen NIIMBOT-service** — hier luistert de firmware, karakteristiek `bef8d6c9-9c21-4c9e-b632-bd58c1009f9f` |
| `49535343-fe7d-4ae5-8fa9-9fafd205e455` | een transparante UART — voldoet aan de eigenschappen (`notify` + `write`), maar **antwoordt helemaal niet op commando's** |

Verbinden met de UART lijkt te lukken, maar de printer blijft stil: de commando's
verdwijnen in het niets. Hier gaat bij het debuggen de meeste tijd naartoe.

**2. BlueZ moet het apparaat eerst «gezien» hebben.**

Zonder voorafgaande scan mislukt het verbinden met een D-Bus-fout:

```
Method "Get" with signature "ss" on interface "org.freedesktop.DBus.Properties" doesn't exist
```

Nog een belangrijk detail voor Go: `Adapter.Scan` van de Linux-backend van
`tinygo.org/x/bluetooth` **blokkeert tot `StopScan`** — start het in een aparte
goroutine, anders loopt het programma vast.

### Andere modellen

De gegevens horen bij de N1: model-id `3586`, printkop `96` punten, dichtheid `1..3`.
Voor een ander model past u die waarden aan in `protocol.go` (de tabel
«model → gegevens» staat in
[NiimBlueLib](https://github.com/MultiMote/niimbluelib/blob/master/src/printer_models.ts)).

## Tests

```bash
go test ./...
```

De tests controleren het samenstellen van pakketten **aan de hand van bytes die van
de echte printer zijn opgenomen** (handshake `5555c10101c1aaaa`, modelvraag
`555540010849aaaa`), het uitlezen van antwoorden, het samenvoegen van onvolledige
pakketten, de punttellers, de oriëntatie van regels en de witte achtergrond.

## Wat is geverifieerd en wat niet

**Geverifieerd op een echte N1 (GA24110447):**

- zoeken, verbinden en handshake;
- model-id `3586`, serienummer, firmware `3.13`, accu;
- **tekst printen** — volledige cyclus: `pagina 1, printen 100%, doorvoer 100%`;
- **afbeelding printen** — kader, tekst en hoekmarkering kwamen er net zo uit als
  in het proefontwerp;
- **oriëntatie** — de afdruk is goed leesbaar: tekst langs het etiket, niet
  gespiegeld. Bevestigd op papier na de twee correcties hierboven;
- beide oriëntatiefouten kwamen pas bij het printen aan het licht — zonder
  fysieke afdruk zijn ze niet te vinden.

**Nog niet geverifieerd:**

- printen op andere labeltypen dan `withgaps`;
- andere NIIMBOT-modellen (die hebben eigen printkopgegevens nodig).

## Dank

Het protocol is achterhaald met behulp van de open-sourcebibliotheek
**[NiimBlueLib](https://github.com/MultiMote/niimbluelib)** (MIT) — de nauwkeurigste
open implementatie van het NIIMBOT-protocol. Daaruit komen het pakketformaat, de
commandocodes, de verdeling van de punttellers en de modelgegevens.
De code van deze driver is vanaf nul in Go geschreven.

## Disclaimer

Dit project is niet verbonden met NIIMBOT en wordt niet door de fabrikant
ondersteund. «NIIMBOT» en modelnamen zijn eigendom van hun respectievelijke
eigenaren. De printer wordt via een achterhaald protocol aangestuurd — gebruik op
eigen risico.

## Licentie

[MIT](LICENSE).
