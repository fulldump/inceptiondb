package apicollectionv1

import (
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"net/http"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/service"
)

type setDefaultsInput map[string]any

func setDefaults(ctx context.Context, w http.ResponseWriter, r *http.Request) error {

	s := GetServicer(ctx)
	collectionName := box.GetUrlParameter(ctx, "collectionName")
	col, err := s.GetCollection(collectionName)
	if err == service.ErrorCollectionNotFound {
		col, err = s.CreateCollection(collectionName)
		if err != nil {
			return err // todo: handle/wrap this properly
		}
		err = col.SetDefaults(newCollectionDefaults())
		if err != nil {
			return err // todo: handle/wrap this properly
		}
	}
	if err != nil {
		return err // todo: handle/wrap this properly
	}

	defaults := col.Defaults
	bodyDecoder := jsontext.NewDecoder(r.Body,
		jsontext.AllowDuplicateNames(true),
		jsontext.AllowInvalidUTF8(true),
	)
	err = jsonv2.UnmarshalDecode(bodyDecoder, &defaults)
	if err != nil {
		return err // todo: handle/wrap this properly
	}

	for k, v := range defaults {
		if v == nil {
			delete(defaults, k)
		}
	}

	if len(defaults) == 0 {
		defaults = nil
	}

	err = col.SetDefaults(defaults)
	if err != nil {
		return err
	}

	err = json.NewEncoder(w).Encode(col.Defaults)
	if err != nil {
		return err // todo: handle/wrap this properly
	}

	return nil
}
