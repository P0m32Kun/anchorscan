package report

import (
	"fmt"
	"strings"
)

// ValidateProjectDeliverable enforces the accepted formal-report slot
// contract before either exporter writes a deliverable.
func ValidateProjectDeliverable(deliverable ProjectDeliverable) error {
	project := deliverable.Project
	for label, value := range map[string]string{
		"报告标题":   project.ReportTitle,
		"被测单位":   project.ClientUnit,
		"测试对象":   project.TestObject,
		"测试开始日期": project.StartDate,
		"测试结束日期": project.EndDate,
		"测试人员":   project.Testers,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("正式报告缺少%s", label)
		}
	}
	if project.CreatedAt.IsZero() {
		return fmt.Errorf("正式报告缺少项目创建时间")
	}
	if deliverable.Stats.Critical > 0 {
		return fmt.Errorf("正式报告暂不支持 critical 结论口径，请先调整严重级别")
	}
	for _, zone := range deliverable.Zones {
		for _, session := range zone.Sessions {
			if strings.TrimSpace(session.AccessPoint) == "" {
				return fmt.Errorf("纳入报告的运行“%s”缺少接入点", session.Label)
			}
			if strings.TrimSpace(session.TesterIP) == "" {
				return fmt.Errorf("纳入报告的运行“%s”缺少测试机 IP", session.Label)
			}
			if strings.TrimSpace(session.Targets) == "" {
				return fmt.Errorf("纳入报告的运行“%s”缺少测试范围", session.Label)
			}
		}
		for _, verification := range zone.Confirmed {
			if strings.EqualFold(strings.TrimSpace(verification.Severity), "critical") {
				return fmt.Errorf("正式报告暂不支持 critical 结论口径，请先调整严重级别")
			}
			if strings.TrimSpace(verification.Description) == "" {
				return fmt.Errorf("纳入报告的已确认验证项“%s”缺少漏洞描述", verification.Title)
			}
			if strings.TrimSpace(verification.Remediation) == "" {
				return fmt.Errorf("纳入报告的已确认验证项“%s”缺少修改建议", verification.Title)
			}
			if len(verification.Assets) == 0 {
				return fmt.Errorf("纳入报告的已确认验证项“%s”缺少关联资产", verification.Title)
			}
			if len(verification.Evidence) == 0 {
				return fmt.Errorf("纳入报告的已确认验证项“%s”缺少证据", verification.Title)
			}
		}
		for _, verification := range zone.NotObserved {
			if len(verification.Assets) == 0 {
				return fmt.Errorf("纳入报告的未发现验证项“%s”缺少端口资产", verification.Title)
			}
			if len(verification.Evidence) == 0 {
				return fmt.Errorf("纳入报告的未发现验证项“%s”缺少证据", verification.Title)
			}
		}
	}
	return nil
}
