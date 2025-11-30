package p2p

import (
	"fmt"
	"sync"
)

type PieceManager struct {
	mu        sync.RWMutex
	numPieces int
	pieces    map[int][]byte //idx -> data
}

func NewPieceManager(numPieces int) *PieceManager {
	return &PieceManager{
		numPieces: numPieces,
		pieces:    make(map[int][]byte),
	}
}

func (pm *PieceManager) GetPiece(idx int) ([]byte, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	data, exists := pm.pieces[idx]
	return data, exists
}

func (pm *PieceManager) AddPiece(idx int, data []byte) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if idx < 0 || idx >= pm.numPieces {
		return fmt.Errorf("piece index out of range")
	}
	if _, exists := pm.pieces[idx]; exists {
		return fmt.Errorf("piece %d already exists", idx)
	}
	pm.pieces[idx] = data
	return nil
}

func (pm *PieceManager) Bitfield() []byte {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	size := (pm.numPieces + 7) / 8
	bitfield := make([]byte, size)

	for i := 0; i < pm.numPieces; i++ {
		if _, ok := pm.pieces[i]; ok {
			byteIndex := i / 8
			bitIndex := 7 - (i % 8) // MSB first, like BitTorrent spec
			bitfield[byteIndex] |= 1 << bitIndex
		}
	}
	return bitfield
}

func (pm *PieceManager) MissingPieces(bitfield []byte) []int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	missing := []int{}
	for i := 0; i < pm.numPieces; i++ {
		byteIndex := i / 8
		bitIndex := 7 - (i % 8)
		if byteIndex >= len(bitfield) {
			break
		}

		peerHas := (bitfield[byteIndex] & (1 << bitIndex)) != 0
		if peerHas {
			if _, ok := pm.pieces[i]; !ok {
				missing = append(missing, i)
			}
		}
	}
	return missing
}

func (pm *PieceManager) ReceivedCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return len(pm.pieces)
}

func (pm *PieceManager) NumPieces() int {
	return pm.numPieces
}

func (pm *PieceManager) AllPiecesReceived() bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return len(pm.pieces) == pm.numPieces
}
