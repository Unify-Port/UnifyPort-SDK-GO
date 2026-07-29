package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

const (
	allowedOverlayTarget = "$.components.responses.RuntimeActionResult"
	allowedOverlayKey    = "x-go-name"
	allowedOverlayValue  = "RuntimeActionResponse"
)

type namingOverlay struct {
	Overlay string          `yaml:"overlay"`
	Info    overlayInfo     `yaml:"info"`
	Actions []overlayAction `yaml:"actions"`
}

type overlayInfo struct {
	Title   string `yaml:"title"`
	Version string `yaml:"version"`
}

type overlayAction struct {
	Target      string         `yaml:"target"`
	Description string         `yaml:"description"`
	Update      map[string]any `yaml:"update"`
}

func validateNamingOverlay(root string) error {
	path := filepath.Join(root, filepath.FromSlash(overlayRelativePath))
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", overlayRelativePath, err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	var overlay namingOverlay
	if err := decoder.Decode(&overlay); err != nil {
		return fmt.Errorf("decode %s: %w", overlayRelativePath, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%s must contain exactly one YAML document", overlayRelativePath)
	}
	if overlay.Overlay != "1.0.0" || overlay.Info.Title == "" || overlay.Info.Version == "" || len(overlay.Actions) != 1 {
		return fmt.Errorf("%s must contain exactly one approved Overlay 1.0.0 action", overlayRelativePath)
	}
	for index, action := range overlay.Actions {
		if action.Description == "" {
			return fmt.Errorf("%s action %d is incomplete", overlayRelativePath, index)
		}
		// Overlay extension 也可改变公开类型语义，因此只允许当前已审批的单一命名冲突规则。
		if action.Target != allowedOverlayTarget || len(action.Update) != 1 || action.Update[allowedOverlayKey] != allowedOverlayValue {
			return fmt.Errorf("%s action %d is not the approved Go naming workaround", overlayRelativePath, index)
		}
	}
	return nil
}
