package apicollectionv1

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/fulldump/box"
	"github.com/go-json-experiment/json/jsontext"

	collectionpkg "github.com/fulldump/inceptiondb/collection"
	"github.com/fulldump/inceptiondb/service"
)

// maxPendingAcks bounds how many durable writes wait for a single commit.
const maxPendingAcks = 4096

func insert(ctx context.Context, w http.ResponseWriter, r *http.Request) error {

	wc := http.NewResponseController(w)
	wcerr := wc.EnableFullDuplex()
	if wcerr != nil {
		return wcerr
	}

	s := GetServicer(ctx)
	collectionName := box.GetUrlParameter(ctx, "collectionName")
	collection, err := s.GetCollection(collectionName)
	if err == service.ErrorCollectionNotFound {
		collection, err = s.CreateCollection(collectionName)
		if err != nil {
			return err // todo: handle/wrap this properly
		}
		err = collection.SetDefaults(newCollectionDefaults())
		if err != nil {
			return err // todo: handle/wrap this properly
		}
	}
	if err != nil {
		return err // todo: handle/wrap this properly
	}

	// READER

	// With durable writes, acknowledgments are held until the documents are
	// fsynced. Commits happen when the decoder runs out of buffered input
	// (it is about to block on the client) or every maxPendingAcks documents,
	// so one fsync covers many documents.
	waitParam := r.URL.Query().Get("wait") == "true"
	durable := waitParam || collection.Durability() == collectionpkg.DurabilityStrong
	acks := &bytes.Buffer{}
	pending := 0
	headerWritten := false
	writeHeader := func(status int) {
		if !headerWritten {
			headerWritten = true
			w.WriteHeader(status)
		}
	}
	commit := func() error {
		if pending == 0 {
			return nil
		}
		if err := collection.Commit(waitParam); err != nil {
			writeHeader(http.StatusInternalServerError)
			return err
		}
		writeHeader(http.StatusCreated)
		w.Write(acks.Bytes())
		acks.Reset()
		pending = 0
		_ = wc.Flush() // best effort: not every writer supports it
		return nil
	}

	// ALT 1
	// Rejects duplicate names (RFC 7493): which value counts would be undefined
	jsonReader := jsontext.NewDecoder(r.Body)

	for i := 0; true; i++ {
		if pending > 0 && (pending >= maxPendingAcks || len(bytes.TrimSpace(jsonReader.UnreadBuffer())) == 0) {
			if err := commit(); err != nil {
				return err
			}
		}

		payload, err := jsonReader.ReadValue()
		// READER:ALT 2
		// err := json2.UnmarshalDecode(jsonReader, &item)
		if err == io.EOF {
			if i == 0 {
				writeHeader(http.StatusNoContent)
			}
			return commit()
		}
		if err != nil {
			// TODO: handle error properly
			fmt.Println("ERROR:", err.Error())
			if commitErr := commit(); commitErr != nil {
				return commitErr
			}
			writeHeader(http.StatusBadRequest)
			return err
		}
		id, err := collection.InsertJSONDeferred(payload)
		if err != nil {
			// TODO: handle error properly
			if commitErr := commit(); commitErr != nil {
				return commitErr
			}
			if errors.Is(err, collectionpkg.ErrDocumentNotObject) {
				writeHeader(http.StatusBadRequest)
			} else {
				writeHeader(http.StatusConflict)
			}
			return err
		}

		// ALT 3
		stored, ok := collection.Get(id)
		if !ok {
			return fmt.Errorf("inserted document not found")
		}

		if durable {
			acks.Write(stored)
			acks.WriteByte('\n')
			pending++
			continue
		}

		writeHeader(http.StatusCreated)
		w.Write(stored)
		w.Write([]byte("\n"))

		// ALT 4
		// query param to optionally write nothing
	}

	return nil
}
