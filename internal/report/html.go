package report

import (
	"html/template"
	"os"
)

const htmlTemplate = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <title>AnchorScan 扫描安全报告</title>
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <style>
    @import url('https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;600&display=swap');
    
    :root {
      --bg: #090a0c;
      --panel: #121316;
      --border: #1f2126;
      --border-strong: #2c2f37;
      --text: #d2d5dc;
      --muted: #666a73;
      --heading: #f0f2f5;
      --primary: #f97316;
      --success: #10b981;
      
      --sev-critical: #f43f5e;
      --sev-critical-soft: rgba(244, 63, 94, 0.06);
      --sev-critical-border: rgba(244, 63, 94, 0.25);
      
      --sev-high: #ff8838;
      --sev-high-soft: rgba(255, 136, 56, 0.06);
      --sev-high-border: rgba(255, 136, 56, 0.25);

      --sev-medium: #facc15;
      --sev-medium-soft: rgba(250, 204, 21, 0.06);
      --sev-medium-border: rgba(250, 204, 21, 0.25);

      --sev-low: #38bdf8;
      --sev-low-soft: rgba(56, 189, 248, 0.06);
      --sev-low-border: rgba(56, 189, 248, 0.25);

      --sev-info: #94a3b8;
      --sev-info-soft: rgba(148, 163, 184, 0.06);
      --sev-info-border: rgba(148, 163, 184, 0.25);

      --radius: 4px;
    }
    
    * { box-sizing: border-box; }
    
    body {
      margin: 0;
      padding: 2.5rem 1.5rem;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", sans-serif;
      background-color: var(--bg);
      color: var(--text);
      line-height: 1.5;
      -webkit-font-smoothing: antialiased;
    }
    
    .container {
      max-width: 1200px;
      margin: 0 auto;
    }
    
    header {
      border-bottom: 1px solid var(--border);
      padding-bottom: 1.25rem;
      margin-bottom: 2rem;
      display: flex;
      justify-content: space-between;
      align-items: flex-end;
    }
    
    h1 {
      margin: 0;
      font-size: 1.65rem;
      font-weight: 700;
      letter-spacing: -0.01em;
      color: var(--heading);
    }
    
    .brand-tag {
      font-size: 0.72rem;
      font-weight: 700;
      color: var(--primary);
      text-transform: uppercase;
      letter-spacing: 0.08em;
      margin-bottom: 0.35rem;
    }
    
    .meta-card {
      background: var(--panel);
      border: 1px solid var(--border);
      border-radius: var(--radius);
      padding: 1rem 1.25rem;
      margin-bottom: 2rem;
      display: flex;
      gap: 2.5rem;
      flex-wrap: wrap;
    }
    
    .meta-item {
      display: flex;
      flex-direction: column;
      gap: 0.25rem;
    }
    
    .meta-label {
      font-size: 0.68rem;
      font-weight: 700;
      color: var(--muted);
      text-transform: uppercase;
      letter-spacing: 0.05em;
    }
    
    .meta-value {
      font-size: 0.95rem;
      font-weight: 600;
      color: var(--heading);
      font-family: 'JetBrains Mono', monospace;
    }
    
    table {
      width: 100%;
      border-collapse: separate;
      border-spacing: 0;
      border: 1px solid var(--border);
      border-radius: var(--radius);
      overflow: hidden;
      margin-bottom: 2.5rem;
      background: #0f1013;
    }
    
    th, td {
      padding: 0.85rem 1.1rem;
      text-align: left;
      border-bottom: 1px solid var(--border);
      vertical-align: top;
    }
    
    th {
      background-color: #14151a;
      color: var(--muted);
      font-size: 0.72rem;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: 0.08em;
    }
    
    tr:last-child td {
      border-bottom: none;
    }
    
    tr:hover td {
      background-color: rgba(255, 255, 255, 0.015);
    }
    
    .ip-cell {
      font-family: 'JetBrains Mono', monospace;
      font-weight: 700;
      color: var(--heading);
    }
    
    .port-badge {
      display: inline-block;
      padding: 0.2rem 0.5rem;
      background: #17181c;
      border: 1px solid var(--border-strong);
      color: var(--heading);
      border-radius: 2px;
      font-family: 'JetBrains Mono', monospace;
      font-size: 0.8rem;
    }
    
    .severity-badge {
      display: inline-flex;
      align-items: center;
      padding: 0.2rem 0.5rem;
      border-radius: 2px;
      font-size: 0.72rem;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      border: 1px solid transparent;
      margin-bottom: 0.25rem;
      font-family: 'JetBrains Mono', monospace;
    }
    
    .sev-critical { color: var(--sev-critical); background: var(--sev-critical-soft); border-color: var(--sev-critical-border); }
    .sev-high { color: var(--sev-high); background: var(--sev-high-soft); border-color: var(--sev-high-border); }
    .sev-medium { color: var(--sev-medium); background: var(--sev-medium-soft); border-color: var(--sev-medium-border); }
    .sev-low { color: var(--sev-low); background: var(--sev-low-soft); border-color: var(--sev-low-border); }
    .sev-info { color: var(--sev-info); background: var(--sev-info-soft); border-color: var(--sev-info-border); }
    
    .finding-item {
      padding: 0.5rem 0;
      border-bottom: 1px dashed var(--border);
    }
    
    .finding-item:last-child {
      border-bottom: none;
      padding-bottom: 0;
    }
    
    .finding-summary {
      font-weight: 600;
      margin-bottom: 0.25rem;
      color: var(--heading);
    }
    
    .finding-meta {
      font-size: 0.78rem;
      color: var(--muted);
    }
    
    .web-badge {
      display: inline-block;
      padding: 0.15rem 0.4rem;
      background: rgba(14, 165, 233, 0.05);
      border: 1px solid rgba(14, 165, 233, 0.25);
      color: #38bdf8;
      border-radius: 2px;
      font-size: 0.7rem;
      font-weight: 700;
    }
    
    @media print {
      body { background: #fff; color: #000; }
      th, td, table { border-color: #ccc; }
      .meta-card { background: #f5f5f5; border-color: #ccc; color: #000; box-shadow: none; }
      .port-badge { border-color: #ccc; color: #000; }
    }
  </style>
</head>
<body>
  <div class="container">
    <header>
      <div>
        <div class="brand-tag">内网安全探测引擎</div>
        <h1>AnchorScan 扫描安全报告</h1>
      </div>
      <div style="font-size: 0.85rem; color: var(--muted); text-align: right;">
        报告自动生成时间: <span style="font-family: 'Outfit'; font-weight: 600;">2026-07-08</span>
      </div>
    </header>
    
    <div class="meta-card">
      <div class="meta-item">
        <span class="meta-label">安全引擎模块</span>
        <span class="meta-value">Rustscan / Nmap / Httpx / Nuclei / NSE</span>
      </div>
      <div class="meta-item">
        <span class="meta-label">报告类型</span>
        <span class="meta-value" style="color: var(--primary);">HTML 离线便携版</span>
      </div>
      <div class="meta-item">
        <span class="meta-label">总发现目标数</span>
        <span class="meta-value">{{len .Hosts}} 个主机</span>
      </div>
    </div>
    
    <table>
      <thead>
        <tr>
          <th style="width: 18%;">主机 IP 地址</th>
          <th style="width: 12%;">开放端口</th>
          <th style="width: 15%;">服务名称</th>
          <th style="width: 18%;">产品指纹</th>
          <th style="width: 10%;">Web 属性</th>
          <th style="width: 27%;">安全漏洞 / 脆弱性发现 (Findings)</th>
        </tr>
      </thead>
      <tbody>
        {{range .Hosts}}
        {{$host := .}}
        {{range .Ports}}
        <tr>
          <td class="ip-cell">{{$host.IP}}</td>
          <td><span class="port-badge">{{.Port}}</span></td>
          <td style="font-weight: 600; color: var(--heading);">{{.Service}}</td>
          <td style="color: var(--muted); font-size: 0.9rem;">{{if .Product}}{{.Product}}{{else}}<span style="color: rgba(255,255,255,0.15);">—</span>{{end}}</td>
          <td>
            {{if .IsWeb}}
            <span class="web-badge">WEB</span>
            <div style="font-size: 0.75rem; font-family: 'JetBrains Mono', monospace; word-break: break-all; margin-top: 0.25rem;">
              <a href="{{.URL}}" target="_blank" style="color: var(--primary);">{{.URL}}</a>
            </div>
            {{else}}
            <span style="color: rgba(255,255,255,0.15);">—</span>
            {{end}}
          </td>
          <td>
            {{range .Findings}}
            <div class="finding-item">
              <div>
                <span class="severity-badge sev-{{.Severity}}">{{.Severity}}</span>
              </div>
              <div class="finding-summary">{{.Summary}}</div>
              <div class="finding-meta">来源: {{.Source}} | 规则: {{.ID}}</div>
            </div>
            {{else}}
            <div style="color: var(--success); font-weight: 600; font-size: 0.85rem;">🛡️ 未发现高危漏洞</div>
            {{end}}
          </td>
        </tr>
        {{end}}
        {{end}}
      </tbody>
    </table>
  </div>
</body>
</html>`

func WriteHTML(path string, scanReport ScanReport) error {
	tpl, err := template.New("report").Parse(htmlTemplate)
	if err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return tpl.Execute(file, scanReport)
}
