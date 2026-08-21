// Sabdopalon dashboard — shared helpers (no framework, no build step).

async function api(path, opts) {
  const r = await fetch(path, opts);
  let data;
  try { data = await r.json(); } catch { data = { error: 'invalid response' }; }
  if (!r.ok && !data.error) data.error = 'HTTP ' + r.status;
  return data;
}

async function post(path, body) {
  return api(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body || {}),
  });
}

function toast(msg, isErr) {
  const t = document.getElementById('toast');
  t.textContent = msg;
  t.className = isErr ? 'err' : '';
  t.style.display = 'block';
  clearTimeout(t._timer);
  t._timer = setTimeout(() => (t.style.display = 'none'), 4200);
}

function esc(s) {
  const d = document.createElement('div');
  d.textContent = s == null ? '' : String(s);
  return d.innerHTML;
}

function poll(fn, ms) { fn(); return setInterval(fn, ms); }

// Header status dot (all pages).
poll(async () => {
  const s = await api('/api/status');
  const dot = document.getElementById('status-dot');
  const txt = document.getElementById('status-text');
  if (!s || s.error) { dot.classList.remove('ok'); txt.textContent = 'offline'; return; }
  dot.classList.add('ok');
  txt.textContent = `v${s.version} · up ${s.uptime}`;
}, 5000);

// ---------- modal helper ----------
function openModal(html) {
  const back = document.createElement('div');
  back.className = 'modal-backdrop';
  back.innerHTML = `<div class="modal">${html}</div>`;
  document.body.appendChild(back);
  return back;
}
