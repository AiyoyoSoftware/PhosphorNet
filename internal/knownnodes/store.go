package knownnodes

import (
	"fmt"
	"os"

	toml "github.com/pelletier/go-toml/v2"
)

type KnownNodes struct {
	Nodes []Record `toml:"node"`
}

type Record struct {
	Address   string `toml:"address"`
	PublicKey string `toml:"public_key"`
	Name      string `toml:"name"`
}

func Load(path string) (*KnownNodes, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &KnownNodes{}, nil
		}
		return nil, fmt.Errorf("read known nodes: %w", err)
	}

	var store KnownNodes
	if err := toml.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("parse known nodes: %w", err)
	}
	return &store, nil
}

func Save(path string, store *KnownNodes) error {
	data, err := toml.Marshal(store)
	if err != nil {
		return fmt.Errorf("marshal known nodes: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

func (k *KnownNodes) Upsert(record Record) {
	for index := range k.Nodes {
		if k.Nodes[index].Address == record.Address {
			k.Nodes[index] = record
			return
		}
	}
	k.Nodes = append(k.Nodes, record)
}

func (k *KnownNodes) Find(address string) (Record, bool) {
	for _, record := range k.Nodes {
		if record.Address == address {
			return record, true
		}
	}
	return Record{}, false
}
