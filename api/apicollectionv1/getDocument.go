package apicollectionv1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/collectionv4"
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

func findRowByID(col *collectionv4.Collection, documentID string) ([]byte, *documentLookupSource, error) {

	var found []byte

	normalizedID := strings.TrimSpace(documentID)
	if normalizedID == "" {
		return nil, nil, nil
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
