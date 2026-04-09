package collection

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

// IndexPK is a highly concurrent Sharded Primary Key Index
// It supports composite primary keys by joining paths during extraction.
type IndexPK struct {
	paths  [][]string
	shards [indexPKNumShards]*pkShard
}

// NewIndexPK creates a new sharded primary key index
// paths expects parameters for jsonparser, for example:
// NewIndexPK([]string{"id"}) for a single ID
// NewIndexPK([]string{"company_id"}, []string{"user_id"}) for a composite PK
func NewIndexPK(paths ...[]string) *IndexPK {
	idx := &IndexPK{
		paths: paths,
	}
	for i := 0; i < indexPKNumShards; i++ {
		idx.shards[i] = &pkShard{
			m: make(map[string]*Row),
		}
	}
	return idx
}

// extractPK reads the primary key from the raw JSON without unmarshaling the entire row.
// Returns a combined string for the hash map to ensure uniqueness.
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
		// For numbers or booleans we can also just convert the raw chunk to string
		return string(val), nil
	}

	// Composite key
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
			buf.WriteByte('|') // Unify composite keys with a separator
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
			// A primary key index should strictly enforce existence of the PK field
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
		// If it doesn't have a PK, it couldn't have been inserted.
		return nil
	}

	shardID := getShardIndex(key)
	shard := idx.shards[shardID]

	shard.mu.Lock()
	defer shard.mu.Unlock()

	delete(shard.m, key)
	return nil
}

// Traverse resolves lookups for the Primary Key Index.
// Using []byte directly allows users to pass the queried PK value raw (or string/buffer).
// In a PK index, we expect 'options' to be the exact primary key to look up.
func (idx *IndexPK) Traverse(options []byte, f func(row *Row) bool) {
	// 1. Get the lookup string straight from options
	key := string(options)
	if len(key) == 0 {
		return
	}

	shardID := getShardIndex(key)
	shard := idx.shards[shardID]

	// 2. Lock solely the shard involved in the lookup
	shard.mu.RLock()
	row, exists := shard.m[key]
	shard.mu.RUnlock()

	if exists {
		f(row)
	}
}
