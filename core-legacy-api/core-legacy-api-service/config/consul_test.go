package config

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Netcracker/qubership-mini-core/core-legacy-api/core-legacy-api-service/model"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const namespace = "test-ns"

// ConsulServiceTestSuite mirrors the Java ConsulServiceTest: it spins up a
// real Consul dev-mode container per test and exercises ConsulService
// against it.
type ConsulServiceTestSuite struct {
	suite.Suite

	ctx           context.Context
	consulC       testcontainers.Container
	consulClient  *api.Client
	consulService ConfigService
}

func TestConsulServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ConsulServiceTestSuite))
}

func (s *ConsulServiceTestSuite) SetupTest() {
	s.ctx = context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "hashicorp/consul:1.8.9",
		ExposedPorts: []string{"8500/tcp"},
		Cmd:          []string{"agent", "-dev", "-client", "0.0.0.0", "--enable-script-checks=true"},
		WaitingFor:   wait.ForListeningPort("8500/tcp"),
	}

	consulC, err := testcontainers.GenericContainer(s.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(s.T(), err)
	s.consulC = consulC

	host, err := consulC.Host(s.ctx)
	require.NoError(s.T(), err)
	port, err := consulC.MappedPort(s.ctx, "8500")
	require.NoError(s.T(), err)

	cfg := api.DefaultConfig()
	cfg.Address = fmt.Sprintf("%s:%s", host, port.Port())

	client, err := api.NewClient(cfg)
	require.NoError(s.T(), err)

	s.consulClient = client
	s.consulService = NewConsulService(client, namespace)
}

func (s *ConsulServiceTestSuite) TearDownTest() {
	if s.consulC != nil {
		_ = s.consulC.Terminate(s.ctx)
	}
}

func propertiesAsMap(profile model.ConfigProfile) map[string]string {
	result := make(map[string]string, len(profile.Properties))
	for _, p := range profile.Properties {
		result[p.Key] = p.Value
	}
	return result
}

func (s *ConsulServiceTestSuite) TestDeleteProperties() {
	const application = "test-app"
	const profile1 = "default"
	const profile2 = "prod"
	const key1 = "my.lovely.key1"
	const key2 = "my.lovely.key2"
	const key3 = "my.lovely.key3"

	properties1 := map[string]string{
		key1: "my.lovely.value1",
		key2: "my.lovely.value2",
		key3: "my.lovely.value3",
	}
	properties2 := map[string]string{
		key1: "my.lovely.value1",
	}

	err := s.consulService.AddProperties(context.Background(), application, profile1, properties1)
	require.NoError(s.T(), err)

	profile1FromConsul, err := s.consulService.FindByApplicationAndProfile(context.Background(), application, profile1)
	require.NoError(s.T(), err)
	profile1Map := propertiesAsMap(profile1FromConsul)
	assert.Contains(s.T(), profile1Map, key1)
	assert.Contains(s.T(), profile1Map, key2)
	assert.Contains(s.T(), profile1Map, key3)

	err = s.consulService.AddProperties(context.Background(), application, profile2, properties2)
	require.NoError(s.T(), err)

	profile2FromConsul, err := s.consulService.FindByApplicationAndProfile(context.Background(), application, profile2)
	require.NoError(s.T(), err)
	assert.Contains(s.T(), propertiesAsMap(profile2FromConsul), key1)

	err = s.consulService.DeleteProperties(context.Background(), application, profile1, []string{key1, key3})
	require.NoError(s.T(), err)

	profile1AfterDelete, err := s.consulService.FindByApplicationAndProfile(context.Background(), application, profile1)
	require.NoError(s.T(), err)
	profile1MapAfterDelete := propertiesAsMap(profile1AfterDelete)
	assert.NotContains(s.T(), profile1MapAfterDelete, key1)
	assert.NotContains(s.T(), profile1MapAfterDelete, key3)
	assert.Contains(s.T(), profile1MapAfterDelete, key2)

	profile2AfterDelete, err := s.consulService.FindByApplicationAndProfile(context.Background(), application, profile2)
	require.NoError(s.T(), err)
	assert.Contains(s.T(), propertiesAsMap(profile2AfterDelete), key1)

	err = s.consulService.DeleteProperties(context.Background(), application, profile2, []string{key1})
	require.NoError(s.T(), err)

	profile2Final, err := s.consulService.FindByApplicationAndProfile(context.Background(), application, profile2)
	require.NoError(s.T(), err)
	assert.Empty(s.T(), profile2Final.Properties)
}

func (s *ConsulServiceTestSuite) TestSetFine() {
	profile := "specific"
	application := "test-app"

	properties := map[string]string{
		"my.lovely.key":  "my-lovely.value",
		"my.lovely.key1": "my-lovely.value1",
		"my.lovely.key2": "my-lovely.value2",
	}

	err := s.consulService.AddProperties(context.Background(), application, profile, properties)
	require.NoError(s.T(), err)

	profile2 := "default"
	application2 := "global"

	properties2 := map[string]string{
		"my.lovely.key3": "my-lovely.value3",
		"my.lovely.key4": "my-lovely.value4",
		"my.lovely.key5": "my-lovely.value5",
	}

	err = s.consulService.AddProperties(context.Background(), application2, profile2, properties2)
	require.NoError(s.T(), err)

	dataFromConsul, err := s.consulService.FindByApplicationAndProfile(context.Background(), "test-app", "specific")
	require.NoError(s.T(), err)

	assert.Equal(s.T(), application, dataFromConsul.Application)
	assert.Equal(s.T(), profile, dataFromConsul.Profile)
	assert.NotEmpty(s.T(), dataFromConsul.Properties)

	// every property in configProfile must exist (by key+value) in dataFromConsul
	for propK, prop_v := range properties {
		found := false
		for _, fromConsul := range dataFromConsul.Properties {
			if fromConsul.Key == propK && fromConsul.Value == prop_v {
				found = true
				break
			}
		}
		assert.Truef(s.T(), found, "expected property %s=%s to be present", propK, prop_v)
	}

	allConfigProfiles, err := s.consulService.FindAll(context.Background())
	require.NoError(s.T(), err)

	assert.Len(s.T(), allConfigProfiles, 2)

	// both applications ("global", "test-app") should be represented
	for _, app := range []string{"global", "test-app"} {
		found := false
		for _, cp := range allConfigProfiles {
			if cp.Application == app {
				found = true
				break
			}
		}
		assert.Truef(s.T(), found, "expected application %s to be present", app)
	}

	// both profiles ("specific", "default") should be represented
	for _, profile := range []string{"specific", "default"} {
		found := false
		for _, cp := range allConfigProfiles {
			if cp.Profile == profile {
				found = true
				break
			}
		}
		assert.Truef(s.T(), found, "expected profile %s to be present", profile)
	}
}

func (s *ConsulServiceTestSuite) TestFailureOnBigProperty() {
	value := strings.Repeat("a", txnMaxReqLen*2)
	profile := "default"
	application := "test-app"
	properties := map[string]string{
		"my.lovely.key": value,
	}

	err := s.consulService.AddProperties(context.Background(), application, profile, properties)
	require.Error(s.T(), err)

	var consulErr ErrConsul
	require.ErrorAs(s.T(), err, &consulErr)

	kv, _, kvErr := s.consulClient.KV().Get("config/test-ns/test-app/my/lovely/key", nil)
	require.NoError(s.T(), kvErr)
	assert.Nil(s.T(), kv)
}

func (s *ConsulServiceTestSuite) TestSkipValueForKeyEqualedToPrefix() {
	propPath := fmt.Sprintf("%s/%s/test-app", consulConfigPrefix, namespace)

	_, err := s.consulClient.KV().Put(&api.KVPair{
		Key:   propPath,
		Value: []byte("hello world from prefix"),
	}, nil)
	require.NoError(s.T(), err)

	_, err = s.consulClient.KV().Put(&api.KVPair{
		Key:   propPath + "/key",
		Value: []byte("hello world from prop"),
	}, nil)
	require.NoError(s.T(), err)

	pair1, _, err := s.consulClient.KV().Get(propPath, nil)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), pair1)

	pair2, _, err := s.consulClient.KV().Get(propPath+"/key", nil)
	require.NoError(s.T(), err)
	assert.NotNil(s.T(), pair2)

	list, err := s.consulService.FindAll(context.Background())
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), list)

	assert.Len(s.T(), list[0].Properties, 1)
	assert.Equal(s.T(), "hello world from prop", list[0].Properties[0].Value)

}

func (s *ConsulServiceTestSuite) TestSaveSeveralBigProperties() {
	size := txnMaxReqLen/2 - 1000
	application := "test-app"
	profile := "default"
	properties := map[string]string{
		"my.lovely.key1": strings.Repeat("a", size),
		"my.lovely.key2": strings.Repeat("b", size),
		"my.lovely.key3": strings.Repeat("c", size),
	}
	err := s.consulService.AddProperties(context.Background(), application, profile, properties)
	s.Require().NoError(err)

	prefix := formatKeyPrefix(namespace, "test-app", "default")

	pair, _, err := s.consulClient.KV().Get(prefix+"my/lovely/key1", nil)
	s.Require().NoError(err)
	s.NotNil(pair)

	pair, _, err = s.consulClient.KV().Get(prefix+"my/lovely/key2", nil)
	s.Require().NoError(err)
	s.NotNil(pair)

	pair, _, err = s.consulClient.KV().Get(prefix+"my/lovely/key3", nil)
	s.Require().NoError(err)
	s.NotNil(pair)
}
