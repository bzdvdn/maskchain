package bootstrap

import (
	"log/slog"
	"os"

	"github.com/bzdvdn/maskchain/src/internal/domain/compliance"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/detector"
	"github.com/bzdvdn/maskchain/src/internal/domain/shield/entity"
	"github.com/bzdvdn/maskchain/src/internal/infra/config"
)

// LoadComplianceRegistry builds the compliance pack registry from the preset
// directory configured in `compliance.preset_dir`. Returns nil when compliance
// is not configured or the preset dir cannot be read (logged as an error).
func LoadComplianceRegistry(cfg *config.Config, log *slog.Logger) *compliance.Registry {
	if cfg == nil || cfg.Compliance == nil || cfg.Compliance.PresetDir == "" {
		return nil
	}
	dir := cfg.Compliance.PresetDir
	if _, err := os.Stat(dir); err != nil {
		log.Error("compliance preset dir unavailable — compliance disabled",
			slog.String("dir", dir), slog.String("error", err.Error()))
		return nil
	}

	catalog := compliance.NewCatalogFromRegistry(complianceDetectorRegistry())
	reg, errs := compliance.LoadPacksFromDir(dir, catalog)
	for _, e := range errs {
		log.Warn("compliance preset skipped", slog.String("error", e.Error()))
	}
	if len(reg.Packs()) == 0 {
		log.Warn("no compliance packs loaded", slog.String("dir", dir))
		return nil
	}
	return reg
}

// complianceDetectorRegistry is the reference detector set for preset
// validation. It mirrors the runtime detector types registered by the gateway
// so presets cannot reference IDs the engine does not serve.
func complianceDetectorRegistry() *detector.DetectorRegistry {
	reg := detector.NewDetectorRegistry()
	_ = reg.Register(entity.DetectorTypeRegex, detector.NewCompositeDetector())
	_ = reg.Register(entity.DetectorTypeDictionary, detector.NewDictionaryDetector(nil))
	_ = reg.Register(entity.DetectorTypePromptInjection, detector.NewPromptInjectionDetector())
	return reg
}
