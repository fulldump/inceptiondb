package apicollectionv1

import (
	"context"
	"os"

	"github.com/fulldump/box"
)

// size reports estimated sizes in bytes. It is O(1): it reads counters
// instead of walking the data.
func size(ctx context.Context) (interface{}, error) {

	s := GetServicer(ctx)
	collectionName := box.GetUrlParameter(ctx, "collectionName")
	col, err := s.GetCollection(collectionName)
	if err != nil {
		return nil, err // todo: handle/wrap this properly
	}

	result := map[string]interface{}{}

	result["memory"] = col.MemorySize()

	// Disk
	info, err := os.Stat(col.Filepath())
	if err == nil {
		result["disk"] = info.Size()
	}

	// Indexes
	for name, index := range col.ListIndexes() {
		result["index."+name] = index.Size()
	}

	return result, nil
}
