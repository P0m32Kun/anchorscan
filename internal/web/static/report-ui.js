function renderVulnDistribution() {
  const container = document.getElementById('distribution-container');
  const bar = document.getElementById('distribution-bar');
  const legend = document.getElementById('distribution-legend');
  if (!container || !bar || !legend) return;

  const badges = document.querySelectorAll('.severity-badge');
  if (badges.length === 0) {
    container.style.display = 'none';
    return;
  }

  const counts = { critical: 0, high: 0, medium: 0, low: 0, info: 0 };
  badges.forEach(badge => {
    const text = badge.textContent.trim().toLowerCase();
    if (counts.hasOwnProperty(text)) {
      counts[text]++;
    }
  });

  const total = Object.values(counts).reduce((a, b) => a + b, 0);
  if (total === 0) {
    container.style.display = 'none';
    return;
  }

  container.style.display = 'block';

  let barHTML = '';
  let legendHTML = '';

  const labelMap = {
    critical: '严重 (Critical)',
    high: '高危 (High)',
    medium: '中危 (Medium)',
    low: '低危 (Low)',
    info: '信息 (Info)'
  };

  Object.entries(counts).forEach(([sev, count]) => {
    if (count > 0) {
      const pct = ((count / total) * 100).toFixed(1);
      barHTML += `<div class="vuln-bar-segment ${sev}" style="width: ${pct}%;" title="${labelMap[sev]}: ${count} (${pct}%)"></div>`;
    }
    legendHTML += `
      <span class="legend-item">
        <span class="legend-dot ${sev}"></span>
        ${labelMap[sev]}: <span class="legend-count">${count}</span>
      </span>
    `;
  });

  bar.innerHTML = barHTML;
  legend.innerHTML = legendHTML;
}

document.addEventListener('DOMContentLoaded', () => {
  renderVulnDistribution();
  initPageJumpForms();
});

// initPageJumpForms wires up the "jump to page" forms so submitting them
// preserves all other query parameters (filters, size, view, etc.) instead of
// replacing the whole query string with just the page number.
function initPageJumpForms() {
  document.querySelectorAll('form.page-jump').forEach(form => {
    form.addEventListener('submit', event => {
      event.preventDefault();
      const pageInput = form.querySelector('input[type="number"]');
      if (!pageInput) return;
      const page = parseInt(pageInput.value, 10);
      if (!Number.isFinite(page) || page < 1) return;
      const max = parseInt(pageInput.max, 10);
      if (Number.isFinite(max) && page > max) return;

      const params = new URLSearchParams(window.location.search);
      params.set(pageInput.name, String(page));
      window.location.search = params.toString();
    });
  });
}
