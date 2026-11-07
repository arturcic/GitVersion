package fixtures

// ConfigurationPort preserves expected public settings from a named C# test.
// Paths use the pinned v7 effective configuration document.
type ConfigurationPort struct {
	ID       string         `json:"id"`
	Config   string         `json:"config"`
	Expected map[string]any `json:"expected"`
}
