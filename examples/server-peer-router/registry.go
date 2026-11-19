package main

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	neta "github.com/majiddarvishan/neta-go"
)

// peerRegistry owns volatile per-peer Session pointers; never a durable message queue.
type peerRegistry struct {
	mu       sync.RWMutex
	bindToPeer map[string]string
	routes   map[string]string
	sessions map[string]*neta.Session
	inflight chan struct{}
}

var (
	errUnknownRoute = errors.New("route is not configured")
	errPeerOffline    = errors.New("target peer has no bound session")
	errDispatchBusy = errors.New("demo dispatch capacity exhausted")
)

type peerSnapshot struct {
	PeerID     string
	SystemID string
	Online   bool
}

func newPeerRegistry(cfg demoConfig) *peerRegistry {
	r := &peerRegistry{
		bindToPeer: make(map[string]string, len(cfg.Peers)),
		routes:   make(map[string]string, len(cfg.Routes)),
		sessions: make(map[string]*neta.Session, len(cfg.Peers)),
		inflight: make(chan struct{}, cfg.MaxInflight),
	}
	for _, peer := range cfg.Peers {
		r.bindToPeer[peer.BindSystemID] = peer.ID
	}
	for key, peerID := range cfg.Routes {
		r.routes[key] = peerID
	}
	return r
}

func (r *peerRegistry) validateBind(info neta.BindInfo) error {
	if info.BindType != uint8(neta.BindTypeBiDirectional) {
		return fmt.Errorf("internal peer link requires a bidirectional neta Bind")
	}
	if _, known := r.bindToPeer[info.SystemID]; !known {
		return fmt.Errorf("unknown internal peer Bind identity")
	}
	// This is a configured allowlist, NOT cryptographic authentication.
	// Production needs the application application HELLO, role and epoch validation.
	return nil
}

func (r *peerRegistry) onBound(s *neta.Session) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if peerID, known := r.bindToPeer[s.SystemID()]; known {
		r.sessions[peerID] = s
	}
}

func (r *peerRegistry) onClose(s *neta.Session) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if peerID, known := r.bindToPeer[s.SystemID()]; known && r.sessions[peerID] == s {
		// A late old OnClose never evicts a newer bound generation.
		delete(r.sessions, peerID)
	}
}

func (r *peerRegistry) lookup(routeKey string) (string, *neta.Session, error) {
	r.mu.RLock()
	peerID, known := r.routes[routeKey]
	s := r.sessions[peerID]
	r.mu.RUnlock()
	if !known {
		return "", nil, fmt.Errorf("%w: %q", errUnknownRoute, routeKey)
	}
	if s == nil || s.State() != neta.SessionStateBound {
		return peerID, nil, fmt.Errorf("%w: %s", errPeerOffline, peerID)
	}
	return peerID, s, nil
}

// dispatch locally admits exactly one Request to exactly one configured peer.
// Never replay on an ambiguous result, disconnect or reconnect.
func (r *peerRegistry) dispatch(routeKey string, payload []byte, completion neta.CompletionHandler) (string, uint32, error) {
	if completion == nil {
		return "", 0, errors.New("completion must not be nil")
	}
	peerID, s, err := r.lookup(routeKey)
	if err != nil {
		return peerID, 0, err
	}
	select {
	case r.inflight <- struct{}{}:
	default:
		return peerID, 0, errDispatchBusy
	}
	seq, err := s.SendRequest(payload, func(resp neta.Response, completionErr error) {
		defer func() { <-r.inflight }()
		completion(resp, completionErr)
	})
	if err != nil {
		<-r.inflight
		return peerID, 0, err
	}
	// Success is local queue admission, NOT delivery or durable application ACK.
	return peerID, seq, nil
}

func (r *peerRegistry) peers() []peerSnapshot {
	r.mu.RLock()
	out := make([]peerSnapshot, 0, len(r.bindToPeer))
	for bindID, peerID := range r.bindToPeer {
		s := r.sessions[peerID]
		out = append(out, peerSnapshot{
			PeerID:     peerID,
			SystemID: bindID,
			Online:   s != nil && s.State() == neta.SessionStateBound,
		})
	}
	r.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].PeerID < out[j].PeerID })
	return out
}
