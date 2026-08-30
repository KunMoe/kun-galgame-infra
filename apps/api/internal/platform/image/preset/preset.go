package preset

import (
	"fmt"
	"os"
	"slices"

	"gopkg.in/yaml.v3"
)

type FitMode string

const (
	FitInside FitMode = "inside"
	FitCover  FitMode = "cover"
)

type MainPipelineConfig struct {
	FitWidth  int    `yaml:"fit_width"`
	FitHeight int    `yaml:"fit_height"`
	Format    string `yaml:"format"`
	Quality   int    `yaml:"quality"`
	StripEXIF bool   `yaml:"strip_exif"`
}

type VariantSpec struct {
	Name    string  `yaml:"name"`
	Width   int     `yaml:"width"`
	Height  int     `yaml:"height"`
	Fit     FitMode `yaml:"fit"`
	Format  string  `yaml:"format"`
	Quality int     `yaml:"quality"`
}

type Preset struct {
	Variants    []VariantSpec `yaml:"variants"`
	AllowedMIME []string      `yaml:"allowed_mime"`
}

func (p *Preset) IsMIMEAllowed(mime string) bool {
	return slices.Contains(p.AllowedMIME, mime)
}

type Config struct {
	MainPipeline MainPipelineConfig `yaml:"main_pipeline"`
	Presets      map[string]Preset  `yaml:"presets"`
}

func (c *Config) Get(name string) (Preset, bool) {
	p, ok := c.Presets[name]
	return p, ok
}

// Load reads configs/image_presets.yaml. New presets are YAML-only plus an
// oauth_clients.image_allowed_presets grant; this package does not name them.
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("preset: read %s: %w", path, err)
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("preset: parse %s: %w", path, err)
	}
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("preset: %w", err)
	}
	return &c, nil
}

func (c *Config) validate() error {
	if c.MainPipeline.FitWidth <= 0 || c.MainPipeline.FitHeight <= 0 {
		return fmt.Errorf("main_pipeline.fit_width and fit_height must be > 0")
	}
	if c.MainPipeline.Quality < 1 || c.MainPipeline.Quality > 100 {
		return fmt.Errorf("main_pipeline.quality must be 1..100")
	}
	if c.MainPipeline.Format == "" {
		c.MainPipeline.Format = "webp"
	}
	if len(c.Presets) == 0 {
		return fmt.Errorf("no presets defined")
	}
	for name, p := range c.Presets {
		for i, v := range p.Variants {
			if v.Name == "" {
				return fmt.Errorf("preset %q variant[%d] missing name", name, i)
			}
			if v.Width <= 0 || v.Height <= 0 {
				return fmt.Errorf("preset %q variant %q invalid size %dx%d", name, v.Name, v.Width, v.Height)
			}
			if v.Fit == "" {
				p.Variants[i].Fit = FitCover
			}
			if v.Format == "" {
				p.Variants[i].Format = "webp"
			}
			if v.Quality == 0 {
				p.Variants[i].Quality = 77
			}
		}
	}
	return nil
}
