package web

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/app"
	"github.com/P0m32Kun/anchorscan/internal/config"
	"github.com/P0m32Kun/anchorscan/internal/ports"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

type toolPageData struct {
	Projects      []store.Project
	Tools         []manualTool
	Tool          manualTool
	HighriskPorts string
}

type manualTool struct {
	Name    string
	Title   string
	Summary string
	Help    []string
	Presets []toolPreset
}

type toolPreset struct {
	Label   string
	Hint    string
	RawArgs string
}

func (s *server) toolNew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	projects, err := s.store.ListProjects()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, "templates/tool_new.html", toolPageData{Projects: projects, Tools: manualTools()})
}

func (s *server) toolPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	toolName := strings.Trim(strings.TrimPrefix(r.URL.Path, "/tools/"), "/")
	tool, ok := manualToolByName(toolName)
	if !ok {
		http.NotFound(w, r)
		return
	}
	projects, err := s.store.ListProjects()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	highriskPorts, _ := ports.LoadPresetForConfig("highrisk", s.opts.ConfigPath)
	render(w, "templates/tool_page.html", toolPageData{Projects: projects, Tool: tool, HighriskPorts: highriskPorts})
}

func (s *server) toolCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	cfg, err := config.Load(s.opts.ConfigPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	toolName := strings.TrimSpace(r.FormValue("tool"))
	if !isManualTool(toolName) {
		http.Error(w, "unknown tool", http.StatusBadRequest)
		return
	}
	_, useNativeArgs := r.Form["raw_args"]
	var nativeArgs []string
	if rawArgs := strings.TrimSpace(r.FormValue("raw_args")); rawArgs != "" {
		nativeArgs, err = config.SplitArgs(rawArgs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	mode := coalesce(strings.TrimSpace(r.FormValue("mode")), "service")
	portsValue := strings.TrimSpace(r.FormValue("ports"))
	projectID := strings.TrimSpace(r.FormValue("project_id"))
	targetValue := strings.TrimSpace(r.FormValue("target"))
	urlValue := strings.TrimSpace(r.FormValue("url"))
	tagsValue := r.FormValue("tags")
	templateValue := strings.TrimSpace(r.FormValue("template"))
	extraArgsText := r.FormValue("extra_args")
	if !useNativeArgs && (toolName == "rustscan" || (toolName == "nmap" && mode != "alive")) {
		if portsValue == "" {
			portsValue = cfg.Scan.Ports
		}
		portsValue, err = ports.ResolveForConfig(portsValue, s.opts.ConfigPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	extraArgs, err := config.SplitArgs(extraArgsText)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	runID := newID("tool-"+toolName, s.opts.Now())
	jsonPath := managedReportPath(s.opts.DBPath, projectID, runID)
	if err := os.MkdirAll(filepath.Dir(jsonPath), 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	opts := app.ToolRunOptions{
		RunID:         runID,
		ProjectID:     projectID,
		Tool:          toolName,
		Mode:          mode,
		Target:        targetValue,
		Ports:         portsValue,
		UseNativeArgs: useNativeArgs,
		NativeArgs:    nativeArgs,
		URL:           urlValue,
		Tags:          splitCSV(tagsValue),
		Template:      templateValue,
		Tools: app.ToolPaths{
			Rustscan: cfg.Tools.Rustscan,
			Nmap:     cfg.Tools.Nmap,
			Httpx:    cfg.Tools.Httpx,
			Nuclei:   cfg.Tools.Nuclei,
		},
		JSONReportPath: jsonPath,
	}
	applyToolExtraArgs(&opts, toolName, extraArgs)
	if _, err := s.manager.StartTool(context.Background(), opts); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	if r.Header.Get("X-Requested-With") == "fetch" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"run_id": runID})
		return
	}
	http.Redirect(w, r, "/runs/"+runID, http.StatusSeeOther)
}

func isManualTool(toolName string) bool {
	switch toolName {
	case "rustscan", "nmap", "httpx", "nuclei":
		return true
	default:
		return false
	}
}

func manualToolByName(name string) (manualTool, bool) {
	for _, tool := range manualTools() {
		if tool.Name == name {
			return tool, true
		}
	}
	return manualTool{}, false
}

func manualTools() []manualTool {
	return []manualTool{
		{
			Name:    "rustscan",
			Title:   "Rustscan 单工具调用",
			Summary: "快速发现主机开放端口，适合先摸清资产入口。",
			Help: []string{
				"参数框填写 rustscan 原生参数，例如 -a 192.168.1.10 --ports 80,443。",
				"端口写法保持 rustscan 习惯：--top、--range 100-1000、--ports 80,443。",
				"这里不会再包装参数；你输入什么就拼到 rustscan 后面执行。",
				"需要调速、批量或脚本参数时，直接按 rustscan 原生命令写。",
			},
			Presets: []toolPreset{
				{Label: "快速 Web", Hint: "常见 Web 端口", RawArgs: "-a 192.168.1.10 --ports 80,443,8080,8443"},
				{Label: "常见内网", Hint: "管理与中间件端口", RawArgs: "-a 192.168.1.10 --ports 22,80,443,445,3389,6379,8080"},
				{Label: "全端口慢扫", Hint: "覆盖完整端口", RawArgs: "-a 192.168.1.10 --range 1-65535"},
			},
		},
		{
			Name:    "nmap",
			Title:   "Nmap 单工具调用",
			Summary: "做主机存活验证或已知端口的服务指纹识别。",
			Help: []string{
				"参数框填写 nmap 原生参数，例如 -sn 192.168.1.10。",
				"存活检测常用 -sn；服务识别常用 -sV 加目标和端口。",
				"限速、重试、时序参数也直接写，例如 -T2 --max-retries 2。",
			},
			Presets: []toolPreset{
				{Label: "存活检测", Hint: "只验证主机在线", RawArgs: "-sn 192.168.1.10"},
				{Label: "服务识别", Hint: "识别常见服务", RawArgs: "-sV -p 22,80,443,3389 192.168.1.10"},
				{Label: "轻一点", Hint: "降低重试", RawArgs: "-sV -T2 --max-retries 2 -p 80,443 192.168.1.10"},
			},
		},
		{
			Name:    "httpx",
			Title:   "Httpx 单工具调用",
			Summary: "识别单个 Web URL 的状态码、标题和技术栈。",
			Help: []string{
				"参数框填写 httpx 原生参数，例如 -u http://192.168.1.10:8080。",
				"URL 要写完整协议，适合在发现 Web 端口后单独做指纹补充。",
				"限速、线程、标题和技术识别参数直接按 httpx 原生命令写。",
			},
			Presets: []toolPreset{
				{Label: "基础识别", Hint: "默认参数", RawArgs: "-u http://192.168.1.10:8080"},
				{Label: "限速稳定", Hint: "低并发少误伤", RawArgs: "-u http://192.168.1.10:8080 -rate-limit 20 -threads 5"},
				{Label: "显示更多", Hint: "标题/状态码/技术栈", RawArgs: "-u http://192.168.1.10:8080 -tech-detect -title -status-code"},
			},
		},
		{
			Name:    "nuclei",
			Title:   "Nuclei 单工具调用",
			Summary: "对单个 URL 按 tags 或指定 template 做漏洞模板探测。",
			Help: []string{
				"参数框填写 nuclei 原生参数，例如 -u http://192.168.1.10:8080 -tags cve。",
				"按 nuclei 原生命令习惯填写 -tags、-t、-u 等参数。",
				"限速和并发参数直接写，例如 -rate-limit 5 -c 5。",
			},
			Presets: []toolPreset{
				{Label: "CVE 检测", Hint: "常见 CVE 模板", RawArgs: "-u http://192.168.1.10:8080 -tags cve"},
				{Label: "暴露面检测", Hint: "配置暴露类", RawArgs: "-u http://192.168.1.10:8080 -tags exposure"},
				{Label: "稳定限速", Hint: "低速低并发", RawArgs: "-u http://192.168.1.10:8080 -tags cve -rate-limit 5 -c 5"},
			},
		},
	}
}

func applyToolExtraArgs(opts *app.ToolRunOptions, toolName string, args []string) {
	switch toolName {
	case "rustscan":
		opts.ExtraArgs.Rustscan = args
	case "nmap":
		opts.ExtraArgs.Nmap = args
	case "httpx":
		opts.ExtraArgs.Httpx = args
	case "nuclei":
		opts.ExtraArgs.Nuclei = args
	}
}

func splitCSV(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		if item := strings.TrimSpace(part); item != "" {
			out = append(out, item)
		}
	}
	return out
}
