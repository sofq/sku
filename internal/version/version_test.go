package version

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInfo_DefaultValuesAreDev(t *testing.T) {
	info := Get()
	require.Equal(t, "dev", info.Version)
	require.Equal(t, "unknown", info.Commit)
	require.Equal(t, "unknown", info.Date)
}

func TestInfo_JSONShape(t *testing.T) {
	info := Info{Version: "1.2.3", Commit: "abc123", Date: "2026-04-18T00:00:00Z", GoVersion: "go1.25.0", Os: "linux", Arch: "amd64"}
	raw, err := json.Marshal(info)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, "1.2.3", got["version"])
	require.Equal(t, "abc123", got["commit"])
	require.Equal(t, "2026-04-18T00:00:00Z", got["date"])
	require.Equal(t, "go1.25.0", got["go_version"])
	require.Equal(t, "linux", got["os"])
	require.Equal(t, "amd64", got["arch"])
}

func TestInfo_OverrideViaLdflags(t *testing.T) {
	oldV, oldC, oldD := version, commit, date
	t.Cleanup(func() { version, commit, date = oldV, oldC, oldD })
	version, commit, date = "1.0.0", "deadbeef", "2026-04-18T12:00:00Z"

	info := Get()
	require.Equal(t, "1.0.0", info.Version)
	require.Equal(t, "deadbeef", info.Commit)
	require.Equal(t, "2026-04-18T12:00:00Z", info.Date)
}
