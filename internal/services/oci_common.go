package services

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/adiecho/oci-panel/internal/config"
	"github.com/adiecho/oci-panel/internal/models"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/oracle/oci-go-sdk/v65/identity"
	"github.com/oracle/oci-go-sdk/v65/identitydomains"
	"github.com/oracle/oci-go-sdk/v65/monitoring"
	"github.com/oracle/oci-go-sdk/v65/networkloadbalancer"
)

type OCIService struct {
	cfg         *config.Config
	clientCache sync.Map
}

type cachedClients struct {
	configProvider common.ConfigurationProvider
	compute        *core.ComputeClient
	network        *core.VirtualNetworkClient
	blockstorage   *core.BlockstorageClient
	identity       *identity.IdentityClient
	nlb            *networkloadbalancer.NetworkLoadBalancerClient
	mu             sync.Mutex
}

func NewOCIService(cfg *config.Config) *OCIService {
	return &OCIService{cfg: cfg}
}

func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func (s *OCIService) GetConfigProvider(user *models.OciUser) (common.ConfigurationProvider, error) {
	keyPath := "keys/" + user.OciKeyPath
	privateKey, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	return common.NewRawConfigurationProvider(
		user.OciTenantID,
		user.OciUserID,
		user.OciRegion,
		user.OciFingerprint,
		string(privateKey),
		nil,
	), nil
}

func (s *OCIService) getCachedClients(user *models.OciUser) (*cachedClients, error) {
	cacheKey := user.ID + ":" + user.OciRegion
	if cached, ok := s.clientCache.Load(cacheKey); ok {
		return cached.(*cachedClients), nil
	}

	configProvider, err := s.GetConfigProvider(user)
	if err != nil {
		return nil, err
	}

	cc := &cachedClients{configProvider: configProvider}
	actual, _ := s.clientCache.LoadOrStore(cacheKey, cc)
	return actual.(*cachedClients), nil
}

func (s *OCIService) GetComputeClient(user *models.OciUser) (core.ComputeClient, error) {
	cc, err := s.getCachedClients(user)
	if err != nil {
		return core.ComputeClient{}, err
	}

	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.compute != nil {
		return *cc.compute, nil
	}

	client, err := core.NewComputeClientWithConfigurationProvider(cc.configProvider)
	if err != nil {
		return core.ComputeClient{}, err
	}
	cc.compute = &client
	return client, nil
}

func (s *OCIService) GetVirtualNetworkClient(user *models.OciUser) (core.VirtualNetworkClient, error) {
	cc, err := s.getCachedClients(user)
	if err != nil {
		return core.VirtualNetworkClient{}, err
	}

	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.network != nil {
		return *cc.network, nil
	}

	client, err := core.NewVirtualNetworkClientWithConfigurationProvider(cc.configProvider)
	if err != nil {
		return core.VirtualNetworkClient{}, err
	}
	cc.network = &client
	return client, nil
}

func (s *OCIService) GetBlockstorageClient(user *models.OciUser) (core.BlockstorageClient, error) {
	cc, err := s.getCachedClients(user)
	if err != nil {
		return core.BlockstorageClient{}, err
	}

	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.blockstorage != nil {
		return *cc.blockstorage, nil
	}

	client, err := core.NewBlockstorageClientWithConfigurationProvider(cc.configProvider)
	if err != nil {
		return core.BlockstorageClient{}, err
	}
	cc.blockstorage = &client
	return client, nil
}

func (s *OCIService) GetIdentityClient(user *models.OciUser) (identity.IdentityClient, error) {
	cc, err := s.getCachedClients(user)
	if err != nil {
		return identity.IdentityClient{}, err
	}

	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.identity != nil {
		return *cc.identity, nil
	}

	client, err := identity.NewIdentityClientWithConfigurationProvider(cc.configProvider)
	if err != nil {
		return identity.IdentityClient{}, err
	}
	cc.identity = &client
	return client, nil
}

func (s *OCIService) GetIdentityDomainsClient(user *models.OciUser, endpoint string) (identitydomains.IdentityDomainsClient, error) {
	configProvider, err := s.GetConfigProvider(user)
	if err != nil {
		return identitydomains.IdentityDomainsClient{}, err
	}

	client, err := identitydomains.NewIdentityDomainsClientWithConfigurationProvider(configProvider, endpoint)
	if err != nil {
		return identitydomains.IdentityDomainsClient{}, err
	}

	return client, nil
}

func (s *OCIService) GetMonitoringClient(user *models.OciUser) (monitoring.MonitoringClient, error) {
	configProvider, err := s.GetConfigProvider(user)
	if err != nil {
		return monitoring.MonitoringClient{}, err
	}

	client, err := monitoring.NewMonitoringClientWithConfigurationProvider(configProvider)
	if err != nil {
		return monitoring.MonitoringClient{}, err
	}

	return client, nil
}

func (s *OCIService) GetNetworkLoadBalancerClient(user *models.OciUser) (networkloadbalancer.NetworkLoadBalancerClient, error) {
	cc, err := s.getCachedClients(user)
	if err != nil {
		return networkloadbalancer.NetworkLoadBalancerClient{}, err
	}

	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.nlb != nil {
		return *cc.nlb, nil
	}

	client, err := networkloadbalancer.NewNetworkLoadBalancerClientWithConfigurationProvider(cc.configProvider)
	if err != nil {
		return networkloadbalancer.NetworkLoadBalancerClient{}, err
	}
	cc.nlb = &client
	return client, nil
}

func (s *OCIService) GetConfigProviderForRegion(user *models.OciUser, region string) (common.ConfigurationProvider, error) {
	keyPath := "keys/" + user.OciKeyPath
	privateKey, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	return common.NewRawConfigurationProvider(
		user.OciTenantID,
		user.OciUserID,
		region,
		user.OciFingerprint,
		string(privateKey),
		nil,
	), nil
}

func (s *OCIService) GetComputeClientForRegion(user *models.OciUser, region string) (core.ComputeClient, error) {
	configProvider, err := s.GetConfigProviderForRegion(user, region)
	if err != nil {
		return core.ComputeClient{}, err
	}
	return core.NewComputeClientWithConfigurationProvider(configProvider)
}

func (s *OCIService) GetVirtualNetworkClientForRegion(user *models.OciUser, region string) (core.VirtualNetworkClient, error) {
	configProvider, err := s.GetConfigProviderForRegion(user, region)
	if err != nil {
		return core.VirtualNetworkClient{}, err
	}
	return core.NewVirtualNetworkClientWithConfigurationProvider(configProvider)
}

func (s *OCIService) GetIdentityClientForRegion(user *models.OciUser, region string) (identity.IdentityClient, error) {
	configProvider, err := s.GetConfigProviderForRegion(user, region)
	if err != nil {
		return identity.IdentityClient{}, err
	}
	return identity.NewIdentityClientWithConfigurationProvider(configProvider)
}

// InvalidateClientCache removes cached clients for a user (call when credentials change)
func (s *OCIService) InvalidateClientCache(userID string) {
	s.clientCache.Range(func(key, value any) bool {
		if k, ok := key.(string); ok && len(k) > len(userID) && k[:len(userID)] == userID {
			s.clientCache.Delete(key)
		}
		return true
	})
}

// isPrivateIP checks if an IP address is private
func isPrivateIP(ip string) bool {
	return strings.HasPrefix(ip, "10.") ||
		strings.HasPrefix(ip, "172.16.") ||
		strings.HasPrefix(ip, "172.17.") ||
		strings.HasPrefix(ip, "172.18.") ||
		strings.HasPrefix(ip, "172.19.") ||
		strings.HasPrefix(ip, "172.20.") ||
		strings.HasPrefix(ip, "172.21.") ||
		strings.HasPrefix(ip, "172.22.") ||
		strings.HasPrefix(ip, "172.23.") ||
		strings.HasPrefix(ip, "172.24.") ||
		strings.HasPrefix(ip, "172.25.") ||
		strings.HasPrefix(ip, "172.26.") ||
		strings.HasPrefix(ip, "172.27.") ||
		strings.HasPrefix(ip, "172.28.") ||
		strings.HasPrefix(ip, "172.29.") ||
		strings.HasPrefix(ip, "172.30.") ||
		strings.HasPrefix(ip, "172.31.") ||
		strings.HasPrefix(ip, "192.168.")
}
