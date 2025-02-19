package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"

	"github.com/negrel/conc"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	for {
		conc.Block(func(n conc.Nursery) error {
			for i := 0; i < 1000; i++ {
				n.Go(func() error {
					return nil
				})
			}

			return nil
		})
	}
}
