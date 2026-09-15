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
$ niimbot text "1 gulden" "1734"
```

## Mogelijkheden

| Commando | Wat het doet |
|---|---|
| `niimbot info` | status van de printer **en de geplaatste verbruiksartikelen**: model-id, serienummer, firmware, accu, labeltype, resterend aantal etiketten en lint |
| `niimbot rfid` | de RFID-tags van de etikettenrol en het lint uitlezen |
| `niimbot text "regel" "..."` | tekst printen; elke regel tekst is een apart argument |
| `niimbot image bestand.png` | een afbeelding printen |
| `niimbot preview "regel"` | een proefontwerp maken **zonder te printen** |
| `niimbot preview --file img.png` | proefontwerp uit een afbeelding: wat inpassen en drempel overhouden |
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
./niimbot preview "1 gulden" "1734"

# tekst printen
./niimbot text "1 gulden" "1734"

# afbeelding
./niimbot image logo.png

# donkerder, twee exemplaren — een voor de flip, een voor de catalogus
./niimbot text "2 gulden 1785" "Utrecht" --density 3 --copies 2
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

Een `|` in een regel begint een nieuwe regel binnen één etiket: `1 gulden|1734` zet
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
West-Friesland;1 gulden 1734;XF
Holland;1 gulden 1762;VF
Utrecht;2 gulden 1785;UNC
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

### Een reeks uit Excel printen

1. Maak een tabel in Excel: **één rij is één etiket**, de kolommen zijn wat erop komt.
2. Sla hem op als **CSV**: Bestand → Opslaan als → CSV.
3. Open in de interface het tabblad «Reeks» en druk op **CSV laden** — de rijen komen in de lijst.
4. Stel een **etikeksjabloon** in met `{1}`, `{2}`… — dat bepaalt hoe het etiket eruitziet.
5. Bekijk het **proefontwerp** en druk op **Printen**.

Het vakje «eerste rij is een kopregel» laat de kop van de tabel weg. Aanhalingstekens
beschermen een scheidingsteken binnen een veld: `"KM# 90; Verkade 45.2";XF` zijn twee
kolommen, geen drie.

**Over de kolommen.** Er moeten er **minstens zoveel** zijn als het hoogste getal in
het sjabloon: een sjabloon met `{3}` vraagt drie kolommen. Excel vult lege cellen
zelf aan, dus `West-Friesland;;XF` zijn drie kolommen en het etiket houdt een **lege
regel** — zo ziet elke etiket in de reeks er hetzelfde uit. Extra kolommen worden
genegeerd. Zijn het er minder, dan **weigert de driver te printen** en zegt welke
ontbreekt, in plaats van een letterlijke `{3}` op het etiket te zetten.

**Kolomvolgorde.** Staan de kolommen in de tabel in een andere volgorde dan op het
etiket nodig is, dan hoeft Excel niet omgegooid te worden — zet gewoon de
kolomnummers van boven naar beneden, door komma's gescheiden:

```
4, 1, 2, 5
```

Dat betekent: de eerste regel van het etiket is kolom 4, de tweede 1, dan 2 en 5. Een
kolom die je weglaat (hier de derde) komt niet op het etiket, en dezelfde kolom mag
twee keer voorkomen. Het veld «Etikeksjabloon» vult zichzelf: `{4}`, `{1}`, `{2}`,
`{5}` — en dat kun je daarna aanvullen, bijvoorbeeld `TAG {2}`.

## Etiketten met een afbeelding

```bash
# kijk wat er geprint zou worden, zonder een etiket te verspillen
./niimbot preview --file schets.png

# printen
./niimbot image schets.png
```

Formaten: **PNG, JPEG, GIF**. Een afbeelding van elk formaat wordt **in het
etiketvlak gepast**, met behoud van verhoudingen, en op een wit veld gecentreerd —
pixels handmatig passen is niet nodig.

### Teken op de maat van het etiket

De printer werkt op **203 dpi**, dat is **8 punten per millimeter**. Het etiketvlak:

| Etiket | Formaat afbeelding |
|---|---|
| 14×30 mm | **240×96** punten |
| 14×40 mm | 320×96 |
| 14×50 mm | 400×96 |

De hoogte is altijd **96** punten — de breedte van de printkop (12 mm) — en de
breedte is de etiketlengte in millimeters maal 8.

**Inpassen werkt, maar tekenen op die maat is beter.** Teken je 900×360, dan krimpt
de tekst bijna viermaal en wordt hij wollig: de resolutie van het etiket is maar
240×96, er is geen ruimte over.

### Waarin tekenen

| Programma | Wanneer handig |
|---|---|
| **Inkscape** | het best voor etiketten: document van 240×96 pixels, vormen en tekst, export naar PNG |
| **GIMP** | als je een foto nodig hebt: schaal naar 8 pixels per millimeter, grijstinten |
| **LibreOffice Draw** | als een kantooreditor vertrouwder voelt en eenvoudige vormen volstaan |
| **ImageMagick** | vanaf de opdrachtregel: `convert bron.png -resize 240x96! etiket.png` |
| al het andere | één ding telt: een PNG van de juiste maat |

Er wordt **zwart-wit** geprint: grijs wordt zwart of wit volgens een drempel, dus
halftinten en dunne lijnen verdwijnen. Controleer met `preview --file` — die toont
precies wat de printer krijgt.

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
- **een reeks van meerdere etiketten als één opdracht** — drie etiketten achter
  elkaar, `printen voltooid (pagina 3, 100%)`;
- **kolomvolgorde** — op papier staan de regels precies in de gevraagde volgorde
  (`4, 1, 2, 5`), en een weggelaten kolom komt niet op het etiket;
- **een lege kolom houdt een lege regel** — elk etiket in de reeks heeft evenveel
  regels en de opmaak verschuift niet;
- **een afbeelding die op de maat van het etiket is getekend is scherper** dan
  dezelfde afbeelding verkleind uit een groot canvas. Vergeleken op papier:
  240×96 tegen 900×360 — de eerste is duidelijk beter. «Teken op die maat» is dus
  geen gril, het volgt uit 203 dpi;
- **afbeelding printen** — kader, tekst en hoekmarkering kwamen er net zo uit als
  in het proefontwerp;
- **oriëntatie** — de afdruk is goed leesbaar: tekst langs het etiket, niet
  gespiegeld. Bevestigd op papier na de twee correcties hierboven;
- beide oriëntatiefouten kwamen pas bij het printen aan het licht — zonder
  fysieke afdruk zijn ze niet te vinden.

**Nog niet geverifieerd:**

- printen op andere labeltypen dan `withgaps`;
- andere NIIMBOT-modellen (die hebben eigen printkopgegevens nodig).

## Bouwen voor andere systemen

De driver is in Go geschreven en bouwt dus voor Linux, Windows en macOS:

```bash
go build -o niimbot .                                  # je eigen systeem
GOOS=windows GOARCH=amd64 go build -o niimbot.exe .    # Windows, ook vanaf Linux
GOOS=darwin  GOARCH=arm64 go build -o niimbot-mac .    # macOS — alleen OP een Mac
```

| Systeem | Stand |
|---|---|
| **Linux** | werkt, getest op echte hardware |
| **Windows** | kruiscompileert ongewijzigd vanaf Linux; nog niet op echte hardware geprobeerd |
| **macOS** | **moet op een Mac gebouwd worden**: Bluetooth is daar CoreBluetooth en die binding vraagt CGO en Apple-frameworks. Kruiscompileren vanaf Linux kan niet |

**Een macOS-eigenaardigheid:** het systeem geeft nooit echte Bluetooth-adressen —
het levert een UUID in plaats van een MAC. Op een Mac wordt de printer dus via
scannen gevonden, en `--address` verwacht een UUID, geen MAC.

**Het lettertype** wordt automatisch gevonden: DejaVu/Noto/Liberation op Linux,
Arial/Segoe UI op Windows, Arial/Helvetica op macOS. Stel `NIIMBOT_FONT` in voor
een eigen lettertype.

## Verwante projecten

Staat er geen Linux-machine naast de printer, dan zijn er andere wegen:

| Project | Wat het biedt |
|---|---|
| [NiimBlueLib](https://github.com/MultiMote/niimbluelib) | de open NIIMBOT-protocollibrary (JS, MIT) — de kennis erachter zat ook in deze driver |
| [NiimBlue](https://niim.blue/) | een kant-en-klare webclient: printen uit de browser via Web Bluetooth, **zonder server**. Werkt in Chrome op Android; op de iPhone is de browser [Bluefy](https://apps.apple.com/us/app/bluefy-web-ble-browser/id1492822055) nodig |
| [NiimPrintX](https://github.com/labbots/NiimPrintX) | een Python-client met GUI; modellen D11, B21, B1, D110, B18 — de N1 staat er niet bij |

Deze driver onderscheidt zich doordat hij vanaf de Linux-opdrachtregel werkt en
**reeksen etiketten uit een CSV met een sjabloon** print.

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
