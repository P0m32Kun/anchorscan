package report

import (
	"testing"
	"time"
)

func TestBuildProjectDeliverableProjectsConfirmedAndNegativeByZone(t *testing.T) {
	project := ProjectMetadata{ID: "p1", Name: "甘肃任务", ClientUnit: "示例电力", ReportTitle: "渗透测试报告", TestObject: "生产系统", Testers: "张三"}
	zones := []ProjectZone{{ZoneID: "I", Name: "I区", SortOrder: 0}, {ZoneID: "III", Name: "III区", SortOrder: 1}}
	verifications := []DeliverableVerification{
		{ID: "v1", ZoneID: "I", Outcome: "confirmed", Title: "弱口令", Severity: "high", Position: 2, Assets: []DeliverableAsset{{IP: "10.0.0.1", Port: 22, Display: "10.0.0.1:22"}}, Evidence: []DeliverableEvidence{{DataURI: "data:image/png;base64,AAAA"}}},
		{ID: "v2", ZoneID: "I", Outcome: "confirmed", Title: "过期组件", Severity: "medium", Position: 1, Assets: []DeliverableAsset{{IP: "10.0.0.2", Port: 443, Display: "10.0.0.2:443"}}},
		{ID: "v3", ZoneID: "III", Outcome: "not_observed", Title: "SQL注入不存在", Severity: "high", Assets: []DeliverableAsset{{IP: "10.0.0.3", Port: 80, Display: "10.0.0.3:80"}}},
		{ID: "v4", ZoneID: "I", Outcome: "inconclusive", Title: "无法判定项", Severity: "high"},
	}

	deliverable := BuildProjectDeliverable(project, zones, verifications, time.Unix(100, 0))

	if len(deliverable.Zones) != 2 {
		t.Fatalf("expected 2 zones, got %d", len(deliverable.Zones))
	}
	if len(deliverable.Zones[0].Confirmed) != 2 {
		t.Fatalf("zone I confirmed = %d", len(deliverable.Zones[0].Confirmed))
	}
	// Position 1 (过期组件) before Position 2 (弱口令)
	if deliverable.Zones[0].Confirmed[0].Title != "过期组件" || deliverable.Zones[0].Confirmed[1].Title != "弱口令" {
		t.Fatalf("confirmed order wrong: %#v", deliverable.Zones[0].Confirmed)
	}
	if len(deliverable.Zones[1].NotObserved) != 1 {
		t.Fatalf("zone III not observed = %d", len(deliverable.Zones[1].NotObserved))
	}

	if len(deliverable.Summary) != 2 {
		t.Fatalf("summary rows = %d", len(deliverable.Summary))
	}
	// Summary follows confirmed order: 过期组件 (medium) then 弱口令 (high)
	if deliverable.Summary[0].Number != 1 || deliverable.Summary[0].Title != "过期组件" {
		t.Fatalf("summary[0] = %#v", deliverable.Summary[0])
	}
	if deliverable.Summary[1].Number != 2 || deliverable.Summary[1].Title != "弱口令" || deliverable.Summary[1].Severity != "高危" {
		t.Fatalf("summary[1] = %#v", deliverable.Summary[1])
	}

	if deliverable.Stats != (DeliverableStats{Total: 2, High: 1, Medium: 1}) {
		t.Fatalf("stats = %#v", deliverable.Stats)
	}
	if deliverable.ZoneNames != "I区、III区" {
		t.Fatalf("zone names = %q", deliverable.ZoneNames)
	}
}

func TestBuildProjectDeliverableEmptyWhenNoIncludedVerifications(t *testing.T) {
	zones := []ProjectZone{{ZoneID: "I", Name: "I区"}}
	deliverable := BuildProjectDeliverable(ProjectMetadata{}, zones, nil, time.Unix(1, 0))
	if len(deliverable.Summary) != 0 || deliverable.Stats.Total != 0 || deliverable.ZoneNames != "" {
		t.Fatalf("expected empty deliverable, got %#v", deliverable)
	}
}
