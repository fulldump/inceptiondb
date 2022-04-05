package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"inceptiondb/api"
	"inceptiondb/collection"
	"inceptiondb/configuration"
)

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

	c := configuration.Default()

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

	api.Build(collections, c.Dir).Serve()
}
