package neta

// NewServer constructs a Server with a validated immutable configuration.
// It does not listen, open a socket, or start goroutines.
func NewServer(options ...ServerOption) (*Server, error) {
	config := defaultServerConfig()
	for i, option := range options {
		if option.apply == nil {
			return nil, newConfigError("server_options["+itoa(i)+"]", "<nil>", "must be a non-zero ServerOption")
		}
		option.apply(&config)
	}
	if err := validateServerConfig(config); err != nil {
		return nil, err
	}
	return &Server{config: config}, nil
}
