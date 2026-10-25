package cli

import (
	"fmt"

	"github.com/jmcampanini/gsd/internal/render"
)

type colorValue struct {
	mode *render.ColorMode
}

func (v colorValue) Set(value string) error {
	mode := render.ColorMode(value)
	switch mode {
	case render.ColorAuto, render.ColorAlways, render.ColorNever:
		*v.mode = mode
		return nil
	default:
		return fmt.Errorf("invalid color mode %q: must be auto, always, or never", value)
	}
}

func (v colorValue) String() string {
	if v.mode == nil || *v.mode == "" {
		return string(render.ColorAuto)
	}
	return string(*v.mode)
}

func (colorValue) Type() string {
	return "color"
}
