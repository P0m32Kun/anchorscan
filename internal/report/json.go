package report

import (
	"encoding/json"
	"os"
)

func WriteJSON(path string, scanReport ScanReport) error {
	data, err := json.MarshalIndent(scanReport, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
