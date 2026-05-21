package model

// Severity classifies the importance of a check Message.
type Severity string

const (
	SeveritySuccess Severity = "success"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)
