package neta

// NewClient applies all Client Options to complete defaults, validates the
// resulting immutable configuration, and performs no network operation.
func NewClient(options ...ClientOption) (*Client, error) {
	config := defaultClientConfig()
	for i, option := range options {
		if option == nil {
			config.optionErrors = append(config.optionErrors, optionIndex("Client", i))
			continue
		}
		option(&config)
	}
	if err := config.validate(); err != nil {
		return nil, err
	}
	config.optionErrors = nil
	return &Client{config: config}, nil
}

// NewServer applies all Server Options to complete defaults, validates the
// resulting immutable configuration, and performs no network operation.
func NewServer(options ...ServerOption) (*Server, error) {
	config := defaultServerConfig()
	for i, option := range options {
		if option == nil {
			config.optionErrors = append(config.optionErrors, optionIndex("Server", i))
			continue
		}
		option(&config)
	}
	if err := config.validate(); err != nil {
		return nil, err
	}
	config.optionErrors = nil
	return &Server{config: config}, nil
}
