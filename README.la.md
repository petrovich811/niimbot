# NIIMBOT N1 — rector pittaciorum pro Linux

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**Aliae linguae:** [English](README.en.md) · [Русский](README.md) · [Nederlands](README.nl.md) · [简体中文](README.zh-CN.md) · [Pirate 🏴‍☠️](README.pirate.md)

Rector machinae imprimendi pittacia **NIIMBOT N1** per Bluetooth LE. Lingua **Go**
scriptus est, in linea mandatorum laborat: litteras et imagines imprimit, statum
machinae et consumptibilium monstrat, formam ante impressionem pingere potest.

Nullum programma venditoris, nullus telephonus, nulla ratio — solum Linux,
Bluetooth et unum mandatum.

```console
$ niimbot text "1 гульден" "1734"
```

## Facultates

| Mandatum | Quid facit |
|---|---|
| `niimbot info` | status machinae **et consumptibilium**: numerus exemplaris, numerus serialis, firmware, onus, typus pittacii, quot pittacia in volumine et in taenia restant |
| `niimbot rfid` | vestigia RFID voluminis pittaciorum et taeniae legere |
| `niimbot text "linea" "..."` | litteras imprimere; quaeque linea suum argumentum est |
| `niimbot image file.png` | imaginem imprimere |
| `niimbot preview "linea"` | formam pingere **sine impressione** |
| `niimbot preview --file imago.png` | formam e imagine pingere: quid post mensuram et limen restat |
| `niimbot gui` | interfacies graphica in navigatro: **series pittaciorum**, formae, praevisum |
| `niimbot testpage` | pagina probatoria machinae |
| `niimbot scan` | machinam per aera quaerere |

## Requisita

- **Linux** cum BlueZ (expertum in Debian et Ubuntu; communicatio per D-Bus fit);
- **Go 1.24+** ad aedificandum;
- typus litterarum cum litteris Cyrillicis (DejaVu, Noto aut Liberation — quaeruntur ipsae);
- machina **NIIMBOT N1** (alia exemplaria suis parametris capitis indigent).

## Institutio

```bash
go install github.com/petrovich811/niimbot@latest
```

Aut e fonte:

```bash
git clone https://github.com/petrovich811/niimbot.git
cd niimbot
go build -o niimbot .
```

## Usus

```bash
# statum machinae videre
./niimbot info

# forma sine impressione (in /tmp/niimbot_label_preview.png servatur)
./niimbot preview "1 гульден" "1734"

# litteras imprimere
./niimbot text "1 гульден" "1734"

# imago
./niimbot image logo.png

# densius et duo exemplaria — unum pro theca, alterum pro catalogo
./niimbot text "2 гульдена 1785" "Утрехт" --density 3 --copies 2
```

### Optiones

| Optio | Significatio | Defectu |
|---|---|---|
| `--address MAC` | inscriptio machinae; sine ea quaeritur nomen `N1-` | automaticum |
| `--length mm` | longitudo pittacii secundum directionem | `30` |
| `--font puncta` | magnitudo; `0` — apta pittacio eligitur | `0` |
| `--density 1..3` | densitas impressionis | `2` |
| `--label` | typus pittacii; **vacuum — e vestigio voluminis legitur** | automaticum |
| `--copies N` | quot exemplaria | `1` |
| `--threshold 1..255` | limen nigri-albi; maius — densius | `200` |
| `--flip` | contentum 180° vertere | non |
| `-v=false` | sine verbosa protocolli narratione | verbosa |

## Interfacies graphica

```bash
./niimbot gui              # http://127.0.0.1:8765 et navigatrum aperit
./niimbot gui --port 9000  # alius portus
./niimbot gui --no-browser # solum servitor
```

Interfacies **in ipso programmate inclusa** est (`embed` + `net/http`) — nullae
bibliothecae graphicae, nullae dependentiae externae. Logica impressionis non
duplicatur: interfacies easdem functiones vocat quas linea mandatorum.

| Quid praesto est | Ad quid |
|---|---|
| **Series** | index pittaciorum, unum in linea; uno mandato imprimuntur |
| **CSV** | columnae in formam substituuntur |
| **Praevisum** | forma ante impressionem |
| **Formae** | collectanea optionum pro operibus solitis |
| **Status** | exemplar, onus, typus pittacii, quantum restat |

### De serie pittaciorum multilinearium: forma et data

Si quodque pittacium seriei ex pluribus lineis constare debet, adhibe **formam
pittacii** et **tabulam datorum**.

**Forma:**

```
{1}
{2}
{3}
```

**Data** (unum testimonium in linea, columnae per `;`):

```
Зап. Фрисландия;1 гульден 1734;XF
Голландия;1 гульден 1762;VF
Утрехт;2 гульдена 1785;UNC
```

Imprimentur **tria pittacia, quodque tribus lineis**: `{1}`, `{2}`, `{3}` columnae
testimonii sunt.

| Regula | Quomodo |
|---|---|
| Lineae formae | quaeque linea formae fit linea pittacii; vacuae in fine abiciuntur |
| Columnae | separator `;`, `,` aut tabula |
| Columna repetita | `TAG {2}` — columna usquam in linea poni potest |
| Sine forma | una linea datorum = unum pittacium; `\|` intra eam lineam frangit |

### De serie ex Excel imprimenda

1. In Excel tabulam fac: **una linea — unum pittacium**, columnae — quid in pittacio erit.
2. Serva ut **CSV** (Fasciculus → Servare ut → CSV).
3. In interfacie sectionem «Series» aperi et **CSV impone** — lineae in indicem veniunt.
4. Formam pittacii cum columnis `{1}`, `{2}`… constitue — ea pittacii speciem definit.
5. Praevisum vide et **imprime**.

Nota «prima linea — tituli» caput tabulae abicit. **De columnis:** tot columnas
habeas necesse est, quot maximus numerus in forma postulat — forma cum `{3}` tribus
columnis eget. Excel cellas vacuas ipse supplet, ita `Зап. Фрисландия;;XF` tres
columnae sunt, et **linea vacua** in pittacio manet: dispositio omnium pittaciorum
seriei eadem est. Si columnae desunt, rector **imprimere renuet** et dicet quae desit —
potius quam `{3}` litteris in pittacio imprimat.

**Ordo columnarum.** Si columnae alio ordine sunt, quam in pittacio necesse est,
numeros columnarum deorsum scribe, comma separatos:

```
4, 1, 2, 5
```

Hoc significat: prima linea pittacii columna 4, secunda 1, deinde 2 et 5. Columna
quam non nominas in pittacium non venit; eadem columna bis nominari potest. Campus
formae ipse suppletur et postea augeri potest.

## Constructor pittaciorum

Aperi per nexum **«Constructor pittaciorum»** in capite interfaciei, aut directe:
`http://127.0.0.1:8765/designer.html`.

Pittacium e **elementis** componitur — litteris et imaginibus, quibus suis
coordinata in millimetris sunt. Coordinata secundum directionem legendi currunt:

- **X** — secundum pittacium (directionem), 0 a sinistra;
- **Y** — trans pittacium (caput), 0 a summo, totum 12 mm.

| Quid fieri potest | Quomodo |
|---|---|
| Elementa movere | trahe mure; coordinata ad 0,5 mm adhaerescunt, ad margines et medium etiam |
| Accurate ponere | campi X, Y, latitudo, altitudo |
| Litterae | textus cum campis `{1}`, `{2}`…, magnitudo in mm, directio |
| Imago | «+ Imago»: fasciculus in formam includitur |
| Eventum videre | «Formam renovare» — iuxta tabulam vera impressio |
| Seriem imprimere | index pittaciorum e CSV |
| Servare | «Servare» in sectione «Forma» |

Tabula in millimetris divisa est: lineae tenues 1 mm, crassae 5 mm, supra regula.
**Memoria tene: 1 mm = 8 puncta.** In pittacio 14×30 sunt tantum 240×96 puncta,
ita quattuor lineae maius quam 2,5 mm non intrant.

### Typus litterarum et altitudo lineae

| Campus | Quid definit |
|---|---|
| **Typus** | familia e systematibus: «Arial», «DejaVu Sans», «Times New Roman»… |
| **Magnitudo, mm** | altitudo litterarum |
| **Altitudo lineae, mm** | spatium inter lineas; `0` — e metrica typi |
| **Crassus** | litterae crassae, si familia eas habet |
| **Automaticum** | magnitudo vacua — apta mensurae pittacii eligitur |

Si campus magnitudinis vacus relinquitur, rector **ipse maximum typum eligit**,
quo textus intra mensuram intrat. Hoc unam formam idoneam facit nominibus diversae
longitudinis: nomen breve maius exit, longum minus, sed semper integrum.

Index typorum **e systemate** sumitur: Linux (`/usr/share/fonts`, `~/.fonts`,
`~/.local/share/fonts`) et Windows (`C:\Windows\Fonts`). In indice sunt solum
familiae **cum litteris Cyrillicis**. In forma **nomen familiae** servatur, non via
fasciculi: ita forma in aliam machinam transferri potest. Si familia nominata in
systemate non est, rector **imprimere renuet** et eam nominabit — subductio tacita
pittacium aliter quam voluisti redderet.

### Lineae, formae, rotatio

| Elementum | Quid dat |
|---|---|
| **Linea** | fascia solida: divisor inter lineas aut tabula |
| **Forma** | circumductus per margines, crassitudine definita (0,3 mm) |
| **Rotatio** | `0°`, `90°`, `180°`, `270°` — pro pittaciis funalibus, ubi textus trans pittacium currit |

Apud elementum rotatum latitudo (`w`) est **longitudo textus**, non latitudo
formae: post rotationem altitudo fit.

### Structurae chemicae e SMILES

```json
{"kind": "smiles", "x": 0.6, "y": 1.2, "w": 9.5, "h": 9.5,
 "smiles": "CC(=CCCC(C)(C=C)O)C", "thickness": 2}
```

SMILES e data sumi potest: `"smiles": "{4}"` — structura e quarta columna legitur,
et quaeque series suam structuram accipit.

#### Quid nostrum sit, quid alienum

Structurae **solum locus** sunt, ubi rector codice alieno nititur:

| Rector noster facit (Go) | RDKit facit (Python) |
|---|---|
| elementum `smiles` in forma | solutionem catenae SMILES |
| mensuram in mm, rotationem, crassitudinem vinculorum | dispositionem 2D: cyclos, coordinata, ne secentur |
| substitutionem columnae: `"smiles": "{4}"` | picturam moleculae in PNG |
| memoriam: una molecula semel in serie pingitur | |
| inventionem Pythonis cum RDKit, errores claros | |

**Cur ipsi non scribimus.** Solutio SMILES dimidium operis est et hebdomas fere.
Difficultas vera est **dispositio 2D**: ubi cycli sint, quomodo atomi ponantur ne
vincula secent neve concurrant. In hoc RDKit et Open Babel annos consumpserunt, et
nostra implementatio peior exiret.

**Licentiae.** Nucleus RDKit **BSD 3-Clause** est, fasciculus PyPI `rdkit` **MIT**.
Ambae licentiae permittentes sunt et cum nostra MIT congruunt. RDKit **in programmate
non includitur**: separatim instituitur, ita licentia rectoris non tangitur.

#### Institutio RDKit

```bash
python3 -m venv ~/.local/share/niimbot/chemvenv
~/.local/share/niimbot/chemvenv/bin/pip install rdkit
```

Iuribus radicis non opus est. Rector Pythonem hoc ordine quaerit: variabile
`NIIMBOT_RDKIT`, directorium `chemvenv` iuxta programma, `~/.local/share/niimbot/chemvenv`,
deinde `python3` systematis.

#### De mensura sciendum

In pittacio 14×30 sunt tantum **240×96 puncta**. Structuram **statim debita mensura**
pingere oportet et **vinculis crassis**: imago magna ad 96 puncta redacta in puncta
dilabitur. Expertum:

| Molecula | Atomi graves | In quadrato 96×96 | Per totam latitudinem |
|---|---|---|---|
| ванилин, кумарин | 9 | legitur | legitur |
| линалоол, геранiol | 10–11 | legitur | legitur |
| мускус-кетон | 15 | legitur | legitur |
| холестерин | 28 | vix conicitur | **legitur** |

Moleculas magnas **tota pittacii latitudo** (230 puncta pro 96) servat. Notae
heteroatomorum (O, N) ad 8 puncta in millimetrum pereunt; stereochemia non distinguitur.

#### Rotatio structurae

Altitudo pittacii 12 mm est, ita structura quadrata in eam incurrit. Rotatio hoc
solvit: elementum campum `rotate` habet, et structura rotata **totam longitudinem
pittacii** occupare potest. **RDKit ipse rotat** (optio eius `rotate`): moleculam sub
formam datam disponit, non in quadratum includit deinde vertit.

**Sed modus est, et in charta stat.** In pittacio 14×30 structura magna et textus
magnus simul non intrant: structura ad 18×11 mm rotata crescit, sed columna textus
ita angustatur ut nomen abscidatur. Mansit **structura 15×10 mm, textus 14 mm** —
et structura magna et textus integer.

#### Forma seriei parata

In optionibus forma **«Парфюмерные ингредиенты»** servatur. Data est CSV:

```
название;CAS;SMILES;нота
ЛИНАЛООЛ;78-70-6;CC(=CCCC(C)(C=C)O)C;цветочный, свежий
```

| In pittacio | Unde |
|---|---|
| structura | columna 3 (SMILES) |
| nomen | columna 1, crassum |
| CAS | columna 2, semper |
| nota | columna 4 |
| dies et hora | momento impressionis substituuntur |

### Dies et hora impressionis

In quolibet textu campi substituuntur **momento impressionis**:

| Campus | Quid dat |
|---|---|
| `{дата}` | `18.09.2026` |
| `{время}` | `00:25` |
| `{дата-время}` | `18.09.2026 00:25` |

### Limen nigri-albi

Machina **solum nigrum et album** imprimit, itaque imago grisea per limen
convertitur. Defectu **200** ex 255. Lineae cum anti-aliasing pinguntur et margines
earum grisei sunt; ad limen 128 vincula structurarum **punctata** exibant et numeri
in numero CAS in puncta dilabebantur. Expertum in charta:

| Limen | Quid exit |
|---|---|
| 128 | vincula rumpuntur, textus parvus dilabitur |
| **200** | lineae solidae, textus integer — defectu |
| 230 et ultra | litterae cohaerescere incipiunt |

Mutatur tribus modis: optione `--threshold 1..255`, campo «Limen nigri-albi» in
constructore, campo `threshold` in `/api/preview` et `/api/print`.

**Forma in constructore iam nigra-alba monstratur** — talis qualis ad machinam ibit,
non grisea ut antea. Ita lineae pallidae ante impressionem videntur.

### Quomodo formam in constructore servare

1. Pittacium in tabula compone — litteras, imagines, lineas, structuras.
2. In sectione **«Forma»** nomen scribe, e.g. `Драхма Парфия`.
3. Preme **«Formam servare»**; sub buttonibus confirmatio apparet.

Forma et **elementa** (quid ubi sit, quo typo et magnitudine) et **optiones
impressionis** servat: longitudinem, densitatem, typum pittacii, exemplaria, limen.
Ad eam redire — elige e indice **«Formam servatam legere»**.

| Button | Quid facit |
|---|---|
| **Formam servare** | tabulam et optiones sub nomine scripto servat |
| **Delere** | formam in indice electam delet |
| **Tabulam purgare** | omnia elementa removet |

Formae in `~/.config/niimbot/templates.json` ut JSON iacent — in aliam machinam
transferri aut manu corrigi possunt.

### Forma ut data

```json
{
  "length": 30,
  "elements": [
    {"kind": "text", "x": 1, "y": 0.5, "w": 13, "text": "лот {4}", "font": 2.2, "align": "left"},
    {"kind": "text", "x": 16, "y": 0.5, "w": 13, "text": "{5}", "font": 2.2, "align": "right"},
    {"kind": "text", "x": 1, "y": 3.2, "w": 28, "text": "{1}", "font": 2.6},
    {"kind": "image", "x": 20, "y": 3, "w": 8, "h": 8, "image": "data:image/png;base64,…"}
  ]
}
```

Eundem indicem `/api/preview` et `/api/print` in campo `elements` accipiunt.

## Pittacium cum imagine

```bash
./niimbot preview --file imago.png   # forma sine impressione, pittacio non consumpto
./niimbot image imago.png            # imprimere
```

Formae: **PNG, JPEG, GIF**. Imago cuiusvis mensurae in aream pittacii **tota
includitur**, proportionibus servatis, et in campo albo centratur — nihil manu
aptandum est.

### Pingas statim ad mensuram pittacii

Machina **203 dpi** imprimit, id est **8 puncta in millimetrum**. Aream pittacii:

| Pittacium | Mensura imaginis |
|---|---|
| 14×30 mm | **240×96** puncta |
| 14×40 mm | 320×96 |
| 14×50 mm | 400×96 |

Altitudo semper **96** puncta est — latitudo capitis (12 mm); latitudo autem est
longitudo pittacii in millimetris octies sumpta.

**Inclusio laborat, sed statim hac mensura pingere melius est.** Si 900×360 pingeris,
textus fere quater minuitur et rarus fit: in pittacio sunt tantum 240×96 puncta.

### In quo pingere

| Programma | Quando commodum |
|---|---|
| **Inkscape** | optimum pro pittaciis: documentum 240×96 punctorum, formae et litterae, exportatio in PNG |
| **GIMP** | si photographia necessaria est: 8 puncta in millimetrum, toni grisei |
| **LibreOffice Draw** | si editor officii familiarior est et formae simplices sufficiunt |
| **ImageMagick** | e linea mandatorum: `convert fons.png -resize 240x96! pittacium.png` |

Impressio **nigra-alba** est: griseum per limen in nigrum aut album abit, ita toni
medii et lineae tenues pereunt. Formam proba per `preview --file`.

## Pittacia et directio

### Species pittaciorum

NIIMBOT octo typos pittaciorum novit, sed **N1 quinque sustinet**:

| Codex | Typus | N1 |
|---|---|---|
| 1 | cum intervallis — solita (`withgaps`) | ✅ |
| 2 | nigra thermica (`black`) | ❌ |
| 3 | continua (`continuous`) | ✅ |
| 4 | perforata (`perforated`) | ❌ |
| 5 | pellucida (`transparent`) | ✅ |
| 6 | pittacia PVC (`pvctag`) | ❌ |
| 10 | cum nota nigra (`blackmarkgap`) | ✅ |
| 11 | tubus thermocontractilis (`heatshrink`) | ✅ |

Typus `black`, `perforated` et `pvctag` in protocollo sunt, sed firmware N1 eos non
sustinet. Rector typum **ante conexionem** inspicit et imprimere renuet, indicem
typorum qui sustinentur addens.

### Consumptibilia automatice cognita

Quodque volumen — pittaciorum et taeniae — **vestigium RFID** habet, quod machina
ipsa legit. Rector `niimbot rfid` vestigium utrumque monstrat.

**Typus pittacii automatice cognoscitur.** Si `--label` non datur, rector vestigium
voluminis legit et typum inde sumit. Typus quem exemplar non sustinet impressionem
arcet; typus explicitus a vestigio discrepans monitum parit.

Longitudinem vestigium quoque servat, sed machina eam **hospiti non tradit**
(programma venditoris mensuras e servitore per numerum voluminis petit), ita
longitudo per `--length` datur.

### Directio

**Caput machinae 96 puncta latum est** — 12 mm ad 203 dpi. Itaque in pittacio 14×30
sunt 240×96 puncta. Litterae secundum pittacium currunt: ad hoc contentum in
«directione legendi» pingitur (latitudo = longitudo pittacii, altitudo = latitudo
capitis) et deinde transponitur, **axe capitis verso**.

Duae errores directionis tantum in charta apparuerunt — sine impressione vera
inveniri non possunt. Si pittacium inversum exit, adde `--flip`.

## Protocollum

```
55 55 | mandatum | longitudo | data | XOR | funis AA AA
```

Numeri multorum octetorum **big-endian** sunt; checksum est XOR mandati, longitudinis
et omnium datorum. Cursus impressionis:

```
densitas (0x21) → typus pittacii (0x23) → initium (0x01) →
initium paginae (0x03) → mensura paginae (0x13) → lineae imaginis (0x85) →
finis paginae (0xE3) → status (0xA3) → finis (0xF3)
```

Linea imaginis: `positio(2) | puncta nigra(3) | repetitiones(1) | data(12)`, ubi
octavus bit primi octeti punctum extremum capitis significat. Lineae omnino albae
mandato proprio (`0x84`) mittuntur.

Status (responsum `0xB3`): `pagina(2) | progressus impressionis % | progressus
directionis %`. Impressio finita est, cum ambo progressus 100 sunt.

### Duae subtilitates seriei

Ambae in machina viva inventae, ambae pittacia perdita:

**1. Inter lineas imaginis pausa necessaria est** (hic 4 ms). `writeWithoutResponse`
datos celerius tradit, quam machina accipit, et ea errorem datorum reddit:
responsum `0xDB`, codex `6`.

**2. Quodque pittacium finem paginae praecedentis exspectare debet.** Dum machina
adhuc imprimit, pittacium sequens ei data supervacanea sunt, et iterum `0xDB`/`6`
reddit. Rector statum interrogat et duas mensuras «100 % impressionis et directionis»
exspectat.

Error `0xDB` etiam **legitur**: rector codicem interpretatur (operculum apertum,
charta deest, calor nimius, taenia deest) et opus sistit, pro eo ut tacite tempus
exspectet.

### Duo laquei notandi

**1. Machina duo servitia BLE habet, quorum unum solum respondet.**

| Servitium | Quid est |
|---|---|
| `e7810a71-73ae-499d-8c15-faa9aef0c3f2` | **servitium NIIMBOT ipsum** — hic firmware audit |
| `49535343-fe7d-4ae5-8fa9-9fafd205e455` | UART apertum — proprietatibus congruit, sed **nulli mandato respondet** |

**2. BlueZ prius machinam videre debet.** Sine exploratione praevia conexio errore
D-Bus deficit. Ceterum `Adapter.Scan` in Linux **usque ad `StopScan` obstruit** —
in goroutine propria mittendus est.

### Alia exemplaria

Numeri N1 proprii sunt: numerus exemplaris `3586`, caput `96` punctorum, densitas
`1..3`. Pro alio exemplari eos in `protocol.go` muta (tabula in
[NiimBlueLib](https://github.com/MultiMote/niimbluelib/blob/master/src/printer_models.ts)).

## Densitas impressionis

`--density 1..3`. Intervallum non arbitrarium est: in tabula exemplarium NiimBlueLib
N1 `densityMin: 1, densityMax: 3, densityDefault: 2` habet; alia exemplaria `1..5`.

Expertum: omnes tres gradus usui sunt, **optimus 2**. Ad 3 ascendendum est, si
lineae tenues rumpuntur; ad 1 descendendum, si litterae parvae cohaerescunt.

## Probationes

```bash
go test ./...
```

Probationes compositionem mandatorum **cum octetis e machina viva captis** conferunt
(handshake `5555c10101c1aaaa`, quaestio exemplaris `555540010849aaaa`), responsa
legunt, lineas et directiones, fontem album, typos pittaciorum, mensuram automaticam
typi, structuras chemicas, diem et horam experiuntur.

## Quid probatum sit, quid non

**In machina viva N1 (GA24110447) probatum:**

- inventio, conexio, handshake;
- numerus exemplaris `3586`, numerus serialis, firmware `3.13`, onus;
- **impressio litterarum** — cursus integer: `pagina 1, impressio 100%, directio 100%`;
- **series plurium pittaciorum uno mandato** — tria pittacia, `pagina 3, 100%`;
- **ordo columnarum** — lineae ordine dato currunt, columna omissa non imprimitur;
- **vacuus columna lineam vacuam servat** — dispositio omnium pittaciorum eadem;
- **imago** — imago et imago ad mensuram redacta;
- **imago ad mensuram picta clarior est** quam e tabula magna redacta (240×96 contra 900×360);
- **typus litterarum et altitudo lineae** — quattuor typorum discrimen in charta visum;
- **linea, forma, rotatio 90°** — omnia in uno pittacio;
- **structura chemica** — линалоол, кумарин, ванилин;
- **densitas** — omnes tres gradus usui sunt, optimus **2**;
- errores directionis et conexionis tantum in machina viva apparuerunt: neque
  probationes neque forma eas monstrabant.

**Nondum probatum:**

- typi pittaciorum praeter `withgaps`;
- alia exemplaria NIIMBOT;
- aedificatio pro Windows — compilatur, sed in machina viva non experta.

## Aedificatio pro aliis systematibus

```bash
go build -o niimbot .                                  # systema proprium
GOOS=windows GOARCH=amd64 go build -o niimbot.exe .    # Windows, etiam e Linux
GOOS=darwin  GOARCH=arm64 go build -o niimbot-mac .    # macOS — solum in Mac
```

| Systema | Status |
|---|---|
| **Linux** | laborat, in machina viva probatum |
| **Windows** | e Linux compilatur immutatum; in machina viva nondum expertum |
| **macOS** | **in Mac aedificandus est**: ibi Bluetooth CoreBluetooth est, cuius ligamen CGO et compages Apple postulat |

**Proprium macOS:** systema inscriptiones veras non tradit — pro MAC dat UUID,
itaque machina exploratione invenitur et `--address` UUID accipit.

**Typus litterarum** ipse quaeritur: DejaVu/Noto/Liberation in Linux, Arial/Segoe UI
in Windows, Arial/Helvetica in macOS. Suum per `NIIMBOT_FONT` dari potest.

## Projecta affinia

Si Linux iuxta machinam non est, aliae viae patent:

| Projectum | Quid dat |
|---|---|
| [NiimBlueLib](https://github.com/MultiMote/niimbluelib) | bibliotheca protocolli NIIMBOT aperta (JS, MIT) |
| [NiimBlue](https://niim.blue/) | cliens interretialis: impressio e navigatro per Web Bluetooth, **sine servitore** |
| [NiimPrintX](https://github.com/labbots/NiimPrintX) | cliens Python cum interfacie graphica |

Hic rector eo distinguitur, quod e linea mandatorum Linux laborat et series
pittaciorum e CSV cum forma imprimere potest.

## Gratiae

Protocollum e bibliotheca aperta **[NiimBlueLib](https://github.com/MultiMote/niimbluelib)**
(MIT) reconstructum est — ea est accuratissima implementatio protocolli NIIMBOT
aperta. Inde forma mandatorum, codices, divisio punctorum et parametri exemplarium
sumpti sunt.

**Structuras chemicas** pingit **[RDKit](https://www.rdkit.org/)** (BSD 3-Clause) —
ut programma separatum vocatur neque in programmate includitur. Cetera omnia huius
rectoris proprius codex Go est.

## Monitum

Hoc projectum cum NIIMBOT non coniunctum est neque ab artifice probatum.
«NIIMBOT» et nomina exemplarium dominis suis pertinent. Machina per protocollum
reconstructum regitur — suo periculo utere.

## Licentia

[MIT](LICENSE).
