package apicollectionv1

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/collection"
)

func remove(ctx context.Context, w http.ResponseWriter, r *http.Request) error {

	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	input := struct {
		Index string
	}{
		Index: "",
	}
	err = json.Unmarshal(requestBody, &input)
	if err != nil {
		return err
	}

	s := GetServicer(ctx)
	collectionName := box.GetUrlParameter(ctx, "collectionName")
	col, err := s.GetCollection(collectionName)
	if err != nil {
		return err // todo: handle/wrap this properly
	}

	// With durable writes, removed documents are only written back once the
	// removal is fsynced, see insert.
	waitParam := r.URL.Query().Get("wait") == "true"
	durable := waitParam || col.Durability() == collection.DurabilityStrong
	acks := &bytes.Buffer{}
	pending := 0
	commit := func() error {
		if pending == 0 {
			return nil
		}
		if err := col.Commit(waitParam); err != nil {
			return err
		}
		w.Write(acks.Bytes())
		acks.Reset()
		pending = 0
		return nil
	}

	var result error

	traverse(requestBody, col, func(id int64, payload []byte) bool {
		err := col.DeleteDeferred(id)
		if err != nil {
			result = err
			return false
		}

		if !durable {
			w.Write(payload)
			w.Write([]byte("\n"))
			return true
		}

		acks.Write(payload)
		acks.WriteByte('\n')
		pending++
		if pending >= maxPendingAcks {
			if result = commit(); result != nil {
				return false
			}
		}
		return true
	})

	if commitErr := commit(); result == nil {
		result = commitErr
	}
	return result
}
