package collectionv4

import (
	"bytes"
	"errors"
	"fmt"
	"hash/fnv"
	"sync"

	"github.com/fulldump/inceptiondb/simdscan"
)

const indexPKNumShards = 256

type pkShard struct {
	mu sync.RWMutex
	m  map[string]int64
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
			m: make(map[string]int64),
		}
	}
	return idx
}

func (idx *IndexPK) extractPK(payload []byte) (string, error) {
	if len(idx.paths) == 0 {
		return "", errors.New("no paths defined for IndexPK")
	}

	if len(idx.paths) == 1 {
		// Single path (top-level or nested) → use SIMD
		val, _, err := simdscan.GetPath(payload, idx.paths[0]...)
		if err != nil {
			return "", err
		}
		return string(val), nil
	}

	// Composite PK: concatenate values from multiple paths
	var buf bytes.Buffer
	for i, path := range idx.paths {
		val, _, err := simdscan.GetPath(payload, path...)
		if err != nil {
			return "", err
		}
		buf.Write(val)
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

func (idx *IndexPK) Add(id int64, data []byte) error {
	key, err := idx.extractPK(data)
	if err != nil {
		if errors.Is(err, simdscan.ErrNotFound) {
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

	shard.m[key] = id
	return nil
}

func (idx *IndexPK) Remove(id int64, data []byte) error {
	key, err := idx.extractPK(data)
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

func (idx *IndexPK) Traverse(optionsData []byte, f func(id int64, data []byte) bool) {
	key := string(optionsData)
	if len(key) == 0 {
		return
	}

	shardID := getShardIndex(key)
	shard := idx.shards[shardID]

	shard.mu.RLock()
	id, exists := shard.m[key]
	shard.mu.RUnlock()

	if exists {
		f(id, nil)
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

func (idx *IndexPK) IsUnique() bool {
	return true
}
