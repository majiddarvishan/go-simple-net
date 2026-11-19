package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

type peerConfig struct {
	ID           string `json:"id"`
	BindSystemID string `json:"bind_system_id"`
}

type demoConfig struct {
	ListenAddress      string            `json:"listen_address"`
	ProtocolVersion    string            `json:"protocol_version"`
	PreBindTimeout     string            `json:"pre_bind_handshake_timeout"`
	FrameTimeout       string            `json:"frame_assembly_timeout"`
	WriteTimeout       string            `json:"write_timeout"`
	RequestTimeout     string            `json:"request_timeout"`
	MaxBoundSessions   uint32            `json:"max_bound_sessions"`
	MaxPendingPreBind  uint32            `json:"max_pending_pre_bind_sessions"`
	ApplicationWorkers uint32            `json:"application_workers"`
	CompletionWorkers  uint32            `json:"completion_workers"`
	OnCloseWorkers     uint32            `json:"on_close_workers"`
	OnCloseCredits     uint32            `json:"on_close_event_credits"`
	QueueBytes         uint64            `json:"aggregate_queued_application_bytes"`
	QueueFrames        uint32            `json:"aggregate_queued_application_frames"`
	AppConcurrency     uint32            `json:"application_callback_concurrency"`
	MaxIncomingFrames  uint32            `json:"max_incoming_application_frames"`
	MaxInflight        uint32            `json:"max_inflight_dispatches"`
	Peers              []peerConfig      `json:"peers"`
	Routes             map[string]string `json:"routes"`
}

func defaults() demoConfig {
	return demoConfig{
		ListenAddress:      "127.0.0.1:19090",
		ProtocolVersion:    "v2",
		PreBindTimeout:     "10s",
		FrameTimeout:       "30s",
		WriteTimeout:       "10s",
		RequestTimeout:     "3s",
		MaxBoundSessions:   8,
		MaxPendingPreBind:  8,
		ApplicationWorkers: 4,
		CompletionWorkers:  4,
		OnCloseWorkers:     2,
		OnCloseCredits:     16,
		QueueBytes:         1 << 20,
		QueueFrames:        128,
		AppConcurrency:     2,
		MaxIncomingFrames:  64,
		MaxInflight:        16,
		Peers: []peerConfig{
			{ID: "peer-east-01", BindSystemID: "peer-east"},
			{ID: "peer-west-01", BindSystemID: "peer-west"},
		},
		Routes: map[string]string{"east": "peer-east-01", "west": "peer-west-01"},
	}
}

func loadJSON(path string, cfg *demoConfig) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open config: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() > 1<<20 {
		return fmt.Errorf("config exceeds 1 MiB demo limit")
	}
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(cfg); err != nil {
		return fmt.Errorf("decode config: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("config must contain exactly one JSON object")
	}
	return nil
}

// CLI explicitly set flags override JSON, which overrides library-style defaults.
func parseConfig(args []string) (demoConfig, error) {
	cfg := defaults()
	fs := flag.NewFlagSet("server-peer-router", flag.ContinueOnError)
	configPath := fs.String("config", "", "optional strict JSON configuration")
	listen := fs.String("listen", "", "internal neta TCP listen address")
	maxBound := fs.Uint("max-bound", 0, "max simultaneous peer Sessions")
	appWorkers := fs.Uint("app-workers", 0, "owner-wide Application workers")
	appConcurrency := fs.Uint("app-concurrency", 0, "per-Session Application callbacks")
	queueFrames := fs.Uint("queue-frames", 0, "aggregate queued Application frames")
	queueBytes := fs.Uint64("queue-bytes", 0, "aggregate queued Application bytes")
	inflight := fs.Uint("max-inflight", 0, "max simultaneous outbound route Requests")
	requestTimeout := fs.String("request-timeout", "", "outbound Request deadline, e.g. 3s")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if fs.NArg() != 0 {
		return cfg, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	if *configPath != "" {
		if err := loadJSON(*configPath, &cfg); err != nil {
			return cfg, err
		}
	}
	var overrideErr error
	setUint32 := func(name string, v uint) uint32 {
		if uint64(v) > uint64(^uint32(0)) {
			overrideErr = fmt.Errorf("flag %s overflows uint32", name)
			return 0
		}
		return uint32(v)
	}
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "listen":
			cfg.ListenAddress = *listen
		case "max-bound":
			cfg.MaxBoundSessions = setUint32(f.Name, *maxBound)
		case "app-workers":
			cfg.ApplicationWorkers = setUint32(f.Name, *appWorkers)
		case "app-concurrency":
			cfg.AppConcurrency = setUint32(f.Name, *appConcurrency)
		case "queue-frames":
			cfg.QueueFrames = setUint32(f.Name, *queueFrames)
		case "queue-bytes":
			cfg.QueueBytes = *queueBytes
		case "max-inflight":
			cfg.MaxInflight = setUint32(f.Name, *inflight)
		case "request-timeout":
			cfg.RequestTimeout = *requestTimeout
		}
	})
	if overrideErr != nil {
		return cfg, overrideErr
	}
	return cfg, validateConfig(cfg)
}

func validLabel(s string, max int) bool {
	if len(s) == 0 || len(s) > max {
		return false
	}
	for _, ch := range s {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '.' || ch == '_' || ch == '-') {
			return false
		}
	}
	return true
}

func validateConfig(cfg demoConfig) error {
	if cfg.ListenAddress == "" {
		return fmt.Errorf("listen_address is required")
	}
	if cfg.ProtocolVersion != "v1" && cfg.ProtocolVersion != "v2" {
		return fmt.Errorf("protocol_version must be v1 or v2")
	}
	for _, d := range []struct{ name, value string }{
		{"pre_bind_handshake_timeout", cfg.PreBindTimeout},
		{"frame_assembly_timeout", cfg.FrameTimeout},
		{"write_timeout", cfg.WriteTimeout},
		{"request_timeout", cfg.RequestTimeout},
	} {
		parsed, err := time.ParseDuration(d.value)
		if err != nil || parsed <= 0 {
			return fmt.Errorf("%s must be a positive Go duration", d.name)
		}
	}
	if cfg.MaxBoundSessions == 0 || cfg.MaxPendingPreBind == 0 ||
		cfg.ApplicationWorkers == 0 || cfg.ApplicationWorkers > 512 ||
		cfg.CompletionWorkers == 0 || cfg.CompletionWorkers > 256 ||
		cfg.OnCloseWorkers == 0 || cfg.OnCloseWorkers > 128 ||
		cfg.OnCloseCredits == 0 || cfg.OnCloseCredits > 4096 ||
		cfg.QueueBytes == 0 || cfg.QueueFrames == 0 || cfg.QueueFrames > 262144 ||
		cfg.AppConcurrency == 0 || cfg.AppConcurrency > 32 ||
		cfg.MaxIncomingFrames == 0 || cfg.MaxIncomingFrames > 1024 ||
		cfg.MaxInflight == 0 || cfg.MaxInflight > 65536 {
		return fmt.Errorf("capacity or worker setting outside documented bounds")
	}
	if len(cfg.Peers) == 0 || len(cfg.Peers) > 512 ||
		uint32(len(cfg.Peers)) > cfg.MaxBoundSessions {
		return fmt.Errorf("peers must fit max_bound_sessions (1..512 configured)")
	}
	knownIDs := make(map[string]bool)
	knownBinds := make(map[string]bool)
	maxBindID := 20
	if cfg.ProtocolVersion == "v1" {
		maxBindID = 19
	}
	for _, peer := range cfg.Peers {
		if !validLabel(peer.ID, 64) || !validLabel(peer.BindSystemID, maxBindID) {
			return fmt.Errorf("peer identifiers must be ASCII labels; bind ID max %d bytes", maxBindID)
		}
		if knownIDs[peer.ID] || knownBinds[peer.BindSystemID] {
			return fmt.Errorf("duplicate peer ID or Bind SystemID in configuration")
		}
		knownIDs[peer.ID] = true
		knownBinds[peer.BindSystemID] = true
	}
	if len(cfg.Routes) == 0 || len(cfg.Routes) > 1024 {
		return fmt.Errorf("routes must contain 1..1024 exact keys")
	}
	for key, id := range cfg.Routes {
		if !validLabel(key, 64) || !knownIDs[id] {
			return fmt.Errorf("route %q references invalid or unknown peer %q", key, id)
		}
	}
	return nil
}
