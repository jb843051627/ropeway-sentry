const $ = (sel) => document.querySelector(sel);

async function api(path, options) {
  const res = await fetch(path, options);
  const text = await res.text();
  let body = text;
  try { body = JSON.parse(text); } catch (e) { /* 非 JSON 直接返回文本 */ }
  if (!res.ok) throw new Error(typeof body === 'object' ? body.error : body);
  return body;
}

function badge(level) {
  const cls = { open: 'ok', restricted: 'warn', maintenance: 'maint', closed: 'crit' }[level] || '';
  return `<span class="badge ${cls}">${level}</span>`;
}

async function refreshLines() {
  try {
    const lines = await api('/api/lines');
    const tbody = $('#lines tbody');
    tbody.innerHTML = lines.map(l =>
      `<tr><td>${l.id}</td><td>${l.code}</td><td>${l.name}</td><td>${badge(l.status)}</td></tr>`).join('');
  } catch (e) { console.error(e); }
}

async function refreshDashboard() {
  try {
    const board = await api('/api/dashboard');
    const items = [
      `线路总数 <b>${board.lines_total}</b>`,
      `开放/受限/维护/停运 ${['open','restricted','maintenance','closed'].map(k => board.lines_by_status[k] || 0).join(' / ')}`,
      `未关闭告警 warning <b>${board.open_warnings}</b> · critical <b>${board.open_criticals}</b>`,
      `生效维护锁 <b>${board.active_holds}</b>`,
      `24h 批次 <b>${board.batches_24h}</b> · 完整率 ${(board.integrity_rate_24h * 100).toFixed(1)}%`
    ];
    $('#dashboard').innerHTML = items.map(i => `<li>${i}</li>`).join('');
  } catch (e) { console.error(e); }
}

async function refreshAlerts() {
  try {
    const alerts = await api('/api/alerts?status=open');
    const acked = await api('/api/alerts?status=acked');
    const all = alerts.concat(acked);
    $('#alerts tbody').innerHTML = all.slice(0, 20).map(a =>
      `<tr><td>${a.id}</td><td class="${a.severity === 'critical' ? 'crit' : 'warn'}">${a.severity}</td>` +
      `<td>${a.kind}</td><td>${a.status}</td><td>${a.occurrences}</td>` +
      `<td><a href="#" onclick="$('#alert-id').value=${a.id};return false;">填入</a></td></tr>`).join('') ||
      '<tr><td colspan="6">暂无未关闭告警</td></tr>';
  } catch (e) { console.error(e); }
}

async function ackAlert() {
  const id = $('#alert-id').value;
  if (!id) return alert('请填写告警ID');
  try { await api(`/api/alerts/${id}/ack`, { method: 'POST', headers: {'Content-Type':'application/json'}, body: '{"by":"console"}'}); refreshAlerts(); }
  catch (e) { alert(e.message); }
}

async function closeAlert() {
  const id = $('#alert-id').value;
  if (!id) return alert('请填写告警ID');
  try { await api(`/api/alerts/${id}/close`, { method: 'POST', headers: {'Content-Type':'application/json'}, body: '{"note":"console closed"}'}); refreshAlerts(); }
  catch (e) { alert(e.message); }
}

async function recompute() {
  try {
    const payload = JSON.parse($('#payload').value);
    const res = await api('/api/telemetry/checksum', {
      method: 'POST', headers: {'Content-Type':'application/json'},
      body: JSON.stringify({ points: payload.points })
    });
    payload.checksum = res.checksum;
    $('#payload').value = JSON.stringify(payload, null, 2);
    $('#ingest-result').textContent = `checksum = ${res.checksum}，已回填`;
  } catch (e) { $('#ingest-result').textContent = `错误：${e.message}`; }
}

async function ingest() {
  try {
    const payload = JSON.parse($('#payload').value);
    const res = await api('/api/telemetry/batches', {
      method: 'POST', headers: {'Content-Type':'application/json'},
      body: JSON.stringify(payload)
    });
    $('#ingest-result').textContent = JSON.stringify(res, null, 2);
    refreshDashboard();
  } catch (e) { $('#ingest-result').textContent = `错误：${e.message}`; }
}

function tickClock() { $('#clock').textContent = new Date().toISOString(); }

refreshLines(); refreshDashboard(); refreshAlerts();
setInterval(refreshLines, 10000);
setInterval(refreshDashboard, 10000);
setInterval(refreshAlerts, 10000);
tickClock(); setInterval(tickClock, 1000);
