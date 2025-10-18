package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/fulldump/goconfig"

	"github.com/fulldump/inceptiondb/bootstrap"
	"github.com/fulldump/inceptiondb/configuration"
)

var banner = `
 _____                     _   _            ____________ 
|_   _|                   | | (_)           |  _  \ ___ \
  | | _ __   ___ ___ _ __ | |_ _  ___  _ __ | | | | |_/ /
  | || '_ \ / __/ _ \ '_ \| __| |/ _ \| '_ \| | | | ___ \
 _| || | | | (_|  __/ |_) | |_| | (_) | | | | |/ /| |_/ /
 \___/_| |_|\___\___| .__/ \__|_|\___/|_| |_|___/ \____/ 
                    | |                                  
                    |_|          version ` + bootstrap.VERSION + `
`

func main() {

	c := configuration.Default()
	goconfig.Read(&c)

	if c.Version {
		fmt.Println("Version:", bootstrap.VERSION)
		return
	}

	if c.ShowBanner {
		fmt.Println(banner)
	}

	if c.ShowConfig {
		e := json.NewEncoder(os.Stdout)
		e.SetIndent("", "    ")
		e.Encode(c)
	}

	start, _ := bootstrap.Bootstrap(c)
	start()
}
