package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/teslapatrick/openai-goproxy/node"
)

func main() {
	args := os.Args
	if len(args) != 3 {
		fmt.Println("Usage: ", args[0], " -c <config file>")
		os.Exit(1)
	}
	// read config file
	filepath := args[2]
	config, err := node.ReadConfig(filepath)
	if err != nil {
		log.Fatal(err)
	}

	// set thread
	runtime.GOMAXPROCS(config.Sys.Threads)
	// check apikey
	if !config.API.UseProxy && config.API.Apikey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is not set when you are not using proxy.")
	}

	// new node
	node := node.New(config)
	// add ctrl+c for exit
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		node.Stop()
	}()
	// run the node
	go node.Run()
	// wait for exit
	node.Wait()
}
