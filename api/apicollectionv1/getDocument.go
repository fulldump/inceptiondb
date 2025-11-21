package apicollectionv1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/collectionv2"
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

	row, source, err := findRowByID(col, documentID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		w.WriteHeader(http.StatusNotFound)
		return nil, fmt.Errorf("document '%s' not found", documentID)
	}

	document := map[string]any{}
	if err := json.Unmarshal(row.Payload, &document); err != nil {
		return nil, fmt.Errorf("decode document: %w", err)
	}

	return &documentLookupResponse{
		ID:       documentID,
		Document: document,
		Source:   source,
	}, nil
}

func findRowByID(col *collectionv2.Collection, documentID string) (*collectionv2.Row, *documentLookupSource, error) {

	var found *collectionv2.Row

	normalizedID := strings.TrimSpace(documentID)
	if normalizedID == "" {
		return nil, nil, nil
	}

	type mapLookupPayload struct {
		Value string `json:"value"`
	}

	// for name, idx := range col.Indexes {
	// if idx == nil || idx.Index == nil {
	// 	continue
	// }
	// if idx.Type != "map" {
	// 	continue
	// }

	// mapOptions, err := normalizeMapOptions(idx.Options)
	// if err != nil || mapOptions == nil {
	// 	continue
	// }
	// if mapOptions.Field != "id" {
	// 	continue
	// }

	// payload, err := json.Marshal(&mapLookupPayload{Value: normalizedID})
	// if err != nil {
	// 	return nil, nil, fmt.Errorf("prepare index lookup: %w", err)
	// }

	// idx.Traverse(payload, func(row *collectionv2.Row) bool {
	// 	found = row
	// 	return false
	// })

	// if found != nil {
	// 	return found, &documentLookupSource{Type: "index", Name: name}, nil
	// }
	// }

	col.Rows.Ascend(func(row *collectionv2.Row) bool {
		var item map[string]any
		if err := json.Unmarshal(row.Payload, &item); err != nil {
			return true
		}
		value, exists := item["id"]
		if !exists {
			return true
		}
		if normalizeDocumentID(value) == normalizedID {
			found = row
			return false
		}
		return true
	})

	fmt.Println("FOUND", found)

	if found == nil {
		return nil, nil, nil
	}

	return found, &documentLookupSource{Type: "fullscan"}, nil
}

func normalizeMapOptions(options interface{}) (*collectionv2.IndexMapOptions, error) {

	if options == nil {
		return nil, nil
	}

	switch value := options.(type) {
	case *collectionv2.IndexMapOptions:
		return value, nil
	case collectionv2.IndexMapOptions:
		return &value, nil
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		opts := &collectionv2.IndexMapOptions{}
		if err := json.Unmarshal(data, opts); err != nil {
			return nil, err
		}
		return opts, nil
	}
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
