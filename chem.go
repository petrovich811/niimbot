// Химические структуры: SMILES → картинка через RDKit.
//
// Своей отрисовки структур здесь нет и быть не должно: разбор SMILES — это
// полдела, а вот 2D-раскладка (восприятие циклов, расстановка координат,
// борьба с наложениями) — работа на годы, и её давно сделал RDKit.
// Поэтому драйвер зовёт RDKit и получает готовый PNG.
//
// Важно: структуру надо рисовать СРАЗУ в нужном размере и с толстыми связями.
// Проверено на живом принтере и в макетах: если уменьшить крупную картинку,
// при 96 точках от структуры остаются точки. Рендер сразу в размер этикетки
// с bondLineWidth 2–3 читается уверенно.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// chemScript — задание для RDKit: JSON на входе, PNG в base64 на выходе.
const chemScript = `
import sys, json, base64
from rdkit import Chem
from rdkit.Chem.Draw import rdMolDraw2D

req = json.load(sys.stdin)
mol = Chem.MolFromSmiles(req["smiles"])
if mol is None:
    sys.exit("не удалось разобрать SMILES")

w, h = int(req["w"]), int(req["h"])
d = rdMolDraw2D.MolDraw2DCairo(w, h)
o = d.drawOptions()
o.bondLineWidth = float(req.get("bond", 2.0))
o.rotate = float(req.get("rotate", 0))
o.padding = float(req.get("padding", 0.02))
o.clearBackground = True
rdMolDraw2D.PrepareAndDrawMolecule(d, mol)
d.FinishDrawing()
sys.stdout.write(base64.b64encode(d.GetDrawingText()).decode())
`

// chemEnvVar позволяет указать свой Python с RDKit.
const chemEnvVar = "NIIMBOT_RDKIT"

var (
	chemMu    sync.Mutex
	chemCache = map[string]*image.Gray{}
)

// chemPython ищет Python, в котором есть RDKit.
func chemPython() string {
	if p := os.Getenv(chemEnvVar); p != "" {
		return p
	}
	var candidates []string
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, "chemvenv", "bin", "python"),
			filepath.Join(dir, "chemvenv", "Scripts", "python.exe"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		base := filepath.Join(home, ".local", "share", "niimbot", "chemvenv")
		candidates = append(candidates,
			filepath.Join(base, "bin", "python"),
			filepath.Join(base, "Scripts", "python.exe"))
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Последняя надежда — системный python3 с установленным rdkit.
	for _, name := range []string{"python3", "python"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// RenderSmiles рисует структуру по строке SMILES.
//
// Результат кэшируется по строке и размеру: в серии из полусотни этикеток
// одна и та же молекула рисуется один раз.
func RenderSmiles(smiles string, width, height, rotate int, bond float64) (*image.Gray, error) {
	smiles = strings.TrimSpace(smiles)
	if smiles == "" {
		return nil, fmt.Errorf("не задана строка SMILES")
	}
	if width < 8 {
		width = 8
	}
	if height < 8 {
		height = 8
	}
	if width > printheadPixels*8 {
		width = printheadPixels * 8
	}
	if bond <= 0 {
		bond = 2
	}

	rotate = ((rotate % 360) + 360) % 360
	key := fmt.Sprintf("%s|%d|%d|%d|%.1f", smiles, width, height, rotate, bond)
	chemMu.Lock()
	if img, ok := chemCache[key]; ok {
		chemMu.Unlock()
		return img, nil
	}
	chemMu.Unlock()

	img, err := runRdkit(smiles, width, height, rotate, bond)
	if err != nil {
		return nil, err
	}

	chemMu.Lock()
	chemCache[key] = img
	chemMu.Unlock()
	return img, nil
}

// runRdkit запускает Python с RDKit и возвращает нарисованную структуру.
func runRdkit(smiles string, width, height, rotate int, bond float64) (*image.Gray, error) {
	py := chemPython()
	if py == "" {
		return nil, fmt.Errorf("не найден Python — без него не нарисовать структуру по SMILES")
	}

	task, _ := json.Marshal(map[string]any{
		"smiles": smiles, "w": width, "h": height, "bond": bond, "rotate": rotate,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, py, "-c", chemScript)
	cmd.Stdin = bytes.NewReader(task)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		if strings.Contains(detail, "ModuleNotFoundError") || strings.Contains(detail, "No module named") {
			return nil, fmt.Errorf(
				"для структур нужен RDKit: pip install rdkit в отдельном окружении "+
					"(подробности в README; путь к Python можно задать через %s)", chemEnvVar)
		}
		return nil, fmt.Errorf("RDKit не нарисовал структуру: %s", detail)
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(stdout.String()))
	if err != nil {
		return nil, fmt.Errorf("ответ RDKit не разобран: %w", err)
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("картинка от RDKit не разобрана: %w", err)
	}

	out := image.NewGray(image.Rect(0, 0, width, height))
	draw.Draw(out, out.Bounds(), src, src.Bounds().Min, draw.Src)
	return out, nil
}

// ChemAvailable сообщает, готов ли RDKit к работе.
func ChemAvailable() bool {
	py := chemPython()
	if py == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, py, "-c", "import rdkit")
	return cmd.Run() == nil
}
