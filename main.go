package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"codeberg.org/apollo7321/nebula-drive/gui"
	"codeberg.org/apollo7321/nebula-drive/lib"
)

func main() {
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigint
		cancel()
	}()

	client, err := lib.NewS3Client(rootCtx, 10*time.Second)
	if err != nil {
		panic(err)
	}
	if err = gui.NewGui(client); err != nil {
		panic(err)
	}
}
