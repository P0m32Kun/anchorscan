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

// ReadJSON reads a previously written JSON report. It ignores unknown fields so
// reports produced by older/newer versions can still be inspected.
func ReadJSON(path string) (ScanReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ScanReport{}, err
	}
	var r ScanReport
	if err := json.Unmarshal(data, &r); err != nil {
		return ScanReport{}, err
	}
	return r, nil
}
