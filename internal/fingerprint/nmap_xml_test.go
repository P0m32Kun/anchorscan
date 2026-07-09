package fingerprint

import (
	"strings"
	"testing"
)

func TestParseNmapXMLExtractsServiceFields(t *testing.T) {
	xmlInput := []byte(`
<nmaprun>
  <host>
    <address addr="192.168.1.10" addrtype="ipv4"/>
    <ports>
      <port protocol="tcp" portid="8443">
        <state state="open"/>
        <service name="ssl/http" product="nginx" version="1.24.0" tunnel="ssl"/>
      </port>
    </ports>
  </host>
</nmaprun>`)

	got, _, err := ParseNmapXML(xmlInput)
	if err != nil {
		t.Fatalf("ParseNmapXML returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("unexpected fingerprint count: %d", len(got))
	}
	if got[0].Service != "ssl/http" || got[0].Product != "nginx" || got[0].Tunnel != "ssl" {
		t.Fatalf("unexpected fingerprint: %#v", got[0])
	}
}

func TestParseNmapXMLKeepsTCPAndUDPSamePort(t *testing.T) {
	xmlInput := []byte(`
<nmaprun>
  <host>
    <address addr="10.0.0.53"/>
    <ports>
      <port protocol="tcp" portid="53">
        <state state="open"/>
        <service name="domain" product="BIND" version="9.18"/>
      </port>
      <port protocol="udp" portid="53">
        <state state="open"/>
        <service name="domain" product="BIND" version="9.18"/>
      </port>
    </ports>
  </host>
</nmaprun>`)

	got, _, err := ParseNmapXML(xmlInput)
	if err != nil {
		t.Fatalf("ParseNmapXML returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected two fingerprints for tcp+udp, got %d", len(got))
	}
	protocols := map[string]bool{}
	for _, fp := range got {
		if fp.Port != 53 {
			t.Fatalf("unexpected port: %d", fp.Port)
		}
		protocols[fp.Protocol] = true
	}
	if !protocols["tcp"] || !protocols["udp"] {
		t.Fatalf("expected both tcp and udp, got %v", protocols)
	}
}

func TestParseNmapXMLParsesCPE(t *testing.T) {
	xmlInput := []byte(`
<nmaprun>
  <host>
    <address addr="10.0.0.2"/>
    <ports>
      <port protocol="tcp" portid="22">
        <state state="open"/>
        <service name="ssh" product="OpenBSD OpenSSH" version="9.0"/>
        <cpe>cpe:/a:openbsd:openssh:9.0</cpe>
        <cpe>cpe:/o:linux:linux_kernel:5</cpe>
      </port>
    </ports>
  </host>
</nmaprun>`)

	got, _, err := ParseNmapXML(xmlInput)
	if err != nil {
		t.Fatalf("ParseNmapXML returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("unexpected fingerprint count: %d", len(got))
	}
	if !strings.Contains(got[0].CPE, "cpe:/a:openbsd:openssh:9.0") || !strings.Contains(got[0].CPE, "cpe:/o:linux:linux_kernel:5") {
		t.Fatalf("CPE not preserved: %q", got[0].CPE)
	}
}

func TestParseNmapXMLParsesPortScript(t *testing.T) {
	xmlInput := []byte(`
<nmaprun>
  <host>
    <address addr="10.0.0.3"/>
    <ports>
      <port protocol="tcp" portid="80">
        <state state="open"/>
        <service name="http"/>
        <script id="http-methods" output="Supported methods: GET HEAD POST"/>
      </port>
    </ports>
  </host>
</nmaprun>`)

	_, scripts, err := ParseNmapXML(xmlInput)
	if err != nil {
		t.Fatalf("ParseNmapXML returned error: %v", err)
	}
	if len(scripts) != 1 {
		t.Fatalf("expected one script, got %d", len(scripts))
	}
	s := scripts[0]
	if s.Scope != "port" || s.IP != "10.0.0.3" || s.Port != 80 || s.Protocol != "tcp" || s.ID != "http-methods" {
		t.Fatalf("unexpected port script: %#v", s)
	}
	if !strings.Contains(s.Output, "Supported methods") {
		t.Fatalf("output not preserved: %q", s.Output)
	}
}

func TestParseNmapXMLParsesHostScript(t *testing.T) {
	xmlInput := []byte(`
<nmaprun>
  <host>
    <address addr="10.0.0.4"/>
    <hostscript>
      <script id="ssh-hostkey" output="2048 aa:bb (RSA)"/>
    </hostscript>
  </host>
</nmaprun>`)

	_, scripts, err := ParseNmapXML(xmlInput)
	if err != nil {
		t.Fatalf("ParseNmapXML returned error: %v", err)
	}
	if len(scripts) != 1 {
		t.Fatalf("expected one hostscript, got %d", len(scripts))
	}
	s := scripts[0]
	if s.Scope != "host" || s.IP != "10.0.0.4" || s.Port != 0 || s.ID != "ssh-hostkey" {
		t.Fatalf("unexpected host script: %#v", s)
	}
}

func TestParseNmapXMLParsesPreAndPostScripts(t *testing.T) {
	xmlInput := []byte(`
<nmaprun>
  <prescripts>
    <script id="whois" output="querying whois..."/>
  </prescripts>
  <host>
    <address addr="10.0.0.5"/>
    <ports>
      <port protocol="tcp" portid="443">
        <state state="open"/>
        <service name="https"/>
      </port>
    </ports>
  </host>
  <postscripts>
    <script id="http-title" output="Welcome"/>
  </postscripts>
</nmaprun>`)

	_, scripts, err := ParseNmapXML(xmlInput)
	if err != nil {
		t.Fatalf("ParseNmapXML returned error: %v", err)
	}
	scopes := map[string]string{}
	for _, s := range scripts {
		scopes[s.Scope] = s.ID
	}
	if scopes["pre"] != "whois" {
		t.Fatalf("prescript not captured: %#v", scopes)
	}
	if scopes["post"] != "http-title" {
		t.Fatalf("postscript not captured: %#v", scopes)
	}
}

func TestParseNmapXMLRejectsEmptyInput(t *testing.T) {
	_, _, err := ParseNmapXML([]byte{})
	if err == nil || !strings.Contains(err.Error(), "empty XML file") {
		t.Fatalf("expected empty XML error, got: %v", err)
	}
	// whitespace-only also counts as empty
	_, _, err = ParseNmapXML([]byte("   \n\t  "))
	if err == nil || !strings.Contains(err.Error(), "empty XML file") {
		t.Fatalf("expected empty XML error for whitespace, got: %v", err)
	}
}

func TestParseNmapXMLRejectsNonNmaprunRoot(t *testing.T) {
	xmlInput := []byte(`<foo><bar/></foo>`)

	_, _, err := ParseNmapXML(xmlInput)
	if err == nil || !strings.Contains(err.Error(), "root element is not nmaprun") {
		t.Fatalf("expected non-nmaprun error, got: %v", err)
	}
}

func TestParseNmapXMLRejectsInvalidXML(t *testing.T) {
	xmlInput := []byte(`<nmaprun><host><address addr="1.2.3.4"`)

	_, _, err := ParseNmapXML(xmlInput)
	if err == nil || !strings.Contains(err.Error(), "invalid Nmap XML") {
		t.Fatalf("expected invalid XML error, got: %v", err)
	}
}
