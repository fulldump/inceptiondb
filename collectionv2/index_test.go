package collectionv2

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestIndexMap(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "collection_index_map_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// 1. Create collection with index
	c, err := OpenCollection(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	err = c.Index("by_email", &IndexMapOptions{
		Field: "email",
	})
	if err != nil {
		t.Fatal(err)
	}

	// 2. Insert documents
	_, err = c.Insert(map[string]any{"id": 1, "email": "alice@example.com", "name": "Alice"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Insert(map[string]any{"id": 2, "email": "bob@example.com", "name": "Bob"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Insert(map[string]any{"id": 3, "email": "charlie@example.com", "name": "Charlie"})
	if err != nil {
		t.Fatal(err)
	}

	// 3. Check index works
	// Helper to query index
	queryIndex := func(c *Collection, indexName string, value string) *Row {
		var found *Row
		index := c.Indexes[indexName]
		if index == nil {
			return nil
		}
		opts, _ := json.Marshal(IndexMapTraverse{Value: value})
		index.Traverse(opts, func(r *Row) bool {
			found = r
			return false // stop
		})
		return found
	}

	row := queryIndex(c, "by_email", "bob@example.com")
	if row == nil {
		t.Fatal("expected to find bob")
	}
	var data map[string]any
	json.Unmarshal(row.Payload, &data)
	if data["name"] != "Bob" {
		t.Fatalf("expected name Bob, got %v", data["name"])
	}

	row = queryIndex(c, "by_email", "david@example.com")
	if row != nil {
		t.Fatal("expected not to find david")
	}

	// 4. Close and reopen
	err = c.Close()
	if err != nil {
		t.Fatal(err)
	}

	c2, err := OpenCollection(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()

	// 5. Check index still works
	// Verify index exists
	if _, ok := c2.Indexes["by_email"]; !ok {
		t.Fatal("index by_email missing after reload")
	}
	if c2.Indexes["by_email"].GetType() != "map" {
		t.Fatal("index type mismatch")
	}

	row = queryIndex(c2, "by_email", "alice@example.com")
	if row == nil {
		t.Fatal("expected to find alice after reload")
	}
	json.Unmarshal(row.Payload, &data)
	if data["name"] != "Alice" {
		t.Fatalf("expected name Alice, got %v", data["name"])
	}

	// Test duplicate error
	_, err = c2.Insert(map[string]any{"id": 4, "email": "alice@example.com", "name": "Alice Duplicate"})
	if err == nil {
		t.Fatal("expected duplicate error")
	}
	if !strings.Contains(err.Error(), "index conflict") {
		t.Fatalf("expected index conflict error, got: %v", err)
	}
}

func TestIndexBTree(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "collection_index_btree_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// 1. Create collection with index
	c, err := OpenCollection(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	err = c.Index("by_age", &IndexBTreeOptions{
		Fields: []string{"age"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// 2. Insert documents
	_, err = c.Insert(map[string]any{"id": 1, "age": 30, "name": "Alice"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Insert(map[string]any{"id": 2, "age": 20, "name": "Bob"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Insert(map[string]any{"id": 3, "age": 40, "name": "Charlie"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Insert(map[string]any{"id": 4, "age": 25, "name": "David"})
	if err != nil {
		t.Fatal(err)
	}

	// 3. Check index works (Range query)
	// Helper to query index
	queryIndexRange := func(c *Collection, indexName string, from, to int) []*Row {
		var results []*Row
		index := c.Indexes[indexName]
		if index == nil {
			return nil
		}

		// Construct traverse options
		optsStruct := IndexBtreeTraverse{
			From: map[string]interface{}{"age": from},
			To:   map[string]interface{}{"age": to},
		}
		opts, _ := json.Marshal(optsStruct)

		index.Traverse(opts, func(r *Row) bool {
			results = append(results, r)
			return true // continue
		})
		return results
	}

	// Query age 20 to 30 (inclusive start, exclusive end? BTree semantics depend on implementation)
	// Looking at index_adapters.go:
	// AscendRange(pivotFrom, pivotTo, iterator)
	// google/btree AscendRange is [a, b)

	rows := queryIndexRange(c, "by_age", 20, 31)
	// Expected: 20 (Bob), 25 (David), 30 (Alice)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}

	// Verify order
	var data map[string]any
	json.Unmarshal(rows[0].Payload, &data)
	if data["name"] != "Bob" {
		t.Errorf("expected Bob, got %v", data["name"])
	}
	json.Unmarshal(rows[1].Payload, &data)
	if data["name"] != "David" {
		t.Errorf("expected David, got %v", data["name"])
	}
	json.Unmarshal(rows[2].Payload, &data)
	if data["name"] != "Alice" {
		t.Errorf("expected Alice, got %v", data["name"])
	}

	// 4. Close and reopen
	err = c.Close()
	if err != nil {
		t.Fatal(err)
	}

	c2, err := OpenCollection(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()

	// 5. Check index still works
	if _, ok := c2.Indexes["by_age"]; !ok {
		t.Fatal("index by_age missing after reload")
	}
	if c2.Indexes["by_age"].GetType() != "btree" {
		t.Fatal("index type mismatch")
	}

	rows = queryIndexRange(c2, "by_age", 25, 41)
	// Expected: 25 (David), 30 (Alice), 40 (Charlie)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows after reload, got %d", len(rows))
	}

	json.Unmarshal(rows[0].Payload, &data)
	if data["name"] != "David" {
		t.Errorf("expected David, got %v", data["name"])
	}
	json.Unmarshal(rows[2].Payload, &data)
	if data["name"] != "Charlie" {
		t.Errorf("expected Charlie, got %v", data["name"])
	}
}
