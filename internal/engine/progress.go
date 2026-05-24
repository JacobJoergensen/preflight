package engine

type ScanProgress interface {
	Plan(total int)
	Start(scopeID, displayName string)
	Finish(scopeID string, included bool)
	Close()
}

type NoopScanProgress struct{}

func (NoopScanProgress) Plan(int)             {}
func (NoopScanProgress) Start(string, string) {}
func (NoopScanProgress) Finish(string, bool)  {}
func (NoopScanProgress) Close()               {}
