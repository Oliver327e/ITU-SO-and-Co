package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
)

type Peer struct {
	ID      int32  `json:"id"`
	Address string `json:"address"`
}

func loadPeers(filename string, nodeID int32) ([]Peer, string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, "", err
	}

	var peers []Peer
	if err := json.Unmarshal(data, &peers); err != nil {
		return nil, "", err
	}

	seenIDs := make(map[int32]bool)
	seenAddresses := make(map[string]bool)
	localAddress := ""

	for _, peer := range peers {
		if peer.ID <= 0 || seenIDs[peer.ID] || seenAddresses[peer.Address] {
			return nil, "", fmt.Errorf("IDs must be positive and IDs/addresses must be unique")
		}

		if _, _, err := net.SplitHostPort(peer.Address); err != nil {
			return nil, "", fmt.Errorf("invalid address %q: %w", peer.Address, err)
		}

		seenIDs[peer.ID] = true
		seenAddresses[peer.Address] = true

		if peer.ID == nodeID {
			localAddress = peer.Address
		}
	}

	if localAddress == "" {
		return nil, "", fmt.Errorf("node %d is missing from the config", nodeID)
	}

	return peers, localAddress, nil
}