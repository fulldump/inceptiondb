package apicollectionv1

import (
	"context"
	"os"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/utils"
)

// This is experimental
func size(ctx context.Context) (interface{}, error) {

	s := GetServicer(ctx)
	collectionName := box.GetUrlParameter(ctx, "collectionName")
	col, err := s.GetCollection(collectionName)
	if err != nil {
		return nil, err // todo: handle/wrap this properly
	}

	result := map[string]interface{}{}

	result["memory"] = utils.SizeOf(col)

	// Disk
	info, err := os.Stat(col.Filepath())
	if err == nil {
		result["disk"] = info.Size()
	}

	// Indexes
	for name, index := range col.ListIndexes() {
		result["index."+name] = utils.SizeOf(index)
	}

	return result, nil
}
