package websocket

import "testing"

func TestDefaultMessageFormatConsistency(t *testing.T) {
	clientCfg := DefaultConfig()
	serverCfg := DefaultServerConfig()

	if clientCfg.MessageFormat != MessageFormatJSON {
		t.Fatalf("expected client default message format %q, got %q", MessageFormatJSON, clientCfg.MessageFormat)
	}
	if serverCfg.MessageFormat != MessageFormatJSON {
		t.Fatalf("expected server default message format %q, got %q", MessageFormatJSON, serverCfg.MessageFormat)
	}
	if clientCfg.MessageFormat != serverCfg.MessageFormat {
		t.Fatalf("expected client/server default message format to match, got %q vs %q", clientCfg.MessageFormat, serverCfg.MessageFormat)
	}
}

func TestConfigFileValidate_MessageFormatEvenWhenNodeDisabled(t *testing.T) {
	cfg := DefaultConfigFile()
	cfg.Node.Enabled = false
	cfg.Client.MessageFormat = "invalid-format"

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validate to fail for invalid client message format")
	}

	cfg = DefaultConfigFile()
	cfg.Node.Enabled = false
	cfg.Server.MessageFormat = "invalid-format"

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validate to fail for invalid server message format")
	}
}

func TestConfigFileNormalize_MessageFormatDefault(t *testing.T) {
	cfg := ConfigFile{}
	cfg.Normalize()

	if cfg.Client.MessageFormat != MessageFormatJSON {
		t.Fatalf("expected normalized client message format %q, got %q", MessageFormatJSON, cfg.Client.MessageFormat)
	}
	if cfg.Server.MessageFormat != MessageFormatJSON {
		t.Fatalf("expected normalized server message format %q, got %q", MessageFormatJSON, cfg.Server.MessageFormat)
	}
}
