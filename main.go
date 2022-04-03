package main

import (
	"context"
	"fmt"
	"inceptiondb/collection"
	"io/fs"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"github.com/fulldump/box"
)

type Configuration struct {
	Dir string `usage:"data directory"`
}

func main() {

	c := &Configuration{
		Dir: "data",
	}

	collections := map[string]*collection.Collection{}

	filepath.WalkDir(c.Dir, func(filename string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		name := filename
		name = strings.TrimPrefix(name, c.Dir)
		name = strings.TrimPrefix(name, "/")

		col, err := collection.OpenCollection(filename)
		if err != nil {
			fmt.Printf("open collection '%s': %s\n", filename, err.Error())
		}

		collections[name] = col

		return nil
	})

	fmt.Println(collections)

	b := box.NewBox()
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

	b.Serve()
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
