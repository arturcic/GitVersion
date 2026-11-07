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
	var data []byte
	file, err := p.Locator.Get(directory)
	if err != nil {
		return ResolvedConfiguration{}, err
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
			p.Locator.log(fmt.Sprintf("Configuration file '%s' uses the temporary v6 compatibility mode. Legacy configuration loading is removed in GitVersion 7.1. Run 'gitversion config migrate' and validate with GITVERSION_CONFIGURATION_VERSION=v7.", *file))
		})
	}
	return result, err
}
