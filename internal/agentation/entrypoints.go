package agentation

// EntryPoint describes one ARMOR web UI page that must prove Agentation
// mounted in a real browser. Keep this inventory explicit: the dashboard
// handler and the demo command share HTML, but they are separate user-facing
// ways to serve that page and both need coverage.
type EntryPoint struct {
	Name        string
	Kind        string
	Source      string
	Route       string
	TestPackage string
	MountTest   string
}

var entryPoints = []EntryPoint{
	{
		Name:        "proxy dashboard",
		Kind:        "dashboard",
		Source:      "internal/dashboard/dashboard.go",
		Route:       "/dashboard",
		TestPackage: "./internal/dashboard",
		MountTest:   "TestDashboardAgentationMountsInBrowser",
	},
	{
		Name:        "demo dashboard",
		Kind:        "demo",
		Source:      "cmd/armor/cmd_demo.go",
		Route:       "/dashboard",
		TestPackage: "./cmd/armor",
		MountTest:   "TestDemoAgentationMountsInBrowser",
	},
	{
		Name:        "fleet console",
		Kind:        "html",
		Source:      "cmd/armor-fleet/server.go",
		Route:       "/",
		TestPackage: "./cmd/armor-fleet",
		MountTest:   "TestFleetAgentationMountsInBrowser",
	},
}

// UIEntryPoints returns the complete UI inventory. A copy prevents tests and
// callers from changing the contract used by the gate.
func UIEntryPoints() []EntryPoint {
	return append([]EntryPoint(nil), entryPoints...)
}
