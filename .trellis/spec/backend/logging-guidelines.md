# Logging Guidelines

Use contextual `log.Printf` at operational boundaries, naming the operation and identifier without secrets or raw evidence. Existing examples include `internal/app/project_deliverable.go` and `internal/web/docx_export.go`. Return errors for callers to classify; logs do not replace error handling.
