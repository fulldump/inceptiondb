package apicollectionv1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/collection"
	"github.com/fulldump/inceptiondb/service"
)

type documentLookupSource struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

type documentLookupResponse struct {
	ID       string                `json:"id"`
	Document map[string]any        `json:"document"`
	Source   *documentLookupSource `json:"source,omitempty"`
}

func getDocument(ctx context.Context) (*documentLookupResponse, error) {

	s := GetServicer(ctx)
	w := box.GetResponse(ctx)

	collectionName := box.GetUrlParameter(ctx, "collectionName")
	documentID := strings.TrimSpace(box.GetUrlParameter(ctx, "documentId"))

	if documentID == "" {
		w.WriteHeader(http.StatusBadRequest)
		return nil, fmt.Errorf("document id is required")
	}

	col, err := s.GetCollection(collectionName)
	if err != nil {
		if err == service.ErrorCollectionNotFound {
			w.WriteHeader(http.StatusNotFound)
		}
		return nil, err
	}

	payload, source, err := findRowByID(col, documentID)
	if err != nil {
		return nil, err
	}
	if payload == nil {
		w.WriteHeader(http.StatusNotFound)
		return nil, fmt.Errorf("document '%s' not found", documentID)
	}

	document := map[string]any{}
	if err := json.Unmarshal(payload, &document); err != nil {
		return nil, fmt.Errorf("decode document: %w", err)
	}

	return &documentLookupResponse{
		ID:       documentID,
		Document: document,
		Source:   source,
	}, nil
}

// findRowByID looks the document up in an index on "id" and falls back to a
// full scan when there is none (e.g. the user dropped it).
func findRowByID(col *collection.Collection, documentID string) ([]byte, *documentLookupSource, error) {

	var found []byte

	normalizedID := strings.TrimSpace(documentID)
	if normalizedID == "" {
		return nil, nil, nil
	}

	if name, options, ok := idIndex(col, normalizedID); ok {
		err := col.TraverseIndex(name, options, func(id int64, payload []byte) bool {
			found = payload
			return false
		})
		if err != nil || found == nil {
			return nil, nil, err
		}
		return found, &documentLookupSource{Type: "index", Name: name}, nil
	}

	rows := col.Scan()
	for rows.Next() {
		_, payload := rows.Read()
		var item map[string]any
		if err := json.Unmarshal(payload, &item); err != nil {
			continue
		}
		value, exists := item["id"]
		if !exists {
			continue
		}
		if normalizeDocumentID(value) == normalizedID {
			found = payload
			break
		}
	}

	if found == nil {
		return nil, nil, nil
	}

	return found, &documentLookupSource{Type: "fullscan"}, nil
}

// idIndex returns an index on "id" (pk or map) and the traverse options to
// look up id. A sparse map only skips documents without id, so it is valid too.
func idIndex(col *collection.Collection, id string) (string, []byte, bool) {
	indexes := col.ListIndexes()
	names := make([]string, 0, len(indexes))
	for name := range indexes {
		names = append(names, name)
	}
	sort.Strings(names) // stable choice when there are several

	for _, name := range names {
		switch index := indexes[name].(type) {
		case *collection.IndexPK:
			paths := index.GetOptions().(*collection.IndexPKOptions).Paths
			if len(paths) == 1 && len(paths[0]) == 1 && paths[0][0] == "id" {
				return name, []byte(id), true
			}
		case *collection.IndexMap:
			if index.Options.Field == "id" {
				options, _ := json.Marshal(collection.IndexMapTraverse{Value: id})
				return name, options, true
			}
		}
	}
	return "", nil, false
}

func normalizeDocumentID(value interface{}) string {

	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}
