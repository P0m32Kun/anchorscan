package web

import (
	"strconv"
	"strings"

	"github.com/P0m32Kun/anchorscan/internal/fingerprint"
	"github.com/P0m32Kun/anchorscan/internal/store"
)

const runMetaSummaryLimit = 80

type hostAssetView struct {
	IP        string
	Ports     string
	Services  string
	Products  string
	URLs      string
	CopyPorts string
	CopyPairs string
}

type runMetaView struct {
	Target      string
	Ports       string
	Profile     string
	FullTarget  string
	FullPorts   string
	FullProfile string
}

func newRunMetaView(run store.ScanRun) runMetaView {
	return runMetaView{
		Target:      summarizeRunValue(run.Target),
		Ports:       summarizeRunValue(run.Ports),
		Profile:     summarizeRunValue(run.Profile),
		FullTarget:  run.Target,
		FullPorts:   run.Ports,
		FullProfile: run.Profile,
	}
}

func summarizeRunValue(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= runMetaSummaryLimit {
		return value
	}
	return string(runes[:runMetaSummaryLimit]) + "..."
}

func groupFingerprintsByHost(items []fingerprint.ServiceFingerprint) []hostAssetView {
	type hostAccumulator struct {
		ip       string
		ports    []string
		services []string
		products []string
		urls     []string
		pairs    []string
	}
	order := make([]string, 0)
	hostMap := map[string]*hostAccumulator{}
	for _, item := range items {
		host := hostMap[item.IP]
		if host == nil {
			host = &hostAccumulator{ip: item.IP}
			hostMap[item.IP] = host
			order = append(order, item.IP)
		}
		port := strconv.Itoa(item.Port)
		displayPort := port
		if item.Protocol != "" {
			displayPort = item.Protocol + "/" + port
		}
		host.ports = append(host.ports, displayPort)
		host.services = appendUnique(host.services, item.Service)
		host.products = appendUnique(host.products, item.Product)
		if item.URL != "" {
			host.urls = append(host.urls, item.URL)
		}
		host.pairs = append(host.pairs, item.IP+":"+displayPort)
	}

	out := make([]hostAssetView, 0, len(order))
	for _, ip := range order {
		host := hostMap[ip]
		out = append(out, hostAssetView{
			IP:        host.ip,
			Ports:     strings.Join(host.ports, ","),
			Services:  strings.Join(host.services, ", "),
			Products:  strings.Join(host.products, ", "),
			URLs:      strings.Join(host.urls, "\n"),
			CopyPorts: strings.Join(host.ports, ","),
			CopyPairs: strings.Join(host.pairs, "\n"),
		})
	}
	return out
}

func appendUnique(items []string, value string) []string {
	if strings.TrimSpace(value) == "" {
		return items
	}
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}
