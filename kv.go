package main

import (
	"context"
	"time"
	"errors"
	"sync"

	"github.com/couchbase/gocb/v2"
	wrpc "wrpc.io/go"
	sdk "go.wasmcloud.dev/provider"
	"go.opentelemetry.io/otel"
	wrpcnats "wrpc.io/go/nats"

	// Generated bindings
	"github.com/couchbase-examples/wasmcloud-provider-couchbase/bindings/exports/wrpc/keyvalue/atomics"
	"github.com/couchbase-examples/wasmcloud-provider-couchbase/bindings/exports/wrpc/keyvalue/store"
	gocbt "github.com/couchbase/gocb-opentelemetry"
	"go.opentelemetry.io/otel/trace"
)

var (
	errNoSuchStore     = store.NewErrorNoSuchStore()
	errInvalidDataType = store.NewErrorOther("invalid data type stored in map")
	tracer             trace.Tracer
)

type KVHandler struct {
	// The provider instance
	*sdk.WasmcloudProvider

	linksMutex *sync.RWMutex
	linkedFrom map[string]map[string]string

	connectionsMutex *sync.RWMutex
	clusterConnections map[string]*gocb.Collection
}

func (h *KVHandler) Get(ctx context.Context, bucket string, key string) (*wrpc.Result[[]uint8, store.Error], error) {
	ctx = extractTraceHeaderContext(ctx)
	ctx, span := tracer.Start(ctx, "GET")
	defer span.End()

	h.Logger.Debug("received request to get value", "key", key)
	collection, err := h.getCollectionFromContext(ctx)
	if err != nil {
		h.Logger.Error("unable to get collection from context", "error", err)
		return wrpc.Err[[]uint8](*errNoSuchStore), err
	}

	res, err := collection.Get(key, &gocb.GetOptions{
		Transcoder: gocb.NewRawJSONTranscoder(),
		ParentSpan: gocbt.NewOpenTelemetryRequestSpan(ctx, span),
	})
	if err != nil {
		h.Logger.Error("unable to get value in store", "key", key, "error", err)
		return wrpc.Err[[]uint8](*errNoSuchStore), err
	}

	var response []uint8
	err = res.Content(&response)
	if err != nil {
		h.Logger.Error("unable to decode content as bytes", "key", key, "error", err)
		return wrpc.Err[[]uint8](*errInvalidDataType), err
	}
	return wrpc.Ok[store.Error](response), nil
}

func (h *KVHandler) Set(ctx context.Context, bucket string, key string, value []uint8) (*wrpc.Result[struct{}, store.Error], error) {
	ctx = extractTraceHeaderContext(ctx)
	ctx, span := tracer.Start(ctx, "SET")
	defer span.End()

	h.Logger.Debug("received request to set value", "key", key)
	collection, err := h.getCollectionFromContext(ctx)
	if err != nil {
		h.Logger.Error("unable to get collection from context", "error", err)
		return wrpc.Err[struct{}](*errNoSuchStore), err
	}

	_, err = collection.Upsert(key, &value, &gocb.UpsertOptions{
		Transcoder: gocb.NewRawJSONTranscoder(),
		ParentSpan: gocbt.NewOpenTelemetryRequestSpan(ctx, span),
	})
	if err != nil {
		h.Logger.Error("unable to store value", "key", key, "error", err)
		return wrpc.Err[struct{}](*errInvalidDataType), err
	}
	return wrpc.Ok[store.Error](struct{}{}), nil
}

func (h *KVHandler) Delete(ctx context.Context, bucket string, key string) (*wrpc.Result[struct{}, store.Error], error) {
	ctx = extractTraceHeaderContext(ctx)
	ctx, span := tracer.Start(ctx, "DELETE")
	defer span.End()

	h.Logger.Debug("received request to delete value", "key", key)
	collection, err := h.getCollectionFromContext(ctx)
	if err != nil {
		h.Logger.Error("unable to get collection from context", "error", err)
		return wrpc.Err[struct{}](*errNoSuchStore), err
	}

	_, err = collection.Remove(key, &gocb.RemoveOptions{
		ParentSpan: gocbt.NewOpenTelemetryRequestSpan(ctx, span),
	})
	if err != nil {
		h.Logger.Error("unable to remove value", "key", key, "error", err)
		return wrpc.Err[struct{}](*errNoSuchStore), err
	}
	return wrpc.Ok[store.Error](struct{}{}), nil
}

func (h *KVHandler) Exists(ctx context.Context, bucket string, key string) (*wrpc.Result[bool, store.Error], error) {
	ctx = extractTraceHeaderContext(ctx)
	ctx, span := tracer.Start(ctx, "EXISTS")
	defer span.End()

	h.Logger.Debug("received request to check value existence", "key", key)
	collection, err := h.getCollectionFromContext(ctx)
	if err != nil {
		h.Logger.Error("unable to get collection from context", "error", err)
		return wrpc.Err[bool](*errNoSuchStore), err
	}

	res, err := collection.Exists(key, &gocb.ExistsOptions{
		ParentSpan: gocbt.NewOpenTelemetryRequestSpan(ctx, span),
	})
	if err != nil {
		h.Logger.Error("unable to check existence of value", "key", key, "error", err)
		return wrpc.Err[bool](*errNoSuchStore), err
	}
	return wrpc.Ok[store.Error](res.Exists()), nil
}

func (h *KVHandler) ListKeys(ctx context.Context, bucket string, cursor *uint64) (*wrpc.Result[store.KeyResponse, store.Error], error) {
	h.Logger.Warn("received request to list keys")
	return wrpc.Err[store.KeyResponse](*store.NewErrorOther("list-keys operation not supported")), nil
}

// Implementation of wasi:keyvalue/atomics
func (h *KVHandler) Increment(ctx context.Context, bucket string, key string, delta uint64) (*wrpc.Result[uint64, atomics.Error], error) {
	ctx = extractTraceHeaderContext(ctx)
	ctx, span := tracer.Start(ctx, "INCREMENT")
	defer span.End()

	h.Logger.Debug("received request to increment key by delta", "key", key, "delta", delta)
	collection, err := h.getCollectionFromContext(ctx)
	if err != nil {
		h.Logger.Error("unable to get collection from context", "error", err)
		return wrpc.Err[uint64](*errNoSuchStore), err
	}

	res, err := collection.Binary().Increment(key, &gocb.IncrementOptions{
		Initial:    int64(delta),
		Delta:      delta,
		ParentSpan: gocbt.NewOpenTelemetryRequestSpan(ctx, span),
	})
	if err != nil {
		h.Logger.Error("unable to increment value at key", "key", key, "error", err)
		return wrpc.Err[uint64](*errInvalidDataType), err
	}

	return wrpc.Ok[atomics.Error](res.Content()), nil
}

// Provider handler functions
func (h *KVHandler) handleNewTargetLink(link sdk.InterfaceLinkDefinition) error {
	h.linksMutex.Lock()
	defer h.linksMutex.Unlock()

	h.Logger.Info("Handling new target link", "link", link)
	h.linkedFrom[link.SourceID] = link.TargetConfig
	couchbaseConnectionArgs, err := validateCouchbaseConfig(link.TargetConfig, link.TargetSecrets)
	if err != nil {
		h.Logger.Error("Invalid couchbase target config", "error", err)
		return err
	}
	h.updateCouchbaseCluster(link.SourceID, couchbaseConnectionArgs)
	return nil
}

func (h *KVHandler) updateCouchbaseCluster(sourceId string, connectionArgs CouchbaseConnectionArgs) {
	h.connectionsMutex.Lock()
	defer h.connectionsMutex.Unlock()

	// Connect to the cluster
	cluster, err := gocb.Connect(connectionArgs.ConnectionString, gocb.ClusterOptions{
		Username: connectionArgs.Username,
		Password: connectionArgs.Password,
		Tracer:   gocbt.NewOpenTelemetryRequestTracer(otel.GetTracerProvider()),
	})
	if err != nil {
		h.Logger.Error("unable to connect to couchbase cluster", "error", err)
		return
	}
	var collection *gocb.Collection
	if connectionArgs.CollectionName != "" && connectionArgs.ScopeName != "" {
		collection = cluster.Bucket(connectionArgs.BucketName).Scope(connectionArgs.ScopeName).Collection(connectionArgs.CollectionName)
	} else {
		collection = cluster.Bucket(connectionArgs.BucketName).DefaultCollection()
	}

	bucket := cluster.Bucket(connectionArgs.BucketName)
	if err = bucket.WaitUntilReady(5*time.Second, nil); err != nil {
		h.Logger.Error("unable to connect to couchbase bucket", "error", err)
	}

	// Store the connection
	h.clusterConnections[sourceId] = collection
}

func (h *KVHandler) handleDelTargetLink(link sdk.InterfaceLinkDefinition) error {
	h.linksMutex.Lock()
	defer h.linksMutex.Unlock()

	h.Logger.Info("Handling del target link", "link", link)
	delete(h.linkedFrom, link.Target)
	return nil
}

// Helper function to get the correct collection from the invocation context
func (h *KVHandler) getCollectionFromContext(ctx context.Context) (*gocb.Collection, error) {
	h.linksMutex.Lock()
	h.connectionsMutex.Lock()
	defer h.connectionsMutex.Unlock()
	defer h.linksMutex.Unlock()

	header, ok := wrpcnats.HeaderFromContext(ctx)
	if !ok {
		h.Logger.Warn("Received request from unknown origin")
		return nil, errors.New("error fetching header from wrpc context")
	}
	// Only allow requests from a linked component
	sourceId := header.Get("source-id")
	if h.linkedFrom[sourceId] == nil {
		h.Logger.Warn("Received request from unlinked source", "sourceId", sourceId)
		return nil, errors.New("received request from unlinked source")
	}
	return h.clusterConnections[sourceId], nil
}
