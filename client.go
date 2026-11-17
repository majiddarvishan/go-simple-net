package neta

// NewClient constructs a Client with a validated immutable configuration.
// It does not dial, open a socket, or start goroutines.
func NewClient(options ...ClientOption) (*Client, error) {
	config := defaultClientConfig()
	for i, option := range options {
		if option.apply == nil {
			if config.invalidOption == "" {
				config.invalidOption = "client_options[" + itoa(i) + "]"
			}
			continue
		}
		option.apply(&config)
	}
	if err := validateClientConfig(config); err != nil {
		return nil, err
	}
	connectGate := make(chan struct{}, 1)
	connectGate <- struct{}{}
	return &Client{
		config:        config,
		resources:     newClientResourceLedger(config.owner),
		callbacks:     newOwnerCallbackDispatcher(config.owner),
		connectGate:   connectGate,
		reconnectWake: make(chan clientReconnectEvent, 1),
	}, nil
}
