package conf

func init() {
	RegisterConfigureFilePostProcessingStage("FakeDNS", &FakeDNSPostProcessingStage{})
}
