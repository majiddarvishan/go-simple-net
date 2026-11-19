// Command client-config demonstrates strict optional JSON + explicit CLI
// overrides mapped to the public neta Client/Session Functional Options.
// Successful SendRequest means local admission, not peer delivery.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	neta "github.com/majiddarvishan/neta-go"
)

type clientConfig struct {
	Address             *string `json:"address"`
	SystemID            *string `json:"system_id"`
	BindType            *string `json:"bind_type"`
	ProtocolVersion     *string `json:"protocol_version"`
	ConnectTimeout      *string `json:"connect_timeout"`
	BindResponseTimeout *string `json:"bind_response_timeout"`
	RequestTimeout      *string `json:"request_timeout"`
	WriteTimeout        *string `json:"write_timeout"`
	EnquireIdle         *string `json:"enquire_link_idle_interval"`
	AutoReconnect       *bool   `json:"auto_reconnect"`
	MaxOutstanding      *uint32 `json:"max_outstanding_requests_per_session"`
	ApplicationWorkers  *uint32 `json:"application_workers"`
	MaxRetainedAppBytes *uint64 `json:"max_retained_application_bytes"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func readClientJSON(path string) (clientConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return clientConfig{}, fmt.Errorf("open -config: %w", err)
	}
	defer f.Close()
	var config clientConfig
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return clientConfig{}, fmt.Errorf("decode -config: %w", err)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return clientConfig{}, fmt.Errorf("-config must contain exactly one JSON object")
		}
		return clientConfig{}, fmt.Errorf("trailing -config data: %w", err)
	}
	return config, nil
}

func parseClientArgs(args []string) (clientConfig, error) {
	fs := flag.NewFlagSet("client-config", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configFile := fs.String("config", "", "optional strict JSON config file")
	address := fs.String("addr", "", "TCP server host:port")
	systemID := fs.String("system-id", "", "Bind identity (V1 max 19 bytes, V2 max 20)")
	bindType := fs.String("bind-type", "", "bi or uni")
	version := fs.String("protocol", "", "v1 or v2 (default v2)")
	connectTimeout := fs.String("connect-timeout", "", "e.g. 5s")
	bindTimeout := fs.String("bind-response-timeout", "", "e.g. 5s")
	requestTimeout := fs.String("request-timeout", "", "e.g. 15s")
	writeTimeout := fs.String("write-timeout", "", "e.g. 10s")
	enquireIdle := fs.String("enquire-idle", "", "e.g. 30s or 0s to disable")
	autoReconnect := fs.Bool("auto-reconnect", false, "retry transport after retryable loss, never replay Requests")
	maxOutstanding := fs.Uint("max-outstanding", 0, "per-Session Request credits (1..512)")
	appWorkers := fs.Uint("app-workers", 0, "Client-wide Application workers (1..512)")
	retainedBytes := fs.Uint64("max-retained-app-bytes", 0, "Client-wide Application retention budget in bytes")
	if err := fs.Parse(args); err != nil {
		return clientConfig{}, err
	}
	if fs.NArg() != 0 {
		return clientConfig{}, fmt.Errorf("unexpected positional arguments: %v", fs.Args())
	}
	var config clientConfig
	var err error
	if *configFile != "" {
		config, err = readClientJSON(*configFile)
		if err != nil {
			return clientConfig{}, err
		}
	}
	var overflow error
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "addr":
			config.Address = address
		case "system-id":
			config.SystemID = systemID
		case "bind-type":
			config.BindType = bindType
		case "protocol":
			config.ProtocolVersion = version
		case "connect-timeout":
			config.ConnectTimeout = connectTimeout
		case "bind-response-timeout":
			config.BindResponseTimeout = bindTimeout
		case "request-timeout":
			config.RequestTimeout = requestTimeout
		case "write-timeout":
			config.WriteTimeout = writeTimeout
		case "enquire-idle":
			config.EnquireIdle = enquireIdle
		case "auto-reconnect":
			config.AutoReconnect = autoReconnect
		case "max-outstanding":
			if *maxOutstanding > uint(^uint32(0)) {
				overflow = fmt.Errorf("-max-outstanding overflows uint32")
			} else {
				v := uint32(*maxOutstanding)
				config.MaxOutstanding = &v
			}
		case "app-workers":
			if *appWorkers > uint(^uint32(0)) {
				overflow = fmt.Errorf("-app-workers overflows uint32")
			} else {
				v := uint32(*appWorkers)
				config.ApplicationWorkers = &v
			}
		case "max-retained-app-bytes":
			config.MaxRetainedAppBytes = retainedBytes
		}
	})
	if overflow != nil {
		return clientConfig{}, overflow
	}
	return config, nil
}

func clientOptions(config clientConfig) ([]neta.ClientOption, error) {
	address := "127.0.0.1:2775"
	if config.Address != nil {
		address = *config.Address
	}
	systemID := "neta-client-config"
	if config.SystemID != nil {
		systemID = *config.SystemID
	}
	options := []neta.ClientOption{
		neta.WithClientAddress(address),
		neta.WithClientSystemID(systemID),
	}
	if config.BindType != nil {
		switch strings.ToLower(*config.BindType) {
		case "bi":
			options = append(options, neta.WithClientBindType(neta.BindTypeBiDirectional))
		case "uni":
			options = append(options, neta.WithClientBindType(neta.BindTypeUniDirectional))
		default:
			return nil, fmt.Errorf("bind_type: expected bi or uni, got %q", *config.BindType)
		}
	}
	sessionOptions := []neta.SessionOption{}
	if config.ProtocolVersion != nil {
		switch strings.ToLower(*config.ProtocolVersion) {
		case "v1":
			sessionOptions = append(sessionOptions, neta.WithProtocolVersion(neta.ProtocolVersionV1))
		case "v2":
			sessionOptions = append(sessionOptions, neta.WithProtocolVersion(neta.ProtocolVersionV2))
		default:
			return nil, fmt.Errorf("protocol_version: expected v1 or v2, got %q", *config.ProtocolVersion)
		}
	}
	parseDuration := func(name string, value *string) (time.Duration, error) {
		duration, err := time.ParseDuration(*value)
		if err != nil {
			return 0, fmt.Errorf("%s %q: %w", name, *value, err)
		}
		return duration, nil
	}
	if config.ConnectTimeout != nil {
		d, err := parseDuration("connect_timeout", config.ConnectTimeout)
		if err != nil { return nil, err }
		options = append(options, neta.WithConnectTimeout(d))
	}
	if config.BindResponseTimeout != nil {
		d, err := parseDuration("bind_response_timeout", config.BindResponseTimeout)
		if err != nil { return nil, err }
		options = append(options, neta.WithBindResponseTimeout(d))
	}
	if config.RequestTimeout != nil {
		d, err := parseDuration("request_timeout", config.RequestTimeout)
		if err != nil { return nil, err }
		sessionOptions = append(sessionOptions, neta.WithRequestTimeout(d))
	}
	if config.WriteTimeout != nil {
		d, err := parseDuration("write_timeout", config.WriteTimeout)
		if err != nil { return nil, err }
		sessionOptions = append(sessionOptions, neta.WithWriteTimeout(d))
	}
	if config.EnquireIdle != nil {
		d, err := parseDuration("enquire_link_idle_interval", config.EnquireIdle)
		if err != nil { return nil, err }
		sessionOptions = append(sessionOptions, neta.WithEnquireLinkIdleInterval(d))
	}
	if config.MaxOutstanding != nil {
		sessionOptions = append(sessionOptions, neta.WithMaxOutstandingRequestsPerSession(*config.MaxOutstanding))
	}
	if len(sessionOptions) > 0 {
		options = append(options, neta.WithClientSessionOptions(sessionOptions...))
	}
	if config.AutoReconnect != nil {
		options = append(options, neta.WithAutoReconnect(*config.AutoReconnect))
	}
	if config.ApplicationWorkers != nil {
		options = append(options, neta.WithClientApplicationWorkers(*config.ApplicationWorkers))
	}
	if config.MaxRetainedAppBytes != nil {
		options = append(options, neta.WithClientMaxRetainedApplicationBytes(*config.MaxRetainedAppBytes))
	}
	return options, nil
}

func run(ctx context.Context, args []string) error {
	config, err := parseClientArgs(args)
	if err != nil { return err }
	opts, err := clientOptions(config)
	if err != nil { return err }
	opts = append(opts, neta.WithClientOnBoundHandler(func(s *neta.Session) {
		log.Printf("bound: version=%d system_id=%q", s.ProtocolVersion(), s.SystemID())
	}))
	client, err := neta.NewClient(opts...)
	if err != nil { return fmt.Errorf("validate client config: %w", err) }
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := client.Stop(stopCtx); err != nil { log.Printf("Client.Stop: %v", err) }
	}()
	connectCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	err = client.Connect(connectCtx)
	cancel()
	if err != nil { return fmt.Errorf("Connect: %w", err) }
	session, ok := client.CurrentSession()
	if !ok { return fmt.Errorf("Connect returned no Bound Session") }

	type result struct { response neta.Response; err error }
	resultCh := make(chan result, 1)
	seq, err := session.SendRequest([]byte("hello from client-config"), func(r neta.Response, completionErr error) {
		resultCh <- result{response: r, err: completionErr}
	})
	if err != nil { return fmt.Errorf("SendRequest: %w", err) }
	log.Printf("Request sequence=%d admitted locally (not a delivery receipt)", seq)
	select {
	case r := <-resultCh:
		if r.err != nil { return fmt.Errorf("Completion sequence=%d: %w", seq, r.err) }
		if r.response.Status != 0 { return fmt.Errorf("Completion sequence=%d status=%d", seq, r.response.Status) }
		log.Printf("Completion sequence=%d payload=%q", r.response.Sequence, r.response.Payload)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(60*time.Second):
		return fmt.Errorf("example completion deadline exceeded")
	}
}
