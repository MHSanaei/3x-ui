package conf

import "github.com/xtls/xray-core/common/errors"

type ConfigureFilePostProcessingStage interface {
	Process(conf *Config) error
}

var configureFilePostProcessingStages map[string]ConfigureFilePostProcessingStage

func RegisterConfigureFilePostProcessingStage(name string, stage ConfigureFilePostProcessingStage) {
	if configureFilePostProcessingStages == nil {
		configureFilePostProcessingStages = make(map[string]ConfigureFilePostProcessingStage)
	}
	configureFilePostProcessingStages[name] = stage
}

func PostProcessConfigureFile(conf *Config) error {
	for k, v := range configureFilePostProcessingStages {
		if err := v.Process(conf); err != nil {
			return errors.New("Rejected by Postprocessing Stage ", k).AtError().Base(err)
		}
	}
	return nil
}
