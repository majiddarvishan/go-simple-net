package wire

import "sync/atomic"

type atomicUint32 struct {
	value atomic.Uint32
}

func nextSequence(s *atomicUint32) uint32 {
	for {
		next := s.value.Add(1)
		if next != 0 {
			return next
		}
	}
}
