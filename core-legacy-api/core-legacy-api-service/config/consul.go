package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/Netcracker/qubership-mini-core/core-legacy-api/core-legacy-api-service/model"

	"github.com/google/uuid"
	"github.com/hashicorp/consul/api"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
)

var logger = logging.GetLogger("ConsulService")

const consulPropertiesSource = "consul"
const consulConfigPrefix = "config"
const consulGlobalApplicationName = "application"
const configPropertiesGlobalApplicationName = "global"
const defaultProfileName = "default"
const consulTxOperationLimit = 64
const txnMaxReqLen = 512 * 1024

func NewConsulService(client *api.Client, namespace string) ConfigService {
	return &consulService{client, namespace}
}

func (s *consulService) AddProperties(ctx context.Context, application string, profile string, properties map[string]string) error {
	batcher := newTxnBatcher(ctx, s.performTransaction)

	for key, value := range properties {
		kvKey := formatKeyPrefix(s.namespace, application, profile) + strings.ReplaceAll(escape(key), ".", "/")
		op := &api.KVTxnOp{Verb: api.KVSet, Key: kvKey, Value: []byte(value)}

		if err := batcher.add(op, calculateOperationSize(kvKey, value)); err != nil {
			return err
		}
	}

	return batcher.done()
}

func (s *consulService) performTransaction(ctx context.Context, operation api.TxnOps) error {
	logger.InfoC(ctx, "Executing Consul transaction with %d operations", len(operation))
	queryOptions := (&api.QueryOptions{}).WithContext(ctx)
	ok, resp, _, err := s.consul.Txn().Txn(operation, queryOptions) //TODO: transaction?
	if err != nil {
		if ctx.Err() != nil {
			logger.ErrorC(ctx, "Consul transaction cancelled: %v", ctx.Err())
			return ctx.Err()
		}
		logger.ErrorC(ctx, "Failed to execute Consul transaction: %v", err.Error())
		return ErrConsul{
			Message: err.Error(),
		}
	}

	if !ok {
		logger.ErrorC(ctx, "Consul transaction was rejected")

		var reasons []string
		if resp != nil {
			for _, e := range resp.Errors {
				logger.ErrorC(ctx, "Transaction error: OpIndex=%d, What=%s", e.OpIndex, e.What)
				reasons = append(reasons, fmt.Sprintf("OpIndex=%d, What=%s", e.OpIndex, e.What))
			}
		}

		return ErrConsul{
			Message: strings.Join(reasons, "\n"),
		}
	}

	logger.InfoC(ctx, "Consul transaction completed successfully")

	return nil
}

func (s *consulService) FindAll(ctx context.Context) ([]model.ConfigProfile, error) {
	prefix := consulConfigPrefix + "/" + s.namespace
	queryOptions := (&api.QueryOptions{}).WithContext(ctx)
	// Get all KV pairs under the prefix.
	pairs, _, err := s.consul.KV().List(prefix, queryOptions)
	if err != nil {
		logger.ErrorC(ctx, "Failed to list KV pairs under prefix=%s: %v", prefix, err)
		return nil, err
	}

	propertiesByApp := groupPropertiesByApplication(pairs)

	profiles := buildConfigProfiles(propertiesByApp, prefix)

	logger.Infof("Found %d config profiles", len(profiles))

	return profiles, nil

}

func (s *consulService) FindByApplicationAndProfile(
	ctx context.Context,
	appName string,
	profileName string,
) (model.ConfigProfile, error) {
	prefix := formatKeyPrefix(s.namespace, appName, profileName)
	queryOptions := (&api.QueryOptions{}).WithContext(ctx)
	properties, _, err := s.consul.KV().List(prefix, queryOptions)
	if err != nil {
		logger.ErrorC(ctx, "Failed to list KV pairs for application=%s, profile=%s: %v", appName, profileName, err)
		return model.ConfigProfile{}, err
	}
	return model.ConfigProfile{
		ID:          uuid.Nil,
		Application: appName,
		Profile:     profileName,
		Version:     0,
		Properties:  toConfigProperties(properties, prefix),
	}, nil
}

func (s *consulService) DeleteProfile(ctx context.Context, application string, profile string) error {
	prefix := formatKeyPrefix(s.namespace, application, profile)
	writeOptions := (&api.WriteOptions{}).WithContext(ctx)
	_, err := s.consul.KV().DeleteTree(prefix, writeOptions)
	if err != nil {
		logger.ErrorC(ctx, "Failed to delete profile tree for application=%s, profile=%s: %v", application, profile, err)
	}
	return err
}

func (s *consulService) DeleteProperties(ctx context.Context, application string, profile string, propertiesToDelete []string) error {
	batcher := newTxnBatcher(ctx, s.performTransaction)

	for _, key := range propertiesToDelete {
		kvKey := formatKeyPrefix(s.namespace, application, profile) + strings.ReplaceAll(escape(key), ".", "/")
		op := &api.KVTxnOp{Verb: api.KVDelete, Key: kvKey}

		if err := batcher.add(op, calculateOperationSize(kvKey, "")); err != nil {
			return err
		}
	}

	return batcher.done()
}

type ErrConsul struct {
	Message string
}

func (e ErrConsul) Error() string {
	return e.Message
}

type consulService struct {
	consul    *api.Client
	namespace string
}
