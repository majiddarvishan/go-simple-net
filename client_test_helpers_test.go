package neta

// newTestClient supplies the mandatory Client identity while preserving the
// caller's option order (and therefore last-option-wins behavior).
func newTestClient(options ...ClientOption) (*Client, error) {
	if len(options) == 0 {
		return NewClient(WithClientSystemID("test-client"))
	}
	combined := make([]ClientOption, 0, len(options)+1)
	combined = append(combined, options[0], WithClientSystemID("test-client"))
	combined = append(combined, options[1:]...)
	return NewClient(combined...)
}
