// Конструктор этикеток: холст с сеткой, элементы, шаблоны, печать серии.
//
// Координаты элементов хранятся в миллиметрах в ориентации чтения:
// X — вдоль этикетки (по подаче), Y — поперёк (по головке, 0…12 мм).
// На холсте они умножаются на масштаб (пикселей на миллиметр).

const $ = (id) => document.getElementById(id);

const LABEL_H_MM = 12;   // высота этикетки: 96 точек печати
const SNAP_MM = 0.5;     // шаг прилипания при перетаскивании

let elements = [];
let selected = -1;
let scale = 26;          // пикселей на миллиметр

// ------------------------------------------------------------------ состояние

async function loadStatus() {
  const el = $('status');
  try {
    const r = await fetch('/api/status');
    const d = await r.json();
    if (!r.ok) throw new Error(d.error || 'нет связи');
    const p = d.printer || {}, paper = d.rfid_paper || {};
    const parts = [];
    if (p.ModelID) parts.push(`модель <b>${p.ModelID}</b>`);
    if (p.Battery !== undefined) parts.push(`заряд <b>${p.Battery}</b>`);
    if (paper.tag) parts.push(`этикетки <b>${paper.labelType}</b>, осталось <b>${paper.left}</b>`);
    el.innerHTML = parts.join(' · ') || 'принтер на связи';
  } catch (e) {
    el.innerHTML = `<span class="warn">нет связи: ${e.message}</span>`;
  }
}

// ----------------------------------------------------------------- холст

function labelLength() {
  return parseFloat($('length').value) || 30;
}

function computeScale() {
  const stage = $('stage');
  const avail = Math.max(300, stage.clientWidth - 50);
  scale = Math.max(8, Math.min(30, avail / labelLength()));
}

function renderCanvas() {
  computeScale();
  const label = $('label');
  const W = labelLength() * scale;
  const H = LABEL_H_MM * scale;

  label.style.width = W + 'px';
  label.style.height = H + 'px';
  label.style.setProperty('--mm', scale + 'px');
  label.classList.toggle('no-grid', !$('grid').checked);

  // убираем старые элементы, оставляя сетку
  [...label.querySelectorAll('.el')].forEach((n) => n.remove());

  elements.forEach((el, i) => {
    const node = document.createElement('div');
    node.className = 'el' + (i === selected ? ' selected' : '');
    node.dataset.i = i;
    node.style.left = el.x * scale + 'px';
    node.style.top = el.y * scale + 'px';

    if (el.kind === 'line' || el.kind === 'frame') {
      node.style.width = (el.w || 1) * scale + 'px';
      node.style.height = (el.h || 0.4) * scale + 'px';
      if (el.kind === 'line') {
        node.style.background = '#111';
      } else {
        node.style.background = 'none';
        node.style.border = `${Math.max(1, (el.thickness || 0.3) * scale)}px solid #111`;
      }
      if (i === selected) node.classList.add('selected');
      label.appendChild(node);
      return;
    }

    if (el.kind === 'image') {
      node.style.width = (el.w || 8) * scale + 'px';
      node.style.height = (el.h || 8) * scale + 'px';
      if (el.image) {
        const img = document.createElement('img');
        img.src = el.image;
        node.appendChild(img);
      } else {
        node.innerHTML = '<span class="ph">нет картинки</span>';
      }
    } else {
      // Логический размер блока надписи, а затем поворот: при 90° и 270°
      // ширина и высота на холсте меняются местами.
      const wPx = (el.w || labelLength()) * scale;
      const lh = el.line > 0 ? el.line : (el.font || 2.5) * 1.15;
      const hPx = lh * scale;
      const rot = ((el.rotate || 0) % 360 + 360) % 360;
      const swapped = rot === 90 || rot === 270;

      node.style.width = (swapped ? hPx : wPx) + 'px';
      node.style.height = (swapped ? wPx : hPx) + 'px';

      const inner = document.createElement('div');
      inner.className = 'txt';
      inner.style.width = wPx + 'px';
      inner.style.height = hPx + 'px';
      inner.style.fontSize = (el.font || 2.5) * scale + 'px';
      inner.style.lineHeight = hPx + 'px';
      inner.style.textAlign = el.align || 'left';
      inner.style.transformOrigin = 'top left';
      if (el.family) inner.style.fontFamily = `"${el.family}", sans-serif`;
      inner.style.fontWeight = el.bold ? '700' : '400';
      if (rot === 90) inner.style.transform = `translate(${hPx}px, 0) rotate(90deg)`;
      else if (rot === 180) inner.style.transform = `translate(${wPx}px, ${hPx}px) rotate(180deg)`;
      else if (rot === 270) inner.style.transform = `translate(0, ${wPx}px) rotate(270deg)`;
      inner.textContent = el.text || '';
      node.textContent = '';
      node.appendChild(inner);
    }
    label.appendChild(node);
  });

  drawGuides(label);
  drawRuler();
}

// Линии-подсказки: показывают, к какому краю или середине прилип элемент.
function drawGuides(label) {
  [...label.querySelectorAll('.guide')].forEach((n) => n.remove());
  const add = (style) => {
    const g = document.createElement('div');
    g.className = 'guide';
    Object.assign(g.style, style);
    label.appendChild(g);
  };
  if (guideX !== null) {
    add({ left: guideX * scale + 'px', top: 0, width: '1px', height: '100%' });
  }
  if (guideY !== null) {
    add({ top: guideY * scale + 'px', left: 0, height: '1px', width: '100%' });
  }
}

function drawRuler() {
  const r = document.querySelector('.ruler-x');
  r.style.width = labelLength() * scale + 'px';
  let html = '';
  for (let mm = 0; mm <= labelLength(); mm += 5) {
    html += `<span style="position:absolute;left:${mm * scale}px">${mm}</span>`;
  }
  r.innerHTML = html;
}

// ------------------------------------------------------- выбор и перетаскивание

function select(i) {
  selected = i;
  renderCanvas();
  showProps();
}

$('label').addEventListener('mousedown', (e) => {
  const node = e.target.closest('.el');
  if (!node) { select(-1); return; }
  const i = Number(node.dataset.i);
  select(i);

  const el = elements[i];
  const startX = e.clientX, startY = e.clientY;
  const ox = el.x, oy = el.y;

  const move = (ev) => {
    const dx = (ev.clientX - startX) / scale;
    const dy = (ev.clientY - startY) / scale;
    let nx = snap(Math.max(0, ox + dx));
    let ny = snap(Math.max(0, oy + dy));
    if (!ev.altKey) { // Alt отключает прилипание к краям
      const r = snapToEdges(el, nx, ny);
      nx = r.x; ny = r.y;
    } else {
      guideX = guideY = null;
    }
    el.x = nx;
    el.y = ny;
    renderCanvas();
    showProps();
  };
  const up = () => {
    document.removeEventListener('mousemove', move);
    document.removeEventListener('mouseup', up);
    guideX = guideY = null;
    renderCanvas();
    refreshPreview();
  };
  document.addEventListener('mousemove', move);
  document.addEventListener('mouseup', up);
  e.preventDefault();
});

function snap(v) {
  return Math.round(v / SNAP_MM) * SNAP_MM;
}

// Прилипание к краям и середине этикетки.
//
// Кроме сетки 0,5 мм элемент «притягивается» к левому краю, середине и
// правому краю по X, и к верху, середине и низу по Y. Без этого выставить
// элемент ровно по центру можно только на глазок.
const SNAP_EDGE_MM = 0.6; // насколько близко надо подойти, чтобы прилипнуть
let guideX = null, guideY = null; // линии-подсказки, мм

function sizeOf(el) {
  if (el.kind === 'text') return { w: el.w || labelLength(), h: (el.line || (el.font || 2.5) * 1.15) };
  return { w: el.w || 0, h: el.h || 0 };
}

function snapToEdges(el, x, y) {
  const L = labelLength(), H = LABEL_H_MM;
  const { w, h } = sizeOf(el);
  guideX = guideY = null;

  // кандидаты для левого края: 0, по центру, вправо
  const candX = [0, (L - w) / 2, L - w];
  for (const c of candX) {
    if (Math.abs(x - c) < SNAP_EDGE_MM) { x = c; guideX = c <= 0.01 ? 0 : (Math.abs(c - (L - w) / 2) < 0.01 ? L / 2 : L); break; }
  }
  const candY = [0, (H - h) / 2, H - h];
  for (const c of candY) {
    if (Math.abs(y - c) < SNAP_EDGE_MM) { y = c; guideY = c <= 0.01 ? 0 : (Math.abs(c - (H - h) / 2) < 0.01 ? H / 2 : H); break; }
  }
  return { x: Math.max(0, Math.min(L - w, x)), y: Math.max(0, Math.min(H - h, y)) };
}

document.addEventListener('keydown', (e) => {
  if ((e.key === 'Delete' || e.key === 'Backspace') && selected >= 0 &&
      !['INPUT', 'TEXTAREA', 'SELECT'].includes(document.activeElement.tagName)) {
    elements.splice(selected, 1);
    select(-1);
    refreshPreview();
    e.preventDefault();
  }
});

// ------------------------------------------------------------------ свойства

// Список системных шрифтов приходит с сервера: в шаблоне хранится семейство.
async function loadFonts() {
  try {
    const r = await fetch('/api/fonts');
    const d = await r.json();
    const sel = $('p-family');
    sel.innerHTML = '<option value="">по умолчанию</option>';
    (d.families || []).forEach((f) => {
      const o = document.createElement('option');
      o.value = f;
      o.textContent = f;
      sel.appendChild(o);
    });
  } catch (e) { /* без списка шрифтов конструктор всё равно работает */ }
}

function showProps() {
  const has = selected >= 0 && selected < elements.length;
  $('props').classList.toggle('hidden', !has);
  $('no-selection').classList.toggle('hidden', has);
  if (!has) return;

  const el = elements[selected];
  $('p-x').value = el.x;
  $('p-y').value = el.y;
  $('p-w').value = el.w || 0;
  $('p-h').value = el.h || 0;
  $('p-text').value = el.text || '';
  $('p-font').value = el.font || 2.5;
  $('p-line').value = el.line || 0;
  $('p-family').value = el.family || '';
  $('p-bold').checked = !!el.bold;
  $('p-align').value = el.align || 'left';

  $('p-rotate').value = String(el.rotate || 0);
  $('p-thickness').value = el.thickness || 0.3;

  const isText = el.kind === 'text';
  $('text-props').classList.toggle('hidden', !isText);
  $('image-props').classList.toggle('hidden', el.kind !== 'image');
  $('shape-props').classList.toggle('hidden', !(el.kind === 'line' || el.kind === 'frame'));
  $('h-wrap').classList.toggle('hidden', isText);
}

function bindProp(id, apply) {
  $(id).addEventListener('input', () => {
    if (selected < 0) return;
    apply(elements[selected], $(id).value);
    renderCanvas();
    schedulePreview();
  });
}

bindProp('p-x', (el, v) => { el.x = Math.max(0, parseFloat(v) || 0); });
bindProp('p-y', (el, v) => { el.y = Math.max(0, parseFloat(v) || 0); });
bindProp('p-w', (el, v) => { el.w = Math.max(0, parseFloat(v) || 0); });
bindProp('p-h', (el, v) => { el.h = Math.max(0, parseFloat(v) || 0); });
bindProp('p-text', (el, v) => { el.text = v; });
bindProp('p-font', (el, v) => { el.font = Math.max(1, parseFloat(v) || 2.5); });
bindProp('p-align', (el, v) => { el.align = v; });
bindProp('p-family', (el, v) => { el.family = v; });
bindProp('p-line', (el, v) => { el.line = Math.max(0, parseFloat(v) || 0); });
bindProp('p-rotate', (el, v) => { el.rotate = parseInt(v, 10) || 0; });
bindProp('p-thickness', (el, v) => { el.thickness = Math.max(0.05, parseFloat(v) || 0.3); });
$('p-bold').addEventListener('change', () => {
  if (selected < 0) return;
  elements[selected].bold = $('p-bold').checked;
  renderCanvas();
  schedulePreview();
});

$('p-image-file').addEventListener('change', (e) => {
  const file = e.target.files[0];
  if (!file || selected < 0) return;
  readAsDataURL(file, (url) => {
    elements[selected].image = url;
    renderCanvas();
    refreshPreview();
  });
  e.target.value = '';
});

function readAsDataURL(file, cb) {
  const r = new FileReader();
  r.onload = () => cb(r.result);
  r.readAsDataURL(file);
}

// ------------------------------------------------------------------- кнопки

$('add-text').onclick = () => {
  elements.push({ kind: 'text', x: 1, y: 1, w: labelLength() - 2, text: 'Надпись {1}', font: 3,
                   align: 'left', family: $('p-family').value || '', bold: $('p-bold').checked });
  select(elements.length - 1);
  refreshPreview();
};

$('add-image').onclick = () => $('image-file').click();

$('add-line').onclick = () => {
  elements.push({ kind: 'line', x: 1, y: 5.7, w: labelLength() - 2, h: 0.4 });
  select(elements.length - 1);
  refreshPreview();
};

$('add-frame').onclick = () => {
  elements.push({ kind: 'frame', x: 0.4, y: 0.4, w: labelLength() - 0.8, h: 11.2, thickness: 0.4 });
  select(elements.length - 1);
  refreshPreview();
};

$('image-file').addEventListener('change', (e) => {
  const file = e.target.files[0];
  if (!file) return;
  readAsDataURL(file, (url) => {
    elements.push({ kind: 'image', x: 1, y: 1, w: 8, h: 8, image: url });
    renderCanvas();
    select(elements.length - 1);
    refreshPreview();
  });
  e.target.value = '';
});

$('del-el').onclick = () => {
  if (selected < 0) return;
  elements.splice(selected, 1);
  select(-1);
  refreshPreview();
};

$('grid').onchange = renderCanvas;
$('length').addEventListener('input', () => {
  // Надписи, растянутые на всю этикетку, тянем за длиной.
  elements.forEach((el) => {
    if (el.kind === 'text' && el.w > 0 && Math.abs(el.w - (el._len || 0)) < 2) el.w = labelLength() - 2;
    el._len = labelLength();
  });
  renderCanvas();
  schedulePreview();
});

// --------------------------------------------------------------- макет и печать

let previewTimer = null;
function schedulePreview() {
  clearTimeout(previewTimer);
  previewTimer = setTimeout(refreshPreview, 400);
}

function requestBody() {
  return {
    items: [$('sample').value],
    elements: elements,
    length: labelLength(),
    density: parseInt($('density').value, 10) || 2,
    label: $('label').value,
    copies: parseInt($('copies').value, 10) || 1,
    flip: $('flip').checked,
  };
}

async function refreshPreview() {
  try {
    const r = await fetch('/api/preview', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(requestBody()),
    });
    if (!r.ok) {
      const d = await r.json().catch(() => ({}));
      $('preview').classList.remove('ready');
      $('preview-empty').textContent = 'ошибка: ' + (d.error || 'не удалось построить макет');
      $('preview-empty').style.display = 'block';
      return;
    }
    const blob = await r.blob();
    const img = $('preview');
    if (img.dataset.url) URL.revokeObjectURL(img.dataset.url);
    img.dataset.url = URL.createObjectURL(blob);
    img.src = img.dataset.url;
    img.classList.add('ready');
  } catch (e) {
    $('preview-empty').textContent = 'ошибка: ' + e.message;
  }
}

$('refresh-preview').onclick = refreshPreview;
$('sample').addEventListener('input', schedulePreview);

// ---------------------------------------------------------------------- серия

function seriesItems() {
  return $('series-text').value.split('\n').map((s) => s.trim()).filter((s) => s.length > 0);
}

function plural(n, one, few, many) {
  const m10 = n % 10, m100 = n % 100;
  if (m10 === 1 && m100 !== 11) return one;
  if (m10 >= 2 && m10 <= 4 && (m100 < 10 || m100 >= 20)) return few;
  return many;
}

function updateCount() {
  const n = seriesItems().length;
  $('series-count').textContent = `${n} ${plural(n, 'этикетка', 'этикетки', 'этикеток')}`;
}
$('series-text').addEventListener('input', updateCount);

$('csv-btn').onclick = () => $('csv-file').click();
$('csv-file').addEventListener('change', async (e) => {
  const file = e.target.files[0];
  if (!file) return;
  const text = await file.text();
  let rows = text.split(/\r?\n/).map(parseCsvLine).filter((c) => c.some((v) => v.length > 0));
  if ($('csv-header').checked && rows.length > 1) rows = rows.slice(1);
  $('series-text').value = rows.map((c) => c.join(';')).join('\n');
  updateCount();
  e.target.value = '';
});

function parseCsvLine(line) {
  const sep = line.includes('\t') ? '\t' : (line.includes(';') ? ';' : ',');
  const out = [];
  let cur = '', q = false;
  for (let i = 0; i < line.length; i++) {
    const c = line[i];
    if (q) {
      if (c === '"') {
        if (line[i + 1] === '"') { cur += '"'; i++; } else q = false;
      } else cur += c;
    } else if (c === '"') q = true;
    else if (c === sep) { out.push(cur); cur = ''; }
    else cur += c;
  }
  out.push(cur);
  return out.map((s) => s.trim());
}

$('print-btn').onclick = async () => {
  const items = seriesItems();
  if (!items.length) { alert('Список пуст'); return; }
  if (!elements.length) { alert('На этикетке нет ни одного элемента'); return; }
  const total = items.length * (parseInt($('copies').value, 10) || 1);
  if (!confirm(`Напечатать ${total} ${plural(total, 'этикетку', 'этикетки', 'этикеток')}?`)) return;

  $('print-btn').disabled = true;
  $('progress').classList.remove('hidden');
  setProgress(0, total, 'отправляю на принтер…');

  const body = { ...requestBody(), items: items };
  try {
    const r = await fetch('/api/print', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    const d = await r.json();
    if (!r.ok) throw new Error(d.error || 'не удалось начать печать');
    await pollJob(d.job, d.total);
  } catch (e) {
    $('progress-text').textContent = 'ошибка: ' + e.message;
  } finally {
    $('print-btn').disabled = false;
  }
};

async function pollJob(id, total) {
  while (true) {
    await new Promise((r) => setTimeout(r, 700));
    const r = await fetch('/api/job?id=' + encodeURIComponent(id));
    if (!r.ok) continue;
    const j = await r.json();
    setProgress(j.done, j.total || total, null);
    if (j.state === 'done') { setProgress(j.total, j.total, 'готово'); loadStatus(); return; }
    if (j.state === 'error') { $('progress-text').textContent = 'ошибка: ' + j.error; return; }
  }
}

function setProgress(done, total, note) {
  const pct = total ? Math.round((done / total) * 100) : 0;
  $('bar-fill').style.width = pct + '%';
  $('progress-text').textContent = note || `напечатано ${done} из ${total} (${pct}%)`;
}

// -------------------------------------------------------------------- шаблоны

async function loadTemplates() {
  const r = await fetch('/api/templates');
  const list = (await r.json()) || [];
  const sel = $('templates');
  sel.innerHTML = '<option value="">— выберите —</option>';
  list.forEach((t) => {
    const o = document.createElement('option');
    o.value = t.name;
    o.textContent = t.name;
    o.dataset.tpl = JSON.stringify(t);
    sel.appendChild(o);
  });
}

$('templates').addEventListener('change', (e) => {
  const opt = e.target.selectedOptions[0];
  if (!opt || !opt.dataset.tpl) return;
  const t = JSON.parse(opt.dataset.tpl);
  elements = t.elements ? JSON.parse(JSON.stringify(t.elements)) : [];
  if (t.length) $('length').value = t.length;
  if (t.density) $('density').value = t.density;
  if (t.label !== undefined) $('label').value = t.label;
  if (t.copies) $('copies').value = t.copies;
  $('flip').checked = !!t.flip;
  select(-1);
  refreshPreview();
});

$('tpl-save').onclick = async () => {
  const name = prompt('Название шаблона:');
  if (!name) return;
  const body = {
    name: name.trim(),
    length: labelLength(),
    density: parseInt($('density').value, 10) || 2,
    label: $('label').value,
    copies: parseInt($('copies').value, 10) || 1,
    flip: $('flip').checked,
    elements: elements,
  };
  const r = await fetch('/api/templates', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!r.ok) {
    const d = await r.json().catch(() => ({}));
    alert(d.error || 'не удалось сохранить');
    return;
  }
  await loadTemplates();
  $('templates').value = name.trim();
};

$('tpl-del').onclick = async () => {
  const name = $('templates').value;
  if (!name) return;
  if (!confirm(`Удалить шаблон «${name}»?`)) return;
  await fetch('/api/templates?name=' + encodeURIComponent(name), { method: 'DELETE' });
  await loadTemplates();
};

$('tpl-new').onclick = () => {
  elements = [];
  $('templates').value = '';
  select(-1);
  refreshPreview();
};

// --------------------------------------------------------------------- запуск

// Пустой конструктор встречаем одной готовой надписью: с чего-то надо начать.
if (!elements.length) {
  elements.push({ kind: 'text', x: 1, y: 4, w: labelLength() - 2, text: '{1}', font: 3, align: 'center' });
}

loadStatus();
loadFonts();
loadTemplates();
renderCanvas();
showProps();
updateCount();
