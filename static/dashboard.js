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
  if (!iso) return '—';
  const d = new Date(iso);
  const now = new Date();
  const diffMs = now - d;
  const diffM = Math.floor(diffMs / 60000);
  const diffH = Math.floor(diffMs / 3600000);
  if (diffM < 1) return 'Updated just now';
  if (diffM < 60) return `Updated ${diffM} min ago`;
  if (diffH < 24) return `Updated ${diffH} hours ago`;
  return `Updated ${Math.floor(diffH / 24)} days ago`;
}

function fmtPrice(p) {
  if (p == null || p === 0) return '—';
  return '$' + Number(p).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

function fmtPct(n) {
  if (n == null) return '—';
  const cls = n >= 0 ? 'up' : 'down';
  const s = (n >= 0 ? '+' : '') + n + '%';
  return `<span class="${cls}">${s}</span>`;
}

function renderCompany(c) {
  const price = c.price ? fmtPrice(c.price) : '—';
  const qTrend = fmtPct(c.quarter_trend);
  let newsHtml = '—';
  if (c.news && c.news.length > 0) {
    newsHtml = c.news.map(n => `<a href="${n.url}" target="_blank" rel="noopener">${escapeHtml(n.title)}</a>`).join('<br>');
  }
  let insidersHtml = '—';
  if (c.top_insiders && c.top_insiders.length > 0) {
    insidersHtml = c.top_insiders.map(i =>
      `<span>${escapeHtml(i.name)} (${i.role || '—'}) — ${formatNum(i.shares)} shares</span>`
    ).join('');
  }
  return `
    <tr>
      <td><span class="sym">${escapeHtml(c.symbol)}</span></td>
      <td>${escapeHtml(c.name)}</td>
      <td class="price">${price}</td>
      <td>${qTrend}</td>
      <td class="news-cell">${newsHtml}</td>
      <td class="insiders">${insidersHtml}</td>
    </tr>
  `;
}

function formatNum(n) {
  if (n >= 1e6) return (n / 1e6).toFixed(1) + 'M';
  if (n >= 1e3) return (n / 1e3).toFixed(1) + 'K';
  return String(Math.round(n));
}

function escapeHtml(s) {
  const d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}

async function loadDashboard() {
  showLoading();
  try {
    const res = await fetch('/api/dashboard');
    const data = await res.json();
    hideLoading();

    if (data.error) {
      showError(data.error);
      return;
    }

    lastUpdatedEl.textContent = formatLastUpdated(data._cached_at);

    sectorsEl.innerHTML = '';
    const sectors = data.sectors || [];
    if (sectors.length === 0) {
      sectorsEl.innerHTML = '<p class="empty-state">Dashboard is being built. This usually takes 2–3 minutes on first run. Check back shortly.</p>';
      return;
    }

    sectors.forEach(sec => {
      const section = document.createElement('div');
      section.className = 'sector';
      const rows = (sec.companies || []).map(renderCompany).join('');
      section.innerHTML = `
        <div class="sector-header">${escapeHtml(sec.name)}</div>
        <table class="sector-table">
          <thead>
            <tr>
              <th>Symbol</th>
              <th>Company</th>
              <th>Price</th>
              <th>Quarterly Trend</th>
              <th>Recent News</th>
              <th>Top Insider Sellers</th>
            </tr>
          </thead>
          <tbody>${rows}</tbody>
        </table>
      `;
      sectorsEl.appendChild(section);
    });
  } catch (err) {
    showError('We couldn’t load the dashboard. It may be updating — try again in a minute.');
  }
}

document.addEventListener('DOMContentLoaded', loadDashboard);
