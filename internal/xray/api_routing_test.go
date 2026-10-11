package xray

import "testing"

// xray-core 26.10.10 resolves routing.script from the loader's working directory, and the
// running core keeps its script anyway: a script beside the xray binary failed every hot
// rule change panel-side, turning it into a full restart and breaking the node bridge.
func TestBuildReloadableRouting_IgnoresLuaScript(t *testing.T) {
	t.Chdir(t.TempDir())
	routing := []byte(`{"script":"route.lua","rules":[{"type":"field","ip":["10.0.0.0/8"],"outboundTag":"direct"}]}`)

	config, err := buildReloadableRouting(routing)
	if err != nil {
		t.Fatalf("buildReloadableRouting: %v", err)
	}
	if config.Script != "" {
		t.Fatalf("Script = %q, want it left to the running core", config.Script)
	}
	if len(config.Rule) != 1 || config.Rule[0].GetTag() != "direct" {
		t.Fatalf("rules = %v, want the one rule to direct", config.Rule)
	}
}
