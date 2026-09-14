// NIIMBOT N1 — интерфейс печати этикеток.
// Никаких фреймворков: обычный JS, общение с драйвером через /api.

const $ = (id) => document.getElementById(id);

let mode = 'single';
let previewURL = null;

// ---------------------------------------------------------------- состояние

async function loadStatus() {
  const el = $('status');
  el.textContent = 'читаю состояние…';
  try {
    const r = await fetch('/api/status');
    const d = await r.json();
    if (!r.ok) throw new Error(d.error || 'нет связи');

    const p = d.printer || {};
    const paper = d.rfid_paper || {};
    const ribbon = d.rfid_ribbon || {};
    const parts = [];

    if (p.ModelID) parts.push(`модель <b>${p.ModelID}</b>`);
    if (p.Serial) parts.push(`№ <b>${p.Serial}</b>`);
    if (p.Battery !== undefined) parts.push(`заряд <b>${p.Battery}</b>`);
    if (paper.tag) {
      parts.push(`этикетки <b>${paper.labelType}</b>, осталось <b>${paper.left}</b>`);
    } else {
      parts.push('<span class="warn">метка рулона не прочитана</span>');
    }
    if (ribbon.tag) parts.push(`лента: осталось <b>${ribbon.left}</b>`);
    if (p.ModelID && p.ModelID !== 3586) {
      parts.push('<span class="warn">модель не N1</span>');
    }
    el.innerHTML = parts.join(' · ');
  } catch (e) {
    el.innerHTML = `<span class="warn">нет связи: ${e.message}</span>`;
  }
}

// ------------------------------------------------------------------- режимы

document.querySelectorAll('.tab').forEach((tab) => {
  tab.onclick = () => {
    document.querySelectorAll('.tab').forEach((t) => t.classList.remove('active'));
    tab.classList.add('active');
    mode = tab.dataset.mode;
    $('pane-single').classList.toggle('hidden', mode !== 'single');
    $('pane-series').classList.toggle('hidden', mode !== 'series');
    updateSeriesCount();
  };
});

function seriesItems() {
  return $('series-text').value
    .split('\n')
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
}

function updateSeriesCount() {
  const n = seriesItems().length;
  $('series-count').textContent = `${n} ${plural(n, 'этикетка', 'этикетки', 'этикеток')}`;
}

function plural(n, one, few, many) {
  const m10 = n % 10, m100 = n % 100;
  if (m10 === 1 && m100 !== 11) return one;
  if (m10 >= 2 && m10 <= 4 && (m100 < 10 || m100 >= 20)) return few;
  return many;
}

$('series-text').addEventListener('input', updateSeriesCount);

// ---------------------------------------------------------------- настройки

function collectRequest() {
  const items = mode === 'single'
    ? [$('single-text').value]
    : seriesItems();

  return {
    items,
    template: mode === 'series' ? $('template').value : '',
    length: parseFloat($('length').value) || 30,
    font: parseFloat($('font').value) || 0,
    density: parseInt($('density').value, 10) || 2,
    label: $('label').value,
    copies: parseInt($('copies').value, 10) || 1,
    flip: $('flip').checked,
  };
}

function applySettings(s) {
  if (s.length) $('length').value = s.length;
  if (s.font !== undefined) $('font').value = s.font;
  if (s.density) $('density').value = s.density;
  if (s.label !== undefined) $('label').value = s.label;
  if (s.copies) $('copies').value = s.copies;
  if (s.template !== undefined) $('template').value = s.template;
  $('flip').checked = !!s.flip;
}

// ------------------------------------------------------------- предпросмотр

async function doPreview() {
  const req = collectRequest();
  if (!req.items.length || !req.items[0].trim()) {
    alert('Введите текст этикетки');
    return;
  }
  try {
    const r = await fetch('/api/preview', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req),
    });
    if (!r.ok) {
      const d = await r.json().catch(() => ({}));
      throw new Error(d.error || 'не удалось построить макет');
    }
    const blob = await r.blob();
    if (previewURL) URL.revokeObjectURL(previewURL);
    previewURL = URL.createObjectURL(blob);

    const img = $('preview');
    img.src = previewURL;
    img.classList.add('ready');

    const px = await imageSize(previewURL);
    $('preview-info').textContent =
      `${px.w}×${px.h} точек · ${(px.w / 8).toFixed(1)}×${(px.h / 8).toFixed(1)} мм` +
      (mode === 'series' ? ' · предпросмотр первой этикетки серии' : '');
  } catch (e) {
    alert(e.message);
  }
}

function imageSize(url) {
  return new Promise((resolve) => {
    const i = new Image();
    i.onload = () => resolve({ w: i.naturalWidth, h: i.naturalHeight });
    i.src = url;
  });
}

// -------------------------------------------------------------------- печать

async function doPrint() {
  const req = collectRequest();
  const n = req.items.filter((s) => s.trim()).length;
  if (!n) {
    alert('Нечего печатать: список пуст');
    return;
  }
  const total = n * req.copies;
  if (!confirm(`Напечатать ${total} ${plural(total, 'этикетку', 'этикетки', 'этикеток')}?`)) return;

  $('print-btn').disabled = true;
  $('progress').classList.remove('hidden');
  setProgress(0, total, 'отправляю на принтер…');

  try {
    const r = await fetch('/api/print', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req),
    });
    const d = await r.json();
    if (!r.ok) throw new Error(d.error || 'не удалось начать печать');
    await pollJob(d.job, d.total);
  } catch (e) {
    $('progress-text').textContent = 'ошибка: ' + e.message;
  } finally {
    $('print-btn').disabled = false;
  }
}

async function pollJob(id, total) {
  while (true) {
    await new Promise((r) => setTimeout(r, 600));
    const r = await fetch('/api/job?id=' + encodeURIComponent(id));
    if (!r.ok) continue;
    const j = await r.json();
    setProgress(j.done, j.total || total, null);

    if (j.state === 'done') {
      setProgress(j.total, j.total, 'готово');
      loadStatus();
      return;
    }
    if (j.state === 'error') {
      $('progress-text').textContent = 'ошибка: ' + j.error;
      return;
    }
  }
}

function setProgress(done, total, note) {
  const pct = total ? Math.round((done / total) * 100) : 0;
  $('bar-fill').style.width = pct + '%';
  $('progress-text').textContent =
    note || `напечатано ${done} из ${total} (${pct}%)`;
}

// ------------------------------------------------------------------ шаблоны

async function loadTemplates() {
  const r = await fetch('/api/templates');
  const list = await r.json();
  const sel = $('templates');
  sel.innerHTML = '<option value="">— выберите —</option>';
  (list || []).forEach((t) => {
    const o = document.createElement('option');
    o.value = t.name;
    o.textContent = t.name;
    o.dataset.tpl = JSON.stringify(t);
    sel.appendChild(o);
  });
}

async function saveTemplate() {
  const name = prompt('Название шаблона (например: кабельные бирки):');
  if (!name) return;
  const s = collectRequest();
  const t = {
    name: name.trim(),
    length: s.length, font: s.font, density: s.density,
    label: s.label, copies: s.copies, flip: s.flip,
    template: s.template,
  };
  const r = await fetch('/api/templates', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(t),
  });
  if (!r.ok) {
    const d = await r.json().catch(() => ({}));
    alert(d.error || 'не удалось сохранить');
    return;
  }
  await loadTemplates();
  $('templates').value = t.name;
}

async function deleteTemplate() {
  const name = $('templates').value;
  if (!name) return;
  if (!confirm(`Удалить шаблон «${name}»?`)) return;
  await fetch('/api/templates?name=' + encodeURIComponent(name), { method: 'DELETE' });
  await loadTemplates();
}

$('templates').addEventListener('change', (e) => {
  const opt = e.target.selectedOptions[0];
  if (opt && opt.dataset.tpl) applySettings(JSON.parse(opt.dataset.tpl));
});

// ---------------------------------------------------------------------- CSV

$('csv-btn').onclick = () => $('csv-file').click();

// Разбор строки CSV: учитывает кавычки и удвоенные кавычки внутри поля.
// Разделителем считается ; , или табуляция — что встретится первым.
function parseCsvLine(line) {
  const sep = line.includes('\t') ? '\t' : (line.includes(';') ? ';' : ',');
  const out = [];
  let cur = '', inQuotes = false;
  for (let i = 0; i < line.length; i++) {
    const c = line[i];
    if (inQuotes) {
      if (c === '"') {
        if (line[i + 1] === '"') { cur += '"'; i++; }
        else inQuotes = false;
      } else cur += c;
    } else if (c === '"') {
      inQuotes = true;
    } else if (c === sep) {
      out.push(cur); cur = '';
    } else cur += c;
  }
  out.push(cur);
  return out.map((s) => s.trim());
}

$('csv-file').addEventListener('change', async (e) => {
  const file = e.target.files[0];
  if (!file) return;
  const text = await file.text();

  let rows = text.split(/\r?\n/)
    .map(parseCsvLine)
    .filter((cols) => cols.some((c) => c.length > 0));

  // Строку заголовков из Excel отбрасываем, если попросили.
  if ($('csv-header').checked && rows.length > 1) rows = rows.slice(1);

  // Все столбцы сохраняем: их подставляет шаблон этикетки.
  const lines = rows.map((cols) => cols.join(';'));

  $('series-text').value = lines.join('\n');
  updateSeriesCount();

  // переключаемся на серию, раз принесли список
  document.querySelector('.tab[data-mode="series"]').click();
  e.target.value = '';
});

// ------------------------------------------------------------------- запуск

$('refresh').onclick = loadStatus;
$('preview-btn').onclick = doPreview;
$('print-btn').onclick = doPrint;
$('tpl-save').onclick = saveTemplate;
$('tpl-del').onclick = deleteTemplate;

loadStatus();
loadTemplates();
updateSeriesCount();
