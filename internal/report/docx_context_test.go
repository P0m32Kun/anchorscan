package report

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBuildDocxContextProducesSidecarContract(t *testing.T) {
	project := ProjectMetadata{ReportTitle: "示例电力安全渗透测试分析报告", ClientUnit: "示例电力有限公司", TestObject: "生产控制系统", StartDate: "2026-07-01", EndDate: "2026-07-05", Testers: "张三、李四", CreatedAt: time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)}
	zones := []ProjectZone{{ZoneID: "I", Name: "I区", SortOrder: 0}}
	verifications := []DeliverableVerification{
		{ID: "v1", ZoneID: "I", Outcome: "confirmed", Title: "弱口令", Severity: "high", Description: "弱口令描述", Remediation: "第一条建议\n第二条建议\n第三条建议", Position: 1,
			Assets:   []DeliverableAsset{{IP: "10.0.0.1", Port: 22, Display: "10.0.0.1:22"}},
			Evidence: []DeliverableEvidence{{FilePath: "/data/projects/p1/evidence/a.png"}}},
		{ID: "v2", ZoneID: "I", Outcome: "not_observed", Title: "Redis未授权", Severity: "high", Position: 2,
			Assets:   []DeliverableAsset{{IP: "10.0.0.2", Port: 6379, Display: "10.0.0.2:6379"}},
			Evidence: []DeliverableEvidence{{FilePath: "/data/projects/p1/evidence/b.png"}}},
	}
	deliverable := BuildProjectDeliverable(project, zones, nil, verifications, time.Unix(1, 0))

	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	context := BuildDocxContext(deliverable, now)

	if context.Report.Title != "示例电力安全渗透测试分析报告" {
		t.Fatalf("title = %q", context.Report.Title)
	}
	if context.Report.TestPeriod != "2026-07-01 至 2026-07-05" {
		t.Fatalf("test period = %q", context.Report.TestPeriod)
	}
	if context.Report.ProjectCreatedDate != "2026年6月3日" {
		t.Fatalf("created date = %q", context.Report.ProjectCreatedDate)
	}
	if context.Report.ProjectCreatedMonth != "二零二六年六月" {
		t.Fatalf("month = %q", context.Report.ProjectCreatedMonth)
	}
	if len(context.NetworkZones) != 1 || context.NetworkZones[0].Name != "I区" {
		t.Fatalf("zones = %#v", context.NetworkZones)
	}
	zone := context.NetworkZones[0]
	if len(zone.Confirmed) != 1 || zone.Confirmed[0].Heading != "弱口令（高危）" {
		t.Fatalf("confirmed = %#v", zone.Confirmed)
	}
	if zone.Confirmed[0].Evidence[0].Path != "/data/projects/p1/evidence/a.png" {
		t.Fatalf("evidence path = %#v", zone.Confirmed[0].Evidence)
	}
	if got := zone.Confirmed[0].RemediationLines; !reflect.DeepEqual(got, []string{"第一条建议", "第二条建议", "第三条建议"}) {
		t.Fatalf("remediation lines = %#v", got)
	}
	if len(zone.NotObserved) != 1 || zone.NotObserved[0].PortsText != "6379" {
		t.Fatalf("not observed = %#v", zone.NotObserved)
	}
	if context.SummaryEmpty || len(context.SummaryRows) != 1 {
		t.Fatalf("summary = %#v", context.SummaryRows)
	}
	if context.Conclusion.Total != 1 || context.Conclusion.High != 1 || context.Conclusion.FocusText != "弱口令" {
		t.Fatalf("conclusion = %#v", context.Conclusion)
	}
}

func TestBuildDocxContextDefaultsDateWhenProjectHasNone(t *testing.T) {
	project := ProjectMetadata{ReportTitle: "x", ClientUnit: "u", TestObject: "o", Testers: "t"}
	deliverable := BuildProjectDeliverable(project, []ProjectZone{{ZoneID: "I", Name: "I区"}}, nil, nil, time.Unix(1, 0))
	now := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	context := BuildDocxContext(deliverable, now)
	if context.Report.ProjectCreatedDate != "2026年1月5日" {
		t.Fatalf("default date = %q", context.Report.ProjectCreatedDate)
	}
	if context.Report.ProjectCreatedMonth != "二零二六年一月" {
		t.Fatalf("default month = %q", context.Report.ProjectCreatedMonth)
	}
}

func TestBuildDocxContextCarriesCriticalConclusion(t *testing.T) {
	context := BuildDocxContext(ProjectDeliverable{Stats: DeliverableStats{Critical: 1}}, time.Unix(1, 0))
	if context.Conclusion.Critical != 1 {
		t.Fatalf("critical conclusion = %d", context.Conclusion.Critical)
	}
}

func TestBuildDocxContextNeverEmitsNullSlices(t *testing.T) {
	project := ProjectMetadata{ReportTitle: "x", ClientUnit: "u", TestObject: "o", Testers: "t"}
	// Zone has only a confirmed verification; not_observed stays empty (nil).
	verifications := []DeliverableVerification{
		{ID: "v1", ZoneID: "I", Outcome: "confirmed", Title: "弱口令", Severity: "high", Position: 1},
	}
	deliverable := BuildProjectDeliverable(project, []ProjectZone{{ZoneID: "I", Name: "I区"}}, nil, verifications, time.Unix(1, 0))
	context := BuildDocxContext(deliverable, time.Unix(1, 0))

	encoded, err := json.Marshal(context)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), ":null") {
		t.Fatalf("context JSON contains null array/object — sidecar expects [] or omitted: %s", encoded)
	}
}

func TestBuildDocxContextIncludesReportRunSessions(t *testing.T) {
	project := ProjectMetadata{ReportTitle: "x", ClientUnit: "u", TestObject: "o", Testers: "t"}
	zones := []ProjectZone{{ZoneID: "I", Name: "I区"}}
	runs := []ProjectRun{
		{RunID: "r1", ZoneID: "I", Status: "completed", IncludeInReport: true, Label: "核心交换机", AccessPoint: "SW-01", TesterIP: "10.0.0.10", Target: "10.0.1.1,10.0.1.2 10.0.1.3", ExcludeTargets: "10.0.1.99", ExcludePorts: "22", Notes: "夜间窗口"},
		{RunID: "r2", ZoneID: "I", Status: "completed", IncludeInReport: false, Label: "不纳入"},
	}
	deliverable := BuildProjectDeliverable(project, zones, runs, nil, time.Unix(1, 0))
	context := BuildDocxContext(deliverable, time.Unix(1, 0))

	if len(context.NetworkZones) != 1 || len(context.NetworkZones[0].Sessions) != 1 {
		t.Fatalf("zones = %#v", context.NetworkZones)
	}
	session := context.NetworkZones[0].Sessions[0]
	if session.Label != "核心交换机" || session.AccessPoint != "SW-01" || session.TesterIP != "10.0.0.10" || session.TargetsText != "\n\u3000\u3000\u3000\u300010.0.1.1\n\u3000\u3000\u3000\u300010.0.1.2\n\u3000\u3000\u3000\u300010.0.1.3" || session.ExclusionsText != "目标：10.0.1.99；端口：22" || session.Notes != "夜间窗口" {
		t.Fatalf("session = %#v", session)
	}
}
