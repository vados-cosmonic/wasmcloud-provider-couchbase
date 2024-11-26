package main

import (
	"errors"
	"fmt"
	"sync"

	"github.com/couchbase/gocb/v2"
	sdk "go.wasmcloud.dev/provider"
)

const TRACER_NAME = "wasmcloud-provider-couchbase"

type combinedProvider struct {
	// The provider instance
	*sdk.WasmcloudProvider

	// All components linked to this provider and their config.
	linkedFrom map[string]map[string]string
	linksMutex sync.RWMutex

	// map that stores couchbase cluster connections
	clusterConnections map[string]*gocb.Collection
	connectionsMutex   sync.RWMutex

	// Embed functionality for both document handler and KV
	DocumentHandler
	KVHandler
}

// Create a new Provider
func NewProvider() (combinedProvider, error) {
	linkedFrom := make(map[string]map[string]string)
	var linksMutex sync.RWMutex
	clusterConnections := make(map[string]*gocb.Collection)
	var connectionsMutex sync.RWMutex

	kv := KVHandler{
		linksMutex:         &linksMutex,
		linkedFrom:         linkedFrom,
		connectionsMutex:   &connectionsMutex,
		clusterConnections: clusterConnections,
	}

	doc := DocumentHandler{
		linksMutex:         &linksMutex,
		linkedFrom:         linkedFrom,
		connectionsMutex:   &connectionsMutex,
		clusterConnections: clusterConnections,
	}

	return combinedProvider{
		nil,
		linkedFrom,
		linksMutex,
		clusterConnections,
		connectionsMutex,
		kv,
		doc,
	}, nil
}

// Handle a new targe tlink
func (h *combinedProvider) handleNewTargetLink(link sdk.InterfaceLinkDefinition) error {
	switch link.WitNamespace {
	case "wasi":
		switch link.WitPackage {
		case "keyvalue":
			return h.KVHandler.handleNewTargetLink(link)
		default:
			h.Logger.Error("unexpected WIT package", "package", link.WitPackage)
			return errors.New(fmt.Sprintf("unexpected WIT package [%s]", link.WitPackage))
		}
	case "wasmcloud":
		switch link.WitPackage {
		case "couchbase":
			return h.DocumentHandler.handleNewTargetLink(link)
		default:
			h.Logger.Error("unexpected WIT package", "package", link.WitPackage)
			return errors.New(fmt.Sprintf("unexpected WIT package [%s]", link.WitPackage))
		}
	default:
		h.Logger.Error("unexpected WIT namespace", "namespace", link.WitNamespace)
		return errors.New(fmt.Sprintf("unexpected WIT namespace [%s]", link.WitNamespace))
	}
}

func (h *combinedProvider) handleDelTargetLink(link sdk.InterfaceLinkDefinition) error {
	switch link.WitNamespace {
	case "wasi":
		switch link.WitPackage {
		case "keyvalue":
			return h.KVHandler.handleDelTargetLink(link)
		default:
			h.Logger.Error("unexpected WIT package", "package", link.WitPackage)
			return errors.New(fmt.Sprintf("unexpected WIT package [%s]", link.WitPackage))
		}
	case "wasmcloud":
		switch link.WitPackage {
		case "couchbase":
			return h.DocumentHandler.handleDelTargetLink(link)
		default:
			h.Logger.Error("unexpected WIT package", "package", link.WitPackage)
			return errors.New(fmt.Sprintf("unexpected WIT package [%s]", link.WitPackage))
		}
	default:
		h.Logger.Error("unexpected WIT namespace", "namespace", link.WitNamespace)
		return errors.New(fmt.Sprintf("unexpected WIT namespace [%s]", link.WitNamespace))
	}
}

func (h *combinedProvider) handleHealthCheck() string {
	h.Logger.Debug("Handling health check")
	return "provider healthy"
}

func (h *combinedProvider) handleShutdown() error {
	h.linksMutex.Lock()
	defer h.linksMutex.Unlock()

	h.Logger.Info("Handling shutdown")
	clear(h.linkedFrom)
	return nil
}
