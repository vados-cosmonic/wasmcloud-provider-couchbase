//go:generate wit-bindgen-wrpc go --out-dir bindings --package github.com/couchbase-examples/wasmcloud-provider-couchbase/bindings wit

package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	wrpc "github.com/couchbase-examples/wasmcloud-provider-couchbase/bindings"
	"github.com/couchbase/gocb/v2"
	wasmcloudprovider "go.wasmcloud.dev/provider"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// Handle SIGINT (CTRL+C) gracefully.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Set up OpenTelemetry.
	otelShutdown, err := setupOTelSDK(ctx)
	if err != nil {
		return err
	}
	// Handle shutdown properly so nothing leaks.
	defer func() {
		err = errors.Join(err, otelShutdown(context.Background()))
	}()

	// Initialize the combined handler
	provider, err := NewProvider()
	if err != nil {
		return err
	}

	// Set up the provider as a wasmCloud provider
	wp, err := wasmcloudprovider.New(
		wasmcloudprovider.TargetLinkPut(provider.handleNewTargetLink),
		wasmcloudprovider.TargetLinkDel(provider.handleDelTargetLink),
		wasmcloudprovider.HealthCheck(provider.handleHealthCheck),
		wasmcloudprovider.Shutdown(provider.handleShutdown),
	)
	if err != nil {
		return err
	}

	// Store the provider for use in the handlers
	provider.WasmcloudProvider = wp

	// Setup two channels to await RPC and control interface operations
	providerCh := make(chan error, 1)
	signalCh := make(chan os.Signal, 1)

	// Handle RPC operations
	stopFunc, err := wrpc.Serve(wp.RPCClient, &provider, &provider, &provider)
	if err != nil {
		wp.Shutdown()
		return err
	}

	// Handle control interface operations
	go func() {
		err := wp.Start()
		providerCh <- err
	}()

	// Shutdown on SIGINT
	signal.Notify(signalCh, syscall.SIGINT)

	// Run provider until either a shutdown is requested or a SIGINT is received
	select {
	case err = <-providerCh:
		stopFunc()
		return err
	case <-signalCh:
		wp.Shutdown()
		stopFunc()
	}
	return nil
}
