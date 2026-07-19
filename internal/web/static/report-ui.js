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

// Colorize JSON formatting
function colorizeJSON(json) {
  let escaped = json.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  return escaped.replace(/("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+-]?\d+)?)/g, function (match) {
    let cls = 'hl-number';
    if (/^"/.test(match)) {
      if (/:$/.test(match)) {
        cls = 'hl-key';
      } else {
        cls = 'hl-string';
      }
    } else if (/true|false/.test(match)) {
      cls = 'hl-boolean';
    } else if (/null/.test(match)) {
      cls = 'hl-boolean';
    }
    return `<span class="${cls}">${match}</span>`;
  });
}

// Colorize RAW HTTP request/response
function colorizeRaw(text) {
  let escaped = text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

  // Status line (HTTP/1.1 200 OK)
  escaped = escaped.replace(/^(HTTP\/1\.[01]\s+(\d{3})\s+.*)$/gm, '<span class="hl-status-line">$1</span>');
  
  // Request line (GET / HTTP/1.1)
  escaped = escaped.replace(/^(GET|POST|PUT|DELETE|HEAD|OPTIONS|PATCH)\s+(.+)\s+(HTTP\/1\.[01])$/gm, 
    '<span class="hl-request-line"><span class="hl-method">$1</span> $2 <span class="hl-http-ver">$3</span></span>');

  // Headers (Key: Value)
  escaped = escaped.replace(/^([\w-]+)\s*:\s*(.*)$/gm, 
    '<span class="hl-header-key">$1</span>: <span class="hl-header-val">$2</span>');

  // Danger/Success tags
  escaped = escaped.replace(/\b(vulnerability|vuln|exploit|error|fail|failed|critical|high)\b/gi, '<span class="hl-danger">$1</span>');
  escaped = escaped.replace(/\b(success|ok|safe)\b/gi, '<span class="hl-success">$1</span>');

  return escaped;
}

// Traverse and colorize all evidence elements
function highlightAllEvidences() {
  document.querySelectorAll('.evidence-pre').forEach(element => {
    if (element.dataset.highlighted === 'true') return;
    
    const text = element.textContent;
    if (!text) return;

    if (text.trim().startsWith('{') || text.trim().startsWith('[')) {
      try {
        const obj = JSON.parse(text);
        const formatted = JSON.stringify(obj, null, 2);
        element.innerHTML = colorizeJSON(formatted);
        element.dataset.highlighted = 'true';
        return;
      } catch (e) {
        // Fallback to RAW
      }
    }
    
    element.innerHTML = colorizeRaw(text);
    element.dataset.highlighted = 'true';
  });
}

document.addEventListener('DOMContentLoaded', () => {
  renderVulnDistribution();
  initPageJumpForms();
  highlightAllEvidences();
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
