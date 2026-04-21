package updater

import (
	"fmt"

	skuerrors "github.com/sofq/sku/internal/errors"
)

// Channel controls whether sku update uses the delta chain or always
// re-downloads the full baseline.
type Channel string

const (
	// ChannelStable always downloads the latest baseline shard. Safe for
	// automated pipelines that need a known-good, complete snapshot.
	ChannelStable Channel = "stable"

	// ChannelDaily walks the manifest delta chain first and falls back to the
	// baseline only when the chain is too long, starts elsewhere, or the shard
	// file is missing. Best for daily/frequent operator updates.
	ChannelDaily Channel = "daily"
)

var validChannels = []Channel{ChannelStable, ChannelDaily}

func isValidChannel(v string) bool {
	for _, c := range validChannels {
		if string(c) == v {
			return true
		}
	}
	return false
}

// ResolveChannel returns the effective Channel from the precedence chain:
// flag > env (SKU_UPDATE_CHANNEL) > configValue > ChannelStable.
// Any non-empty value that is not "stable" or "daily" produces a
// skuerrors.CodeValidation error with reason "flag_invalid".
func ResolveChannel(flag, env, configValue string) (Channel, error) {
	for _, src := range []struct{ name, value string }{
		{"flag", flag},
		{"env", env},
		{"config", configValue},
	} {
		if src.value == "" {
			continue
		}
		if !isValidChannel(src.value) {
			return "", skuerrors.Validation(
				"flag_invalid",
				"channel",
				src.value,
				fmt.Sprintf("valid values: stable, daily (got %q from %s)", src.value, src.name),
			)
		}
		return Channel(src.value), nil
	}
	return ChannelStable, nil
}
