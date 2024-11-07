package main

import (
	"errors"

	"go.wasmcloud.dev/provider"
)

type CouchbaseConnectionArgs struct {
	Username         string
	Password         string
	BucketName       string
	ConnectionString string
	ScopeName        string
	CollectionName   string
}

// Construct Couchbase connection args from config and secrets
func validateCouchbaseConfig(config map[string]string, secrets map[string]provider.SecretValue) (CouchbaseConnectionArgs, error) {
	connectionArgs := CouchbaseConnectionArgs{}
	if username, ok := config["username"]; !ok || username == "" {
		return connectionArgs, errors.New("username config is required")
	} else {
		connectionArgs.Username = username
	}
	if bucketName, ok := config["bucketName"]; !ok || bucketName == "" {
		return connectionArgs, errors.New("bucketName config is required")
	} else {
		connectionArgs.BucketName = bucketName
	}
	if connectionString, ok := config["connectionString"]; !ok || connectionString == "" {
		return connectionArgs, errors.New("connectionString config is required")
	} else {
		connectionArgs.ConnectionString = connectionString
	}

	// NOTE: normaly we'd use the provided secrets for passwords, but to reduce
	// complexity for the demo, we'll use a configured password secret, if 'allow_unsafe_password'
	// is present in config
	if _, exists := config["allow_unsafe_password"]; exists {
		if password, ok := config["password"]; !ok || password == "" {
			return connectionArgs, errors.New("password config is required")
		} else {
			connectionArgs.Password = password
		}
	} else {
		password := secrets["password"].String.Reveal()
		if password == "" {
			return connectionArgs, errors.New("password secret is required")
		} else {
			connectionArgs.Password = password
		}
	}

	return connectionArgs, nil
}
