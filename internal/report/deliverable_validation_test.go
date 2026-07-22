package report

import (
	"strings"
	"testing"
	"time"
)

func validDeliverable() ProjectDeliverable {
	return ProjectDeliverable{
		Project: ProjectMetadata{
			ReportTitle: "报告", ClientUnit: "单位", TestObject: "系统",
			StartDate: "2026-07-01", EndDate: "2026-07-02", Testers: "张三",
			CreatedAt: time.Unix(1, 0),
		},
		Zones: []DeliverableZone{{
			Zone: DeliverableZoneInfo{Name: "I区"},
			Sessions: []DeliverableSession{{
				Label: "run-1", AccessPoint: "SW-01", TesterIP: "10.0.0.10", Targets: "10.0.1.0/24",
			}},
			Confirmed: []DeliverableVerification{{
				Title: "弱口令", Description: "描述", Remediation: "整改",
				Assets:   []DeliverableAsset{{IP: "10.0.0.1", Port: 22}},
				Evidence: []DeliverableEvidence{{FilePath: "/tmp/evidence.png"}},
			}},
			NotObserved: []DeliverableVerification{{
				Title: "Redis未授权", Assets: []DeliverableAsset{{IP: "10.0.0.2", Port: 6379}},
				Evidence: []DeliverableEvidence{{FilePath: "/tmp/negative.png"}},
			}},
		}},
		Stats: DeliverableStats{Total: 1, High: 1},
	}
}

func TestValidateProjectDeliverableAcceptsCompleteReport(t *testing.T) {
	if err := ValidateProjectDeliverable(validDeliverable()); err != nil {
		t.Fatalf("ValidateProjectDeliverable returned error: %v", err)
	}
}

func TestValidateProjectDeliverableRejectsMissingFormalFields(t *testing.T) {
	tests := []struct {
		name string
		want string
		edit func(*ProjectDeliverable)
	}{
		{name: "created at", want: "项目创建时间", edit: func(d *ProjectDeliverable) { d.Project.CreatedAt = time.Time{} }},
		{name: "access point", want: "接入点", edit: func(d *ProjectDeliverable) { d.Zones[0].Sessions[0].AccessPoint = "" }},
		{name: "tester ip", want: "测试机 IP", edit: func(d *ProjectDeliverable) { d.Zones[0].Sessions[0].TesterIP = "" }},
		{name: "description", want: "漏洞描述", edit: func(d *ProjectDeliverable) { d.Zones[0].Confirmed[0].Description = "" }},
		{name: "remediation", want: "修改建议", edit: func(d *ProjectDeliverable) { d.Zones[0].Confirmed[0].Remediation = "" }},
		{name: "confirmed asset", want: "关联资产", edit: func(d *ProjectDeliverable) { d.Zones[0].Confirmed[0].Assets = nil }},
		{name: "not observed asset", want: "端口资产", edit: func(d *ProjectDeliverable) { d.Zones[0].NotObserved[0].Assets = nil }},
		{name: "critical", want: "critical", edit: func(d *ProjectDeliverable) { d.Stats.Critical = 1 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			deliverable := validDeliverable()
			test.edit(&deliverable)
			err := ValidateProjectDeliverable(deliverable)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q error, got %v", test.want, err)
			}
		})
	}
}

func TestValidateProjectDeliverableRejectsCriticalHiddenByCrossZoneSummaryDedup(t *testing.T) {
	deliverable := validDeliverable()
	deliverable.Zones[0].Confirmed[0].VulnerabilityKey = "CVE-2026-0001"
	deliverable.Zones[0].Confirmed[0].Severity = "high"
	critical := deliverable.Zones[0].Confirmed[0]
	critical.ZoneID = "II"
	critical.Severity = "critical"
	deliverable.Zones = append(deliverable.Zones, DeliverableZone{
		Zone:      DeliverableZoneInfo{ZoneID: "II", Name: "II区"},
		Confirmed: []DeliverableVerification{critical},
	})
	deliverable.Stats.Critical = 0

	err := ValidateProjectDeliverable(deliverable)
	if err == nil || !strings.Contains(err.Error(), "critical") {
		t.Fatalf("expected critical error, got %v", err)
	}
}
