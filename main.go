package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fulldump/box"
	"inceptiondb/collection"
	"io/fs"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type Configuration struct {
	Dir string `usage:"data directory"`
}

var banner = `
 _____                     _   _            ____________ 
|_   _|                   | | (_)           |  _  \ ___ \
  | | _ __   ___ ___ _ __ | |_ _  ___  _ __ | | | | |_/ /
  | || '_ \ / __/ _ \ '_ \| __| |/ _ \| '_ \| | | | ___ \
 _| || | | | (_|  __/ |_) | |_| | (_) | | | | |/ /| |_/ /
 \___/_| |_|\___\___| .__/ \__|_|\___/|_| |_|___/ \____/ 
                    | |                                  
                    |_|                                  
`

func main() {

	c := &Configuration{
		Dir: "data",
	}

	fmt.Println(banner)

	collections := map[string]*collection.Collection{}
	fmt.Printf("Loading data...\n")
	filepath.WalkDir(c.Dir, func(filename string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		name := filename
		name = strings.TrimPrefix(name, c.Dir)
		name = strings.TrimPrefix(name, "/")

		t0 := time.Now()
		col, err := collection.OpenCollection(filename)
		if err != nil {
			fmt.Printf("ERROR: open collection '%s': %s\n", filename, err.Error())
			return nil
		}
		fmt.Println(name, len(col.Rows), time.Since(t0))

		collections[name] = col

		return nil
	})

	b := box.NewBox()

	b.WithInterceptors(InterceptorPrintError)

	b.Resource("collections").
		WithActions(
			box.Get(listCollections(collections)),
			box.Post(createCollection(collections, c.Dir)),
		)

	b.Resource("collections/{collection_name}").
		WithActions(
			box.Get(listItems(collections)),
			box.Post(insertItem(collections)),
		)

	b.Resource("collections/{collection_name}/count").
		WithActions(
			box.Get(countItems(collections)),
		)

	b.Resource("collections/{collection_name}/index").
		WithActions(
			box.Get(listIndexes(collections)),
			box.Post(createIndex(collections)),
		)

	b.Resource("collections/{collection_name}/index/{index_name}").
		WithActions(
			box.Get(getIndex(collections)),
		)

	b.Resource("collections/{collection_name}/index/{index_name}/findBy/{value}").
		WithActions(
			box.Get(indexFindBy(collections)),
		)

	b.Serve()
}

func InterceptorPrintError(next box.H) box.H {
	return func(ctx context.Context) {
		next(ctx)
		err := box.GetError(ctx)
		if nil != err {
			json.NewEncoder(box.GetResponse(ctx)).Encode(map[string]interface{}{
				"error": err.Error(),
			})
		}
	}
}

func getBoxContext(ctx context.Context) *box.C {

	v := ctx.Value("box_context")
	if c, ok := v.(*box.C); ok {
		return c
	}

	return nil
}

func listCollections(collections map[string]*collection.Collection) interface{} {
	return func() []string {
		result := []string{}

		for k, _ := range collections {
			result = append(result, k)
		}

		return result
	}
}

type createCollectionRequest struct {
	Name string `json:"name"`
}

func createCollection(collections map[string]*collection.Collection, dir string) interface{} {
	return func(input *createCollectionRequest) (*createCollectionRequest, error) {

		filename := path.Join(dir, input.Name)

		col, err := collection.OpenCollection(filename)
		if err != nil {
			return nil, err
		}

		collections[input.Name] = col

		return input, nil
	}
}

func listItems(collections map[string]*collection.Collection) interface{} {
	return func(ctx context.Context, w http.ResponseWriter) {

		name := getBoxContext(ctx).Parameters["collection_name"]
		collections[name].Traverse(func(data []byte) {
			w.Write(data)
			w.Write([]byte("\n"))
		})

	}
}

func countItems(collections map[string]*collection.Collection) interface{} {
	return func(ctx context.Context) interface{} {

		name := getBoxContext(ctx).Parameters["collection_name"]

		return map[string]interface{}{
			"count": len(collections[name].Rows),
		}
	}
}

func insertItem(collections map[string]*collection.Collection) interface{} {

	type Item map[string]interface{}

	return func(ctx context.Context, item Item) (Item, error) {

		name := getBoxContext(ctx).Parameters["collection_name"]

		err := collections[name].Insert(item)
		if err != nil {
			return nil, err
		}

		return item, nil
	}
}

func listIndexes(collections map[string]*collection.Collection) interface{} {
	return func(ctx context.Context) []string {

		result := []string{}

		name := getBoxContext(ctx).Parameters["collection_name"]
		for name, _ := range collections[name].Indexes {
			result = append(result, name)
		}

		return result
	}
}

func createIndex(collections map[string]*collection.Collection) interface{} {

	return func(ctx context.Context, indexOptions *collection.IndexOptions) (*collection.IndexOptions, error) {

		name := getBoxContext(ctx).Parameters["collection_name"]
		err := collections[name].Index(indexOptions)
		if err != nil {
			return nil, err
		}

		return indexOptions, nil
	}
}

func getIndex(collections map[string]*collection.Collection) interface{} {

	return func(ctx context.Context) (*collection.IndexOptions, error) {

		collectionName := getBoxContext(ctx).Parameters["collection_name"]
		indexName := getBoxContext(ctx).Parameters["index_name"]
		_, exists := collections[collectionName].Indexes[indexName]
		if !exists {
			return nil, fmt.Errorf("index '%s' does not exist", indexName)
		}

		return &collection.IndexOptions{
			Field: indexName,
		}, nil
	}
}

func indexFindBy(collections map[string]*collection.Collection) interface{} {

	return func(ctx context.Context) (interface{}, error) {

		collectionName := getBoxContext(ctx).Parameters["collection_name"]
		indexName := getBoxContext(ctx).Parameters["index_name"]
		value := getBoxContext(ctx).Parameters["value"]
		result := map[string]interface{}{}
		err := collections[collectionName].FindBy(indexName, value, &result)
		if err != nil {
			return nil, fmt.Errorf("item %s='%s' does not exist", indexName, value)
		}

		return result, nil
	}
}
