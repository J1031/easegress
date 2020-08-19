package requestadaptor

import (
	"regexp"
	"strings"

	"github.com/megaease/easegateway/pkg/logger"
)

type (
	// PathAdaptorSpec describes rules for adapting path.
	PathAdaptorSpec struct {
		Replace       string         `yaml:"replace,omitempty" jsonschema:"omitempty"`
		AddPrefix     string         `yaml:"addPrefix,omitempty" jsonschema:"omitempty,pattern=^/"`
		TrimPrefix    string         `yaml:"trimPrefix,omitempty" jsonschema:"omitempty,pattern=^/"`
		RegexpReplace *RegexpReplace `yaml:"regexpReplace,omitempty" jsonschema:"omitempty"`
	}

	// RegexpReplace use regexp-replace pair to rewrite path.
	RegexpReplace struct {
		Regexp  string `yaml:"regexp" jsonschema:"required,format=regexp"`
		Replace string `yaml:"replace"`

		re *regexp.Regexp
	}

	// PathAdaptor is the adaptor to adapt the path.
	PathAdaptor struct {
		spec *PathAdaptorSpec
	}
)

// newPathAdaptor creates a pathAdaptor.
func newPathAdaptor(spec *PathAdaptorSpec) *PathAdaptor {
	if spec.RegexpReplace != nil {
		var err error
		spec.RegexpReplace.re, err = regexp.Compile(spec.RegexpReplace.Regexp)
		if err != nil {
			logger.Errorf("BUG: compile regexp %s failed: %v",
				spec.RegexpReplace.Regexp, err)
		}
	}

	return &PathAdaptor{
		spec: spec,
	}
}

// Adapt adapts path.
func (a *PathAdaptor) Adapt(path string) string {
	if len(a.spec.Replace) != 0 {
		return a.spec.Replace
	}

	if len(a.spec.AddPrefix) != 0 {
		return a.spec.AddPrefix + path
	}

	if len(a.spec.TrimPrefix) != 0 {
		return strings.TrimPrefix(path, a.spec.TrimPrefix)
	}

	if a.spec.RegexpReplace != nil && a.spec.RegexpReplace.re != nil {
		return a.spec.RegexpReplace.re.ReplaceAllString(path,
			a.spec.RegexpReplace.Replace)
	}

	return path
}
