const runBtn = document.getElementById('runBtn');
const limitInput = document.getElementById('limit');
const baselineInput = document.getElementById('baseline');
const currentInput = document.getElementById('current');
const stdInput = document.getElementById('std');
const statsEl = document.getElementById('stats');
const errorEl = document.getElementById('error');
const loadingEl = document.getElementById('loading');
const anomaliesSection = document.getElementById('anomaliesSection');
const anomaliesTable = document.getElementById('anomaliesTable');
const signalsSection = document.getElementById('signalsSection');
const signalsTable = document.getElementById('signalsTable');

function hideAll() {
  statsEl.style.display = 'none';
  errorEl.style.display = 'none';
  loadingEl.style.display = 'none';
  anomaliesSection.style.display = 'none';
  signalsSection.style.display = 'none';
}

function showError(msg) {
  hideAll();
  errorEl.textContent = msg;
  errorEl.style.display = 'block';
}

function formatNum(n) {
  if (n >= 1e6) return (n / 1e6).toFixed(1) + 'M';
  if (n >= 1e3) return (n / 1e3).toFixed(1) + 'K';
  return String(n);
}

function zClass(z) {
  if (z >= 3) return 'z-very-high';
  if (z >= 2) return 'z-high';
  return '';
}

function renderTable(rows, showAll) {
  if (!rows || rows.length === 0) {
    return '<div class="empty">No data</div>';
  }
  const toShow = showAll ? rows : rows.filter(r => r.is_anomaly);
  if (toShow.length === 0) {
    return '<div class="empty">None detected</div>';
  }
  let html = '<table><thead><tr>';
  html += '<th>Ticker</th><th>Current (shares)</th><th>Baseline mean</th><th>Baseline std</th><th>Z-score</th>';
  html += '</tr></thead><tbody>';
  toShow.forEach(r => {
    html += `<tr>
      <td class="ticker">${escapeHtml(r.ticker)}</td>
      <td>${formatNum(r.current_shares_sold)}</td>
      <td>${formatNum(r.baseline_mean)}</td>
      <td>${formatNum(r.baseline_std)}</td>
      <td class="${zClass(r.z_score)}">${r.z_score.toFixed(2)}</td>
    </tr>`;
  });
  html += '</tbody></table>';
  return html;
}

function escapeHtml(s) {
  const div = document.createElement('div');
  div.textContent = s;
  return div.innerHTML;
}

async function runScan() {
  hideAll();
  errorEl.textContent = '';
  loadingEl.style.display = 'flex';
  runBtn.disabled = true;

  const params = new URLSearchParams({
    limit: limitInput.value || '0',
    baseline_days: baselineInput.value || '365',
    current_days: currentInput.value || '30',
    std_threshold: stdInput.value || '2',
  });

  try {
    const res = await fetch(`/api/scan?${params}`, { method: 'POST' });
    const data = await res.json();

    loadingEl.style.display = 'none';
    runBtn.disabled = false;

    if (data.error) {
      showError(data.error);
      return;
    }

    statsEl.style.display = 'grid';
    statsEl.innerHTML = `
      <div class="stat">
        <div class="stat-label">Tickers scanned</div>
        <div class="stat-value">${data.tickers_count}</div>
      </div>
      <div class="stat">
        <div class="stat-label">Insider records</div>
        <div class="stat-value">${data.records_count}</div>
      </div>
      <div class="stat">
        <div class="stat-label">Anomalies</div>
        <div class="stat-value ${data.anomalies_count > 0 ? 'warn' : 'success'}">${data.anomalies_count}</div>
      </div>
      <div class="stat">
        <div class="stat-label">Date range</div>
        <div class="stat-value" style="font-size:0.9rem">${data.date_from} → ${data.date_to}</div>
      </div>
    `;

    anomaliesSection.style.display = 'block';
    anomaliesTable.innerHTML = renderTable(data.anomalies || data.all_signals, false);

    signalsSection.style.display = 'block';
    signalsTable.innerHTML = renderTable(data.all_signals, true);
  } catch (err) {
    loadingEl.style.display = 'none';
    runBtn.disabled = false;
    showError('Request failed: ' + err.message);
  }
}

runBtn.addEventListener('click', runScan);
