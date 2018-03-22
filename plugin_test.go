package main

import (
	"path/filepath"
	"testing"

	"github.com/zalando/skipper"
)

var pluginDir string = "./build"

func TestLoadPluginNoop(t *testing.T) {
	if _, err := skipper.LoadFilterPlugin(filepath.Join(pluginDir, "filter_noop.so"), []string{}); err != nil {
		t.Errorf("failed to load plugin `noop`: %s", err)
	}
}

func TestLoadPluginGeoIP(t *testing.T) {
	if _, err := skipper.LoadFilterPlugin(filepath.Join(pluginDir, "filter_geoip.so"), []string{"db=GeoLite2-Country.mmdb"}); err != nil {
		t.Errorf("failed to load plugin `geoip`: %s", err)
	}
}
