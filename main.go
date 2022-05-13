package main

import (
	"context"
	"fmt"
	"github.com/fulldump/box"
	"inceptiondb/api"
	"inceptiondb/configuration"
	"inceptiondb/database"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
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

	fmt.Println(banner)

	c := configuration.Default()
	d := database.NewDatabase(c)
	b := api.Build(d, c.Dir, c.Statics)
	s := &http.Server{
		Addr:    c.HttpAddr,
		Handler: box.Box2Http(b),
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		for {
			sig := <-signalChan
			fmt.Println("Signal received", sig.String())
			d.Stop()
			s.Shutdown(context.Background())
		}
	}()

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := d.Start()
		if err != nil {
			fmt.Println(err.Error())
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Println("listening on", s.Addr)
		err := s.ListenAndServe()
		if err != nil {
			fmt.Println(err.Error())
		}
	}()

	wg.Wait()
}
