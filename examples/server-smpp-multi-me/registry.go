package main

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	neta "github.com/majiddarvishan/neta-go"
)

// meRegistry owns volatile per-ME Session pointers; never a durable message queue.
type meRegistry struct {
	mu       sync.RWMutex
	bindToME map[string]string
	routes   map[string]string
	sessions map[string]*neta.Session
	inflight chan struct{}
}

var (
	errUnknownRoute = errors.New("route is not configured")
	errMEOffline    = errors.New("target ME has no bound session")
	errDispatchBusy = errors.New("demo dispatch capacity exhausted")
)

type peerSnapshot struct {
	MEID     string
	SystemID string
	Online   bool
}

func newMERegistry(cfg demoConfig) *meRegistry {
	r := &meRegistry{
		bindToME: make(map[string]string, len(cfg.MessageEngines)),
		routes:   make(map[string]string, len(cfg.Routes)),
		sessions: make(map[string]*neta.Session, len(cfg.MessageEngines)),
		inflight: make(chan struct{}, cfg.MaxInflight),
	}
	for _, me := range cfg.MessageEngines {
		r.bindToME[me.BindSystemID] = me.ID
	}
	for key, meID := range cfg.Routes {
		r.routes[key] = meID
	}
	return r
}

func (r *meRegistry) validateBind(info neta.BindInfo) error {
	if info.BindType != uint8(neta.BindTypeBiDirectional) {
		return fmt.Errorf("internal ME link requires a bidirectional neta Bind")
	}
	if _, known := r.bindToME[info.SystemID]; !known {
		return fmt.Errorf("unknown internal ME Bind identity")
	}
	// This is a configured allowlist, NOT cryptographic authentication.
	// Production needs the SMSC application HELLO, role and epoch validation.
	return nil
}

func (r *meRegistry) onBound(s *neta.Session) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if meID, known := r.bindToME[s.SystemID()]; known {
		r.sessions[meID] = s
	}
}

func (r *meRegistry) onClose(s *neta.Session) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if meID, known := r.bindToME[s.SystemID()]; known && r.sessions[meID] == s {
		// A late old OnClose never evicts a newer bound generation.
		delete(r.sessions, meID)
	}
}

func (r *meRegistry) lookup(routeKey string) (string, *neta.Session, error) {
	r.mu.RLock()
	meID, known := r.routes[routeKey]
	s := r.sessions[meID]
	r.mu.RUnlock()
	if !known {
		return "", nil, fmt.Errorf("%w: %q", errUnknownRoute, routeKey)
	}
	if s == nil || s.State() != neta.SessionStateBound {
		return meID, nil, fmt.Errorf("%w: %s", errMEOffline, meID)
	}
	return meID, s, nil
}

// dispatch locally admits exactly one Request to exactly one configured ME.
// Never replay on an ambiguous result, disconnect or reconnect.
func (r *meRegistry) dispatch(routeKey string, payload []byte, completion neta.CompletionHandler) (string, uint32, error) {
	if completion == nil {
		return "", 0, errors.New("completion must not be nil")
	}
	meID, s, err := r.lookup(routeKey)
	if err != nil {
		return meID, 0, err
	}
	select {
	case r.inflight <- struct{}{}:
	default:
		return meID, 0, errDispatchBusy
	}
	seq, err := s.SendRequest(payload, func(resp neta.Response, completionErr error) {
		defer func() { <-r.inflight }()
		completion(resp, completionErr)
	})
	if err != nil {
		<-r.inflight
		return meID, 0, err
	}
	// Success is local queue admission, NOT delivery or durable SMSC ACK.
	return meID, seq, nil
}

func (r *meRegistry) peers() []peerSnapshot {
	r.mu.RLock()
	out := make([]peerSnapshot, 0, len(r.bindToME))
	for bindID, meID := range r.bindToME {
		s := r.sessions[meID]
		out = append(out, peerSnapshot{
			MEID:     meID,
			SystemID: bindID,
			Online:   s != nil && s.State() == neta.SessionStateBound,
		})
	}
	r.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].MEID < out[j].MEID })
	return out
}
