package collectionv2

import (
	"bytes"
	"errors"
	"fmt"
	"hash/fnv"
	"sync"

	"github.com/buger/jsonparser"
)

const indexPKNumShards = 256

type pkShard struct {
	mu sync.RWMutex
	m  map[string]*Row
}

type IndexPK struct {
	paths  [][]string
	shards [indexPKNumShards]*pkShard
}

type IndexPKOptions struct {
	Paths [][]string `json:"paths"`
}

func NewIndexPK(options *IndexPKOptions) *IndexPK {
	idx := &IndexPK{
		paths: options.Paths,
	}
	for i := 0; i < indexPKNumShards; i++ {
		idx.shards[i] = &pkShard{
			m: make(map[string]*Row),
		}
	}
	return idx
}

func (idx *IndexPK) extractPK(payload []byte) (string, error) {
	if len(idx.paths) == 0 {
		return "", errors.New("no paths defined for IndexPK")
	}

	if len(idx.paths) == 1 {
		val, t, _, err := jsonparser.Get(payload, idx.paths[0]...)
		if err != nil {
			return "", err
		}
		if t == jsonparser.String {
			return string(val), nil
		}
		return string(val), nil
	}

	var buf bytes.Buffer
	for i, path := range idx.paths {
		val, t, _, err := jsonparser.Get(payload, path...)
		if err != nil {
			return "", err
		}
		if t == jsonparser.String {
			buf.Write(val)
		} else {
			buf.Write(val)
		}
		if i < len(idx.paths)-1 {
			buf.WriteByte('|')
		}
	}

	return buf.String(), nil
}

func getShardIndex(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32() % indexPKNumShards
}

func (idx *IndexPK) AddRow(row *Row) error {
	key, err := idx.extractPK(row.Payload)
	if err != nil {
		if errors.Is(err, jsonparser.KeyPathNotFoundError) {
			return fmt.Errorf("primary key missing in payload")
		}
		return err
	}

	shardID := getShardIndex(key)
	shard := idx.shards[shardID]

	shard.mu.Lock()
	defer shard.mu.Unlock()

	if _, exists := shard.m[key]; exists {
		return fmt.Errorf("duplicate primary key: %s", key)
	}

	shard.m[key] = row
	return nil
}

func (idx *IndexPK) RemoveRow(row *Row) error {
	key, err := idx.extractPK(row.Payload)
	if err != nil {
		return nil
	}

	shardID := getShardIndex(key)
	shard := idx.shards[shardID]

	shard.mu.Lock()
	defer shard.mu.Unlock()

	delete(shard.m, key)
	return nil
}

func (idx *IndexPK) Traverse(optionsData []byte, f func(row *Row) bool) {
	key := string(optionsData)
	if len(key) == 0 {
		return
	}

	shardID := getShardIndex(key)
	shard := idx.shards[shardID]

	shard.mu.RLock()
	row, exists := shard.m[key]
	shard.mu.RUnlock()

	if exists {
		f(row)
	}
}

func (idx *IndexPK) GetType() string {
	return "pk"
}

func (idx *IndexPK) GetOptions() interface{} {
	return &IndexPKOptions{
		Paths: idx.paths,
	}
}
