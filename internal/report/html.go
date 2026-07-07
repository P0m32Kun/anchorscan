package report

import (
	"html/template"
	"os"
)

const htmlTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>AnchorScan Report</title>
</head>
<body>
  <h1>AnchorScan Report</h1>
  <table border="1" cellspacing="0" cellpadding="6">
    <thead>
      <tr>
        <th>IP</th>
        <th>Port</th>
        <th>Service</th>
        <th>Product</th>
        <th>Web</th>
        <th>URL</th>
        <th>Findings</th>
      </tr>
    </thead>
    <tbody>
      {{range .Hosts}}
      {{$host := .}}
      {{range .Ports}}
      <tr>
        <td>{{$host.IP}}</td>
        <td>{{.Port}}</td>
        <td>{{.Service}}</td>
        <td>{{.Product}}</td>
        <td>{{.IsWeb}}</td>
        <td>{{.URL}}</td>
        <td>
          {{range .Findings}}
          <div><strong>{{.Summary}}</strong> ({{.Severity}}/{{.Source}})</div>
          {{else}}
          <div>-</div>
          {{end}}
        </td>
      </tr>
      {{end}}
      {{end}}
    </tbody>
  </table>
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
