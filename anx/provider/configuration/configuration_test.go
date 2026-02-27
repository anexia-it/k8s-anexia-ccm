package configuration

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// errReader implements io.Reader that always returns an error.
type errReader struct{}

func (errReader) Read(_ []byte) (int, error) {
    return 0, errors.New("read failed")
}

func TestNewProviderConfig_nil(t *testing.T) {
    // make sure env is clean
    os.Unsetenv("ANEXIA_TOKEN")
    os.Unsetenv("ANEXIA_CLUSTER_NAME")

    cfg, err := NewProviderConfig(nil)
    if err != nil {
        t.Fatal(err)
    }
    if cfg.Token != "" {
        t.Errorf("expected empty token, got %q", cfg.Token)
    }
    if cfg.ClusterName != "" {
        t.Errorf("expected empty cluster name, got %q", cfg.ClusterName)
    }
}

func TestNewProviderConfig_yaml(t *testing.T) {
    // simple YAML to verify unmarshalling
    data := "anexiaToken: foo\nclusterName: mycluster\n"
    r := strings.NewReader(data)
    cfg, err := NewProviderConfig(r)
    if err != nil {
        t.Fatal(err)
    }
    if cfg.Token != "foo" {
        t.Errorf("token mismatch: %q", cfg.Token)
    }
    if cfg.ClusterName != "mycluster" {
        t.Errorf("cluster name mismatch: %q", cfg.ClusterName)
    }
}

func TestNewProviderConfig_env_overrides(t *testing.T) {
    os.Setenv("ANEXIA_TOKEN", "bar")
    os.Setenv("ANEXIA_CLUSTER_NAME", "envcluster")
    defer os.Unsetenv("ANEXIA_TOKEN")
    defer os.Unsetenv("ANEXIA_CLUSTER_NAME")

    cfg, err := NewProviderConfig(strings.NewReader(""))
    if err != nil {
        t.Fatal(err)
    }
    if cfg.Token != "bar" {
        t.Errorf("expected token from env, got %q", cfg.Token)
    }
    if cfg.ClusterName != "envcluster" {
        t.Errorf("expected cluster name from env, got %q", cfg.ClusterName)
    }
}

func TestNewProviderConfig_readError(t *testing.T) {
    _, err := NewProviderConfig(errReader{})
    if err == nil {
        t.Fatal("expected error when reader fails")
    }
}

func TestNewProviderConfig_yamlError(t *testing.T) {
    r := strings.NewReader("not: valid: yaml: [")
    _, err := NewProviderConfig(r)
    if err == nil {
        t.Fatal("expected yaml unmarshal error")
    }
}

func TestGetManagerOptions_cached(t *testing.T) {
    // reset global so we can test caching
    managerOptions = nil
    o1, err := GetManagerOptions()
    if err != nil {
        t.Fatal(err)
    }
    o2, err := GetManagerOptions()
    if err != nil {
        t.Fatal(err)
    }
    if o1 != o2 {
        t.Errorf("expected same managerOptions instance")
    }
}

func TestApplyCliFlagsToProviderConfig(t *testing.T) {
    // obtain a real managerOptions instance so that inner pointers are valid
    mo, err := GetManagerOptions()
    if err != nil {
        t.Fatal(err)
    }
    mo.KubeCloudShared.ClusterName = "kubernetes"
    // also update global variable to keep applyCliFlagsFromProviderConfig happy
    managerOptions = mo
    cfg := &ProviderConfig{}
    if err := applyCliFlagsToProviderConfig(cfg); err != nil {
        t.Fatal(err)
    }
    if cfg.ClusterName != "" {
        t.Errorf("expected no cluster name when shared name is kubernetes, got %q", cfg.ClusterName)
    }
    managerOptions.KubeCloudShared.ClusterName = "foo"
    if err := applyCliFlagsToProviderConfig(cfg); err != nil {
        t.Fatal(err)
    }
    if cfg.ClusterName != "foo" {
        t.Errorf("expected cluster name to be set, got %q", cfg.ClusterName)
    }
}

func TestConstants(t *testing.T) {
    if CloudProviderScheme != "anexia://" {
        t.Errorf("unexpected scheme %q", CloudProviderScheme)
    }
}
