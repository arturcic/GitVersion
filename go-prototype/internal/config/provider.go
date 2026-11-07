package config

import (
	"fmt"
	"sync"
)

// Provider resolves a configuration file and optional runtime overrides.
type Provider struct {
	Locator       FileLocator
	Version       string
	legacyWarning sync.Once
}

func (p *Provider) Provide(directory *string, overrides []byte) (ResolvedConfiguration, error) {
	return p.ProvideFrom(directory, nil, overrides)
}

func (p *Provider) ProvideFrom(directory, root *string, overrides []byte) (ResolvedConfiguration, error) {
	if directory != nil && root != nil {
		if err := p.Locator.Verify(*directory, *root); err != nil {
			return ResolvedConfiguration{}, err
		}
	}
	var data []byte
	file, err := p.Locator.Get(directory)
	if err == nil && file == nil && root != nil && pathValue(directory) != *root {
		file, err = p.Locator.Get(root)
	}
	if err != nil {
		return ResolvedConfiguration{}, err
	}
	if p.Locator.Log != nil {
		selected, err := SelectConfigurationVersion(&p.Version)
		if err != nil {
			return ResolvedConfiguration{}, err
		}
		p.Locator.log("Configuration version: " + selected)
		p.Locator.log(configurationSource(file))
	}
	if file != nil {
		data, err = p.Locator.fs().ReadFile(*file)
		if err != nil {
			return ResolvedConfiguration{}, err
		}
	}
	result, err := ResolveConfiguration(data, overrides, p.Version)
	if err == nil && file != nil && IsExplicitV6(&p.Version) {
		p.legacyWarning.Do(func() {
			p.Locator.log(LegacyConfigurationWarning(*file))
		})
	}
	return result, err
}

func LegacyConfigurationWarning(file string) string {
	return fmt.Sprintf("Configuration file '%s' uses the temporary v6 compatibility mode. Legacy configuration loading is removed in GitVersion 7.1. Run 'gitversion config migrate' and validate with GITVERSION_CONFIGURATION_VERSION=v7.", file)
}

func configurationSource(file *string) string {
	if file == nil {
		return "No configuration file found, using default configuration"
	}
	return fmt.Sprintf("Using configuration file '%s'", *file)
}
