package main

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/fulldump/box"
)

type Configuration struct {
	Dir string `usage:"data directory"`
}

func main() {

	c := &Configuration{
		Dir: "data",
	}

	collections := map[string][]interface{}{}

	filepath.WalkDir(c.Dir, func(path string, d fs.DirEntry, err error) error {
		fmt.Println(d)
		return nil
	})

	fmt.Println(collections)

	b := box.NewBox()
	b.Resource("collections")

	b.Serve()
}
