package apicollectionv1

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/fulldump/box"
	"github.com/fulldump/inceptiondb/collection"
)

func patch(ctx context.Context, w http.ResponseWriter, r *http.Request) error {

	s := GetServicer(ctx)
	collectionName := box.GetUrlParameter(ctx, "collectionName")
	col, err := s.GetCollection(collectionName)
	if err != nil {
		return err // todo: handle/wrap this properly
	}

	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	patch := struct {
		Filter map[string]interface{}
		Patch  json.RawMessage
	}{}
	json.Unmarshal(requestBody, &patch) // TODO: handle err

	plan, err := collection.CompilePatch(patch.Patch)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return err
	}

	wb := bufio.NewWriterSize(w, 64*1024)
	defer wb.Flush()

	// With durable writes, patched documents are only written back once
	// they are fsynced, see insert.
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
		wb.Write(acks.Bytes())
		acks.Reset()
		pending = 0
		return nil
	}

	var result error
	traverse(requestBody, col, func(id int64, payload []byte) bool {

		updated, err := col.PatchDeferred(id, plan)
		if err != nil {
			// TODO: handle err??
			// return err
			return true // todo: OR return false?
		}

		if !durable {
			wb.Write(updated)
			wb.Write([]byte("\n"))
			return true
		}

		acks.Write(updated)
		acks.WriteByte('\n')
		pending++
		if pending >= maxPendingAcks {
			if result = commit(); result != nil {
				return false
			}
		}
		return true
	})

	if result != nil {
		return result
	}
	return commit()
}
