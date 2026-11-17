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
	return &Client{
		config:    config,
		resources: newClientResourceLedger(config.owner),
	}, nil
}
