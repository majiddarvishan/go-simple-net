package neta

// NewServer constructs a Server with a validated immutable configuration.
// It does not listen, open a socket, or start goroutines.
func NewServer(options ...ServerOption) (*Server, error) {
	config := defaultServerConfig()
	for i, option := range options {
		if option.apply == nil {
			if config.invalidOption == "" {
				config.invalidOption = "server_options[" + itoa(i) + "]"
			}
			continue
		}
		option.apply(&config)
	}
	if err := validateServerConfig(config); err != nil {
		return nil, err
	}
	// Unlike pendingSockets, these permits survive a timed-out Session until
	// its user-supplied validator actually returns, bounding blocked callbacks.
	var validatorSlots chan struct{}
	if config.bindValidator != nil {
		validatorSlots = make(chan struct{}, config.maxPendingPreBindSessions)
	}
	return &Server{
		config:         config,
		validatorSlots: validatorSlots,
		resources:  newServerResourceLedger(config.owner, config),
		callbacks:  newOwnerCallbackDispatcher(config.owner),
		sessions:   make(map[*Session]struct{}),
		preBind:    make(map[*Session]struct{}),
		pendingIDs: make(map[string]*Session),
		boundIDs:   make(map[string]*Session),
		sessionIDs: make(map[*Session]string),
	}, nil
}
