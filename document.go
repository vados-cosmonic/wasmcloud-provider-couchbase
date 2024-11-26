package main

import (
	"context"
	"time"
	"errors"
	"sync"

	"github.com/couchbase/gocb/v2"
	wrpc "wrpc.io/go"
	wrpcnats "wrpc.io/go/nats"
	gocbt "github.com/couchbase/gocb-opentelemetry"
	"go.opentelemetry.io/otel"

	sdk "go.wasmcloud.dev/provider"

	// Generated bindings
	"github.com/couchbase-examples/wasmcloud-provider-couchbase/bindings/exports/wasmcloud/couchbase/document"
	"github.com/couchbase-examples/wasmcloud-provider-couchbase/bindings/wasmcloud/couchbase/types"
)

func Ok[T any](v T) *wrpc.Result[T, types.DocumentError] {
	return wrpc.Ok[types.DocumentError](v)
}

func Err[T any](e types.DocumentError) *wrpc.Result[T, types.DocumentError] {
	return wrpc.Err[T](e)
}

type DocumentHandler struct {
	// The provider instance
	*sdk.WasmcloudProvider

	linksMutex *sync.RWMutex
	linkedFrom map[string]map[string]string

	connectionsMutex *sync.RWMutex
	clusterConnections map[string]*gocb.Collection
}

func (h *DocumentHandler) Get(ctx context.Context, id string, options *document.DocumentGetOptions) (*wrpc.Result[document.DocumentGetResult, types.DocumentError], error) {
	collection, err := h.getCollectionFromContext(ctx)
	if err != nil {
		h.Logger.Error("Error fetching collection from context", "error", err)
		return nil, err
	}
	couchbaseResult, err := collection.Get(id, GetOptions(options))
	if err != nil {
		h.Logger.Error("Error getting document", "error", err)
		return Err[document.DocumentGetResult](*types.NewDocumentErrorNotFound()), nil
	}
	documentResult, err := GetResult(couchbaseResult)
	if err != nil {
		h.Logger.Error("Error getting document result", "error", err)
		return Err[document.DocumentGetResult](*types.NewDocumentErrorNotJson()), nil
	}
	return Ok(documentResult), nil
}

// GetAllReplicas implements document.Handler.
func (h *DocumentHandler) GetAllReplicas(ctx context.Context, id string, options *document.DocumentGetAllReplicaOptions) (*wrpc.Result[[]*document.DocumentGetReplicaResult, types.DocumentError], error) {
	collection, err := h.getCollectionFromContext(ctx)
	if err != nil {
		h.Logger.Error("Error fetching collection from context", "error", err)
		return nil, err
	}
	res, err := collection.GetAllReplicas(id, GetAllReplicaOptions(options))
	if err != nil {
		h.Logger.Error("Error fetching all replicas", "error", err)
		return nil, err
	}
	return Ok(GetAllReplicasResult(res)), nil
}

// GetAndLock implements document.Handler.
func (h *DocumentHandler) GetAndLock(ctx context.Context, id string, options *document.DocumentGetAndLockOptions) (*wrpc.Result[document.DocumentGetResult, types.DocumentError], error) {
	collection, err := h.getCollectionFromContext(ctx)
	if err != nil {
		h.Logger.Error("Error fetching collection from context", "error", err)
		return nil, err
	}
	couchbaseResult, err := collection.GetAndLock(id, time.Duration(options.LockTime), GetAndLockOptions(options))
	if err != nil {
		h.Logger.Error("Error getting and locking document", "error", err)
		return nil, err
	}
	documentResult, err := GetResult(couchbaseResult)
	if err != nil {
		h.Logger.Error("Error getting document result", "error", err)
		return nil, err
	}
	return Ok(documentResult), nil
}

// GetAndTouch implements document.Handler.
func (h *DocumentHandler) GetAndTouch(ctx context.Context, id string, options *document.DocumentGetAndTouchOptions) (*wrpc.Result[document.DocumentGetResult, types.DocumentError], error) {
	collection, err := h.getCollectionFromContext(ctx)
	if err != nil {
		h.Logger.Error("Error fetching collection from context", "error", err)
		return nil, err
	}
	couchbaseResult, err := collection.GetAndTouch(id, time.Duration(options.ExpiresIn), GetAndTouchOptions(options))
	if err != nil {
		h.Logger.Error("Error getting and touching document", "error", err)
		return nil, err
	}
	documentResult, err := GetResult(couchbaseResult)
	if err != nil {
		h.Logger.Error("Error getting document result", "error", err)
		return nil, err
	}
	return Ok(documentResult), nil
}

// GetAnyRepliacs implements document.Handler.
func (h *DocumentHandler) GetAnyRepliacs(ctx context.Context, id string, options *document.DocumentGetAnyReplicaOptions) (*wrpc.Result[document.DocumentGetReplicaResult, types.DocumentError], error) {
	collection, err := h.getCollectionFromContext(ctx)
	if err != nil {
		h.Logger.Error("Error fetching collection from context", "error", err)
		return nil, err
	}
	result, err := collection.GetAnyReplica(id, GetAnyReplicaOptions(options))
	if err != nil {
		h.Logger.Error("Error getting any replica", "error", err)
		return nil, err
	}
	return Ok(GetReplicaResult(result)), nil
}

// Insert implements document.Handler.
func (h *DocumentHandler) Insert(ctx context.Context, id string, doc *types.Document, options *document.DocumentInsertOptions) (*wrpc.Result[types.MutationMetadata, types.DocumentError], error) {
	collection, err := h.getCollectionFromContext(ctx)
	if err != nil {
		h.Logger.Error("Error fetching collection from context", "error", err)
		return nil, err
	}
	docToInsert, ok := doc.GetRaw()
	if !ok {
		h.Logger.Error("Error getting raw document", "doc", doc)
		return Err[types.MutationMetadata](*types.NewDocumentErrorNotJson()), nil
	}
	result, err := collection.Insert(id, docToInsert, InsertOptions(options))
	if err != nil {
		h.Logger.Error("Error inserting document", "error", err)
		return Err[types.MutationMetadata](*types.NewDocumentErrorInvalidValue()), err
	}
	return Ok(MutationMetadata(result)), nil
}

// Remove implements document.Handler.
func (h *DocumentHandler) Remove(ctx context.Context, id string, options *document.DocumentRemoveOptions) (*wrpc.Result[types.MutationMetadata, types.DocumentError], error) {
	collection, err := h.getCollectionFromContext(ctx)
	if err != nil {
		h.Logger.Error("Error fetching collection from context", "error", err)
		return nil, err
	}
	result, err := collection.Remove(id, RemoveOptions(options))
	if err != nil {
		h.Logger.Error("Error removing document", "error", err)
		return nil, err
	}
	return Ok(MutationMetadata(result)), nil
}

// Replace implements document.Handler.
func (h *DocumentHandler) Replace(ctx context.Context, id string, doc *types.Document, options *document.DocumentReplaceOptions) (*wrpc.Result[types.MutationMetadata, types.DocumentError], error) {
	collection, err := h.getCollectionFromContext(ctx)
	if err != nil {
		h.Logger.Error("Error fetching collection from context", "error", err)
		return nil, err
	}
	result, err := collection.Replace(id, doc, ReplaceOptions(options))
	if err != nil {
		h.Logger.Error("Error replacing document", "error", err)
		return nil, err
	}
	return Ok(MutationMetadata(result)), nil
}

// Touch implements document.Handler.
func (h *DocumentHandler) Touch(ctx context.Context, id string, options *document.DocumentTouchOptions) (*wrpc.Result[types.MutationMetadata, types.DocumentError], error) {
	collection, err := h.getCollectionFromContext(ctx)
	if err != nil {
		h.Logger.Error("Error fetching collection from context", "error", err)
		return nil, err
	}
	result, err := collection.Touch(id, time.Duration(options.ExpiresIn), TouchOptions(options))
	if err != nil {
		h.Logger.Error("Error touching document", "error", err)
		return nil, err
	}
	return Ok(MutationMetadata(result)), nil
}

// Unlock implements document.Handler.
func (h *DocumentHandler) Unlock(ctx context.Context, id string, options *document.DocumentUnlockOptions) (*wrpc.Result[struct{}, types.DocumentError], error) {
	collection, err := h.getCollectionFromContext(ctx)
	if err != nil {
		h.Logger.Error("Error fetching collection from context", "error", err)
		return nil, err
	}
	err = collection.Unlock(id, gocb.Cas(options.Cas), UnlockOptions(options))
	if err != nil {
		h.Logger.Error("Error unlocking document", "error", err)
		return nil, err
	}
	return Ok(struct{}{}), nil
}

// Upsert implements document.Handler.
func (h *DocumentHandler) Upsert(ctx context.Context, id string, doc *types.Document, options *document.DocumentUpsertOptions) (*wrpc.Result[types.MutationMetadata, types.DocumentError], error) {
	collection, err := h.getCollectionFromContext(ctx)
	if err != nil {
		h.Logger.Error("Error fetching collection from context", "error", err)
		return nil, err
	}
	raw, ok := doc.GetRaw()
	if !ok {
		h.Logger.Error("Error getting raw document", "doc", doc)
		return Err[types.MutationMetadata](*types.NewDocumentErrorNotJson()), nil
	}
	result, err := collection.Upsert(id, raw, UpsertOptions(options))
	if err != nil {
		h.Logger.Error("Error upserting document", "error", err)
		return Err[types.MutationMetadata](*types.NewDocumentErrorNotJson()), nil
	}
	return Ok(MutationMetadata(result)), nil
}

// Helper function to get the correct collection from the invocation context
func (h *DocumentHandler) getCollectionFromContext(ctx context.Context) (*gocb.Collection, error) {
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

// Provider handler functions
func (h *DocumentHandler) handleNewTargetLink(link sdk.InterfaceLinkDefinition) error {
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

func (h *DocumentHandler) updateCouchbaseCluster(sourceId string, connectionArgs CouchbaseConnectionArgs) {
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

func (h *DocumentHandler) handleDelTargetLink(link sdk.InterfaceLinkDefinition) error {
	h.Logger.Info("Handling del target link", "link", link)
	delete(h.linkedFrom, link.Target)
	return nil
}

func (h *DocumentHandler) handleHealthCheck() string {
	h.Logger.Debug("Handling health check")
	return "provider healthy"
}

func (h *DocumentHandler) handleShutdown() error {
	h.Logger.Info("Handling shutdown")
	// clear(handler.linkedFrom)
	return nil
}
