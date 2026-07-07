package fingerprint

import "testing"

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

	got, err := ParseNmapXML(xmlInput)
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
