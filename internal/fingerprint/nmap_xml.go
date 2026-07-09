package fingerprint

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type nmapRun struct {
	XMLName     xml.Name     `xml:"nmaprun"`
	Hosts       []nmapHost   `xml:"host"`
	Prescripts  []nmapScript `xml:"prescripts>script"`
	Postscripts []nmapScript `xml:"postscripts>script"`
}

type nmapHost struct {
	Address     nmapAddress `xml:"address"`
	Ports       []nmapPort  `xml:"ports>port"`
	Hostscripts []nmapScript `xml:"hostscript>script"`
}

type nmapAddress struct {
	Addr string `xml:"addr,attr"`
}

type nmapPort struct {
	Protocol string       `xml:"protocol,attr"`
	PortID   string       `xml:"portid,attr"`
	State    nmapState    `xml:"state"`
	Service  nmapService  `xml:"service"`
	Scripts  []nmapScript `xml:"script"`
	CPEs     []string     `xml:"cpe"`
}

type nmapState struct {
	State string `xml:"state,attr"`
}

type nmapService struct {
	Name      string `xml:"name,attr"`
	Product   string `xml:"product,attr"`
	Version   string `xml:"version,attr"`
	ExtraInfo string `xml:"extrainfo,attr"`
	Tunnel    string `xml:"tunnel,attr"`
}

type nmapScript struct {
	ID     string `xml:"id,attr"`
	Output string `xml:"output,attr"`
}

func ParseNmapXML(data []byte) ([]ServiceFingerprint, []ImportedScript, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil, errors.New("empty XML file")
	}

	var root struct {
		XMLName xml.Name
	}
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, nil, fmt.Errorf("invalid Nmap XML: %w", err)
	}
	if root.XMLName.Local != "nmaprun" {
		return nil, nil, errors.New("root element is not nmaprun")
	}

	var doc nmapRun
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, nil, fmt.Errorf("invalid Nmap XML: %w", err)
	}

	var fps []ServiceFingerprint
	var scripts []ImportedScript
	for _, host := range doc.Hosts {
		for _, port := range host.Ports {
			if port.State.State != "open" {
				continue
			}
			portID, err := strconv.Atoi(port.PortID)
			if err != nil {
				return nil, nil, err
			}
			fps = append(fps, ServiceFingerprint{
				IP:        host.Address.Addr,
				Port:      portID,
				Protocol:  port.Protocol,
				Service:   port.Service.Name,
				Product:   port.Service.Product,
				Version:   port.Service.Version,
				ExtraInfo: port.Service.ExtraInfo,
				Tunnel:    port.Service.Tunnel,
				CPE:       strings.Join(port.CPEs, "\n"),
			})
			for _, script := range port.Scripts {
				scripts = append(scripts, ImportedScript{
					Scope:    "port",
					IP:       host.Address.Addr,
					Port:     portID,
					Protocol: port.Protocol,
					ID:       script.ID,
					Output:   script.Output,
				})
			}
		}
		for _, script := range host.Hostscripts {
			scripts = append(scripts, ImportedScript{
				Scope: "host",
				IP:    host.Address.Addr,
				ID:    script.ID,
				Output: script.Output,
			})
		}
	}
	for _, script := range doc.Prescripts {
		scripts = append(scripts, ImportedScript{
			Scope:  "pre",
			ID:     script.ID,
			Output: script.Output,
		})
	}
	for _, script := range doc.Postscripts {
		scripts = append(scripts, ImportedScript{
			Scope:  "post",
			ID:     script.ID,
			Output: script.Output,
		})
	}

	return fps, scripts, nil
}
