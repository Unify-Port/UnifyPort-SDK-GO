package unifyport

import (
	"errors"
	"testing"
	"time"
)

func TestResolveCallOptions(t *testing.T) {
	t.Parallel()
	settings, err := resolveCallOptions([]CallOption{
		WithTimeout(2 * time.Second),
		WithMaxRetries(1),
	})
	if err != nil {
		t.Fatalf("resolve options: %v", err)
	}
	if settings.Timeout != 2*time.Second || settings.MaxRetries == nil || *settings.MaxRetries != 1 || settings.DisableRetry {
		t.Fatalf("unexpected settings: %#v", settings)
	}

	disabled, err := resolveCallOptions([]CallOption{WithMaxRetries(1), WithoutRetry()})
	if err != nil {
		t.Fatalf("resolve disabled retry: %v", err)
	}
	if !disabled.DisableRetry || disabled.MaxRetries != nil {
		t.Fatalf("retry was not disabled: %#v", disabled)
	}
}

func TestResolveCallOptionsRejectsInvalidValues(t *testing.T) {
	t.Parallel()
	for _, options := range [][]CallOption{
		{WithTimeout(0)},
		{WithMaxRetries(-1)},
		{nil},
	} {
		_, err := resolveCallOptions(options)
		var configurationError *ConfigurationError
		if !errors.As(err, &configurationError) {
			t.Fatalf("expected ConfigurationError, got %v", err)
		}
	}
}
