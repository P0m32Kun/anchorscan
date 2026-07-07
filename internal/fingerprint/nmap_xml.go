package fingerprint

import (
	"encoding/xml"
	"strconv"
)

type nmapRun struct {
	Hosts []nmapHost `xml:"host"`
}

type nmapHost struct {
	Address nmapAddress `xml:"address"`
	Ports   []nmapPort  `xml:"ports>port"`
}

type nmapAddress struct {
	Addr string `xml:"addr,attr"`
}

type nmapPort struct {
	Protocol string      `xml:"protocol,attr"`
	PortID   string      `xml:"portid,attr"`
	State    nmapState   `xml:"state"`
	Service  nmapService `xml:"service"`
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

func ParseNmapXML(data []byte) ([]ServiceFingerprint, error) {
	var doc nmapRun
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	var out []ServiceFingerprint
	for _, host := range doc.Hosts {
		for _, port := range host.Ports {
			if port.State.State != "open" {
				continue
			}
			portID, err := strconv.Atoi(port.PortID)
			if err != nil {
				return nil, err
			}
			out = append(out, ServiceFingerprint{
				IP:        host.Address.Addr,
				Port:      portID,
				Protocol:  port.Protocol,
				Service:   port.Service.Name,
				Product:   port.Service.Product,
				Version:   port.Service.Version,
				ExtraInfo: port.Service.ExtraInfo,
				Tunnel:    port.Service.Tunnel,
			})
		}
	}

	return out, nil
}
