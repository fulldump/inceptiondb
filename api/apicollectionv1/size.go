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

	// Data memory
	memory := utils.SizeOf(col.Rows)
	result["memory"] = memory

	// Disk
	info, err := os.Stat(col.Filename)
	if err == nil {
		result["disk"] = info.Size()
	}

	// Indexes
	for name, index := range col.Indexes {
		result["index."+name] = utils.SizeOf(index) - memory
	}

	return result, nil
}
