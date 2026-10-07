package config

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/netcracker/qubership-core-lib-go-actuator-common/v2/health"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-mini-core/core-legacy-api/core-legacy-api-service/model"

	"github.com/google/uuid"
	"github.com/hashicorp/consul/api"
)

const consulPropertiesSource = "consul"
const consulConfigPrefix = "config"
const consulGlobalApplicationName = "application"
const configPropertiesGlobalApplicationName = "global"
const defaultProfileName = "default"
const consulTxOperationLimit = 64
const txnMaxReqLen = 512 * 1024

func NewConsulService() *consulService {

	namespace := configloader.GetOrDefaultString("microservice.namespace", "")
	consulURL := configloader.GetOrDefaultString("consul.url", "")
	consulToken := configloader.GetOrDefaultString("consul.token", "")

	u, err := url.Parse(consulURL)
	if err != nil {
		logger.Errorf("invalid consul.url %q: %v", consulURL, err)
		panic(err)
	}

	conf := api.DefaultConfig()
	conf.Address = u.Host
	conf.Scheme = u.Scheme
	conf.Token = consulToken

	consulClient, err := api.NewClient(conf)

	if err != nil {
		logger.Errorf("Couldn't create Consul client: %v", err)
		panic(err)
	}

	return &consulService{consulClient, namespace, logging.GetLogger("ConsulService")}
}

func (s *consulService) HealthCheck() health.Status {
	_, err := s.consul.Status().Leader()
	if err != nil {
		return health.Status{Name: health.StatusProblem, Details: map[string]interface{}{
			"error": err.Error(),
		}}
	}
	return health.Status{Name: health.StatusUp}
}

func (s *consulService) AddProperties(ctx context.Context, application string, profile string, properties map[string]string) error {
	batcher := newTxnBatcher(ctx, s.performTransaction)

	for key, value := range properties {
		kvKey := formatKeyPrefix(s.namespace, application, profile) + strings.ReplaceAll(escape(key), ".", "/")
		op := &api.KVTxnOp{Verb: api.KVSet, Key: kvKey, Value: []byte(value)}

		if err := batcher.add(op, calculateOperationSize(kvKey, value)); err != nil {
			return fmt.Errorf("failed to prepare property update for application=%s profile=%s key=%s: %w", application, profile, key, err)
		}
	}

	if err := batcher.done(); err != nil {
		return fmt.Errorf("failed to add properties for application=%s profile=%s: %w", application, profile, err)
	}
	return nil
}

func (s *consulService) performTransaction(ctx context.Context, operation api.TxnOps) error {
	s.logger.InfoC(ctx, "Executing Consul transaction with %d operations", len(operation))
	queryOptions := (&api.QueryOptions{}).WithContext(ctx)
	ok, resp, _, err := s.consul.Txn().Txn(operation, queryOptions) //TODO: transaction?
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("consul transaction cancelled for context=%v: %w", ctx.Err(), ctx.Err())
		}
		return fmt.Errorf("failed to execute Consul transaction: %w", err)
	}

	if !ok {
		var reasons []string
		if resp != nil {
			for _, e := range resp.Errors {
				reasons = append(reasons, fmt.Sprintf("OpIndex=%d, What=%s", e.OpIndex, e.What))
			}
		}
		return fmt.Errorf("consul transaction rejected: %s", strings.Join(reasons, "; "))
	}

	s.logger.InfoC(ctx, "Consul transaction completed successfully")

	return nil
}

func (s *consulService) FindAll(ctx context.Context) ([]model.ConfigProfile, error) {
	prefix := consulConfigPrefix + "/" + s.namespace + "/"
	queryOptions := (&api.QueryOptions{}).WithContext(ctx)
	// Get all KV pairs under the prefix.
	pairs, _, err := s.consul.KV().List(prefix, queryOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list KV pairs under prefix=%s: %w", prefix, err)
	}

	propertiesByApp := groupPropertiesByApplication(pairs)

	profiles := buildConfigProfiles(propertiesByApp, prefix)

	s.logger.Infof("Found %d config profiles", len(profiles))

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
		return model.ConfigProfile{}, fmt.Errorf("failed to list KV pairs for application=%s profile=%s: %w", appName, profileName, err)
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
		return fmt.Errorf("failed to delete profile tree for application=%s profile=%s: %w", application, profile, err)
	}
	return nil
}

func (s *consulService) DeleteProperties(ctx context.Context, application string, profile string, propertiesToDelete []string) error {
	batcher := newTxnBatcher(ctx, s.performTransaction)

	for _, key := range propertiesToDelete {
		kvKey := formatKeyPrefix(s.namespace, application, profile) + strings.ReplaceAll(escape(key), ".", "/")
		op := &api.KVTxnOp{Verb: api.KVDelete, Key: kvKey}

		if err := batcher.add(op, calculateOperationSize(kvKey, "")); err != nil {
			return fmt.Errorf("failed to prepare property deletion for application=%s profile=%s key=%s: %w", application, profile, key, err)
		}
	}

	if err := batcher.done(); err != nil {
		return fmt.Errorf("failed to delete properties for application=%s profile=%s: %w", application, profile, err)
	}
	return nil
}

type consulService struct {
	consul    *api.Client
	namespace string
	logger    logging.Logger
}
