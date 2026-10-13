package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/XB-Dong/classroom-annotator/backend/internal/mediaagent"
)

var version = "dev"

func main() {
	cfg, addr, err := mediaagent.LoadConfig()
	if err != nil {
		log.Fatal("media agent configuration rejected")
	}
	cfg.AgentVersion = version
	agent, err := mediaagent.New(cfg)
	if err != nil {
		log.Fatal("media agent initialization failed")
	}
	defer agent.Close()
	server := &http.Server{Addr: addr, Handler: agent, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 600 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)
	go func() {
		<-stop
		agent.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	log.Print("media agent starting on configured loopback address")
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("media agent listener failed")
	}
}
