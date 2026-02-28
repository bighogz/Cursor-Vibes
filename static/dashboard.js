const loadingEl = document.getElementById('loading');
const errorEl = document.getElementById('error');
const sectorsEl = document.getElementById('sectors');
const lastUpdatedEl = document.getElementById('lastUpdated');

function showLoading() {
  errorEl.style.display = 'none';
  sectorsEl.innerHTML = '';
  loadingEl.style.display = 'block';
}

function hideLoading() {
  loadingEl.style.display = 'none';
}

function showError(msg) {
  hideLoading();
  errorEl.textContent = msg;
  errorEl.style.display = 'block';
}

function formatLastUpdated(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  const now = new Date();
  const diffMs = now - d;
  const diffM = Math.floor(diffMs / 60000);
  const diffH = Math.floor(diffMs / 3600000);
  if (diffM < 1) return 'Updated just now';
  if (diffM < 60) return `Updated ${diffM}m ago`;
  if (diffH < 24) return `Updated ${diffH}h ago`;
  return `Updated ${Math.floor(diffH / 24)}d ago`;
}

function fmtPrice(p) {
  if (p == null || p === 0) return '<span class="muted">—</span>';
  return '$' + Number(p).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

function fmtPct(n) {
  if (n == null) return null;
  return (n >= 0 ? '+' : '') + n.toFixed(2) + '%';
}

function escapeHtml(s) {
  if (!s) return '';
  const d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}

function formatNum(n) {
  if (n >= 1e6) return (n / 1e6).toFixed(1) + 'M';
  if (n >= 1e3) return (n / 1e3).toFixed(1) + 'K';
  return String(Math.round(n));
}

// SVG sparkline from an array of numbers
function sparklineSVG(closes, isUp) {
  if (!closes || closes.length < 2) return '';
  const w = 100, h = 32, pad = 2;
  const min = Math.min(...closes);
  const max = Math.max(...closes);
  const range = max - min || 1;
  const pts = closes.map((v, i) => {
    const x = pad + (i / (closes.length - 1)) * (w - pad * 2);
    const y = pad + (1 - (v - min) / range) * (h - pad * 2);
    return `${x.toFixed(1)},${y.toFixed(1)}`;
  });
  const color = isUp ? '#22c55e' : '#ef4444';
  const fill = isUp ? 'rgba(34,197,94,0.08)' : 'rgba(239,68,68,0.08)';
  const lastPt = pts[pts.length - 1];
  const areaPath = `M${pts[0]} ${pts.join(' L')} L${lastPt.split(',')[0]},${h} L${pts[0].split(',')[0]},${h} Z`;
  return `<svg class="sparkline" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}">
    <path d="${areaPath}" fill="${fill}" />
    <polyline points="${pts.join(' ')}" fill="none" stroke="${color}" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
  </svg>`;
}

function renderTrend(c) {
  const pct = c.quarter_trend;
  const closes = c.quarter_closes;
  if (pct == null) return '<span class="muted">—</span>';
  const isUp = pct >= 0;
  const cls = isUp ? 'up' : 'down';
  const badge = `<span class="trend-badge ${cls}">${fmtPct(pct)}</span>`;
  const spark = sparklineSVG(closes, isUp);
  return `<div class="trend-cell">${spark}${badge}</div>`;
}

function renderCompany(c) {
  const price = fmtPrice(c.price);
  const trendHtml = renderTrend(c);

  let newsHtml = '<span class="muted">—</span>';
  if (c.news && c.news.length > 0) {
    newsHtml = c.news.map(n =>
      `<a href="${n.url || n.link}" target="_blank" rel="noopener">${escapeHtml(n.title)}</a>`
    ).join('');
  }

  let insidersHtml = '<span class="muted">—</span>';
  if (c.top_insiders && c.top_insiders.length > 0) {
    insidersHtml = c.top_insiders.map(i =>
      `<span>${escapeHtml(i.name)} — ${formatNum(i.shares)}</span>`
    ).join('');
  }

  return `<tr>
    <td><span class="sym">${escapeHtml(c.symbol)}</span></td>
    <td class="company-name" title="${escapeHtml(c.name)}">${escapeHtml(c.name)}</td>
    <td class="price-cell">${price}</td>
    <td>${trendHtml}</td>
    <td class="news-cell">${newsHtml}</td>
    <td class="insiders">${insidersHtml}</td>
  </tr>`;
}

function buildDashboardUrl() {
  const sector = document.getElementById('sectorFilter').value;
  const limit = document.getElementById('limitFilter')?.value?.trim();
  const params = new URLSearchParams();
  if (sector) params.set('sector', sector);
  const limitNum = limit ? parseInt(limit, 10) : 50;
  params.set('limit', limitNum > 0 ? limitNum : 50);
  const qs = params.toString();
  return '/api/dashboard' + (qs ? '?' + qs : '');
}

function populateSectorFilter(sectors) {
  const sel = document.getElementById('sectorFilter');
  if (!sel || !sectors || sectors.length === 0) return;
  const first = sel.options[0];
  sel.innerHTML = '';
  sel.appendChild(first);
  sectors.forEach(s => {
    const opt = document.createElement('option');
    opt.value = s;
    opt.textContent = s;
    sel.appendChild(opt);
  });
}

async function loadDashboard() {
  showLoading();
  try {
    const url = buildDashboardUrl();
    const res = await fetch(url);
    const data = await res.json();
    hideLoading();

    if (data.error) {
      showError(data.error);
      return;
    }

    lastUpdatedEl.textContent = formatLastUpdated(data._cached_at || data.as_of);
    const availableSectors = data.available_sectors || data.sectors?.map(s => s.name) || [];
    populateSectorFilter(availableSectors);

    sectorsEl.innerHTML = '';
    const sectors = data.sectors || [];
    if (sectors.length === 0) {
      sectorsEl.innerHTML = '<p class="empty-state">Dashboard is being built. This usually takes 2–3 minutes on first run.</p>';
      return;
    }

    sectors.forEach(sec => {
      const companies = sec.companies || [];
      const section = document.createElement('div');
      section.className = 'sector';
      const rows = companies.map(renderCompany).join('');
      section.innerHTML = `
        <div class="sector-header">
          ${escapeHtml(sec.name)}
          <span class="sector-count">${companies.length}</span>
        </div>
        <table class="sector-table">
          <colgroup>
            <col class="col-sym"><col class="col-name"><col class="col-price">
            <col class="col-trend"><col class="col-news"><col class="col-ins">
          </colgroup>
          <thead>
            <tr>
              <th>Symbol</th>
              <th>Company</th>
              <th>Price</th>
              <th>Quarterly Trend</th>
              <th>Recent News</th>
              <th>Insiders</th>
            </tr>
          </thead>
          <tbody>${rows}</tbody>
        </table>`;
      sectorsEl.appendChild(section);
    });
  } catch (err) {
    showError('Could not load the dashboard. It may be updating — try again in a minute.');
  }
}

document.addEventListener('DOMContentLoaded', () => {
  loadDashboard();
  document.getElementById('applyFilters')?.addEventListener('click', loadDashboard);
});
