package machine

import (
	"fmt"
	"plugin"
	"reflect"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

type goPluginProvider struct{}

type yaegiProvider struct{}

func (y *yaegiProvider) Load(pd *PluginDefinition) (interface{}, error) {
	i := interp.New(interp.Options{})
	i.Use(stdlib.Symbols)
	i.Use(symbols)

	if _, err := i.Eval(pd.Payload); err != nil {
		return nil, fmt.Errorf("error evaluating script %w", err)
	}

	sym, err := i.Eval(pd.Symbol)

	if err != nil {
		return nil, fmt.Errorf("error evaluating symbol %w", err)
	}

	if sym.Kind() != reflect.Func {
		return nil, fmt.Errorf("symbol is not of kind func")
	}

	return sym.Interface(), nil
}

func (g *goPluginProvider) Load(pd *PluginDefinition) (interface{}, error) {
	p, err := plugin.Open(pd.Payload)

	if err != nil {
		return nil, fmt.Errorf("error opening plugin %w", err)
	}

	sym, err := p.Lookup(pd.Symbol)

	if err != nil {
		return nil, fmt.Errorf("error looking up symbol %w", err)
	}

	return sym, nil
}

func init() {
	pluginProviders["plugin"] = &goPluginProvider{}
	pluginProviders["yaegi"] = &yaegiProvider{}
}
