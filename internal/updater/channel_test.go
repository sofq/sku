package updater_test

import (
	"errors"
	"testing"

	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/updater"
)

func TestResolveChannel(t *testing.T) {
	tests := []struct {
		name        string
		flag        string
		env         string
		configValue string
		want        updater.Channel
		wantErr     bool
	}{
		// Defaults
		{name: "all_empty_defaults_to_stable", flag: "", env: "", configValue: "", want: updater.ChannelStable},

		// Flag wins over everything
		{name: "flag_stable_wins", flag: "stable", env: "daily", configValue: "daily", want: updater.ChannelStable},
		{name: "flag_daily_wins", flag: "daily", env: "stable", configValue: "stable", want: updater.ChannelDaily},

		// Env wins over config
		{name: "env_daily_beats_config", flag: "", env: "daily", configValue: "stable", want: updater.ChannelDaily},
		{name: "env_stable_beats_config", flag: "", env: "stable", configValue: "daily", want: updater.ChannelStable},

		// Config wins over default
		{name: "config_daily_beats_default", flag: "", env: "", configValue: "daily", want: updater.ChannelDaily},
		{name: "config_stable_explicit", flag: "", env: "", configValue: "stable", want: updater.ChannelStable},

		// Invalid values
		{name: "flag_invalid", flag: "weekly", env: "", configValue: "", wantErr: true},
		{name: "env_invalid", flag: "", env: "nightly", configValue: "", wantErr: true},
		{name: "config_invalid", flag: "", env: "", configValue: "beta", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := updater.ResolveChannel(tc.flag, tc.env, tc.configValue)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				var e *skuerrors.E
				if !errors.As(err, &e) {
					t.Fatalf("want *skuerrors.E, got %T: %v", err, err)
				}
				if e.Code != skuerrors.CodeValidation {
					t.Errorf("want CodeValidation, got %v", e.Code)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
