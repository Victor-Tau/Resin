package service

import (
	"errors"
	"fmt"
	"maps"
	"net/netip"
	"slices"
	"strings"

	"github.com/Resinat/Resin/internal/localipv6"
	"github.com/Resinat/Resin/internal/model"
	"github.com/Resinat/Resin/internal/state"
	"github.com/google/uuid"
)

const maxIPv6BatchSize = 20

type IPv6ConfigInput struct {
	SOCKS5Account
	PrefixID    string   `json:"prefix_id,omitempty"`
	Interface   string   `json:"interface"`
	EndpointIDs []string `json:"endpoint_ids"`
}

type IPv6ConfigResponse struct {
	IPv6ConfigInput
	ID string `json:"id"`
}

func ipv6ConfigResponse(config model.IPv6Config) IPv6ConfigResponse {
	return IPv6ConfigResponse{
		ID: config.ID,
		IPv6ConfigInput: IPv6ConfigInput{
			PrefixID:    config.PrefixID,
			Interface:   config.Account.Interface,
			EndpointIDs: config.EndpointIDs,
			SOCKS5Account: SOCKS5Account{
				Username: config.Username,
				Address:  config.Account.Address.String(),
			},
		},
	}
}

func (s *ControlPlaneService) ListIPv6Configs() ([]IPv6ConfigResponse, error) {
	if s == nil || s.Engine == nil {
		return nil, internal("IPv6 configuration service is not initialized", nil)
	}
	s.endpointMu.RLock()
	defer s.endpointMu.RUnlock()

	configs, err := s.Engine.ListIPv6Configs()
	if err != nil {
		return nil, internal("list IPv6 configurations", err)
	}
	responses := make([]IPv6ConfigResponse, 0, len(configs))
	for _, config := range configs {
		responses = append(responses, ipv6ConfigResponse(config))
	}
	return responses, nil
}

func (s *ControlPlaneService) CreateIPv6Configs(inputs []IPv6ConfigInput) ([]IPv6ConfigResponse, error) {
	if len(inputs) == 0 || len(inputs) > maxIPv6BatchSize {
		return nil, invalidArg(fmt.Sprintf("each batch must contain 1–%d IPv6 configurations", maxIPv6BatchSize))
	}
	return s.changeIPv6Configs("", inputs)
}

func (s *ControlPlaneService) UpdateIPv6Config(id string, input IPv6ConfigInput) (IPv6ConfigResponse, error) {
	responses, err := s.changeIPv6Configs(id, []IPv6ConfigInput{input})
	if err != nil {
		return IPv6ConfigResponse{}, err
	}
	return responses[0], nil
}

func (s *ControlPlaneService) DeleteIPv6Config(id string) error {
	_, err := s.changeIPv6Configs(id, nil)
	return err
}

func (s *ControlPlaneService) changeIPv6Configs(id string, inputs []IPv6ConfigInput) ([]IPv6ConfigResponse, error) {
	if s == nil || s.Engine == nil {
		return nil, internal("IPv6 configuration service is not initialized", nil)
	}
	s.endpointMu.Lock()
	defer s.endpointMu.Unlock()

	current, err := s.Engine.ListIPv6Configs()
	if err != nil {
		return nil, internal("list IPv6 configurations", err)
	}
	custom, err := s.Engine.ListEndpoints()
	if err != nil {
		return nil, internal("list endpoints", err)
	}
	endpoints := append([]model.Endpoint{s.defaultEndpoint()}, custom...)
	available := make(map[string]bool, len(endpoints))
	for _, endpoint := range endpoints {
		available[endpoint.ID] = endpoint.AllowSOCKS5
	}

	var previous model.IPv6Config
	affected := make(map[string]bool)
	usedAddresses := make(map[netip.Addr]bool, len(current)+len(inputs))
	next := make([]model.IPv6Config, 0, len(current)+len(inputs))
	for _, config := range current {
		if config.ID == id {
			previous = config
			for _, endpointID := range config.EndpointIDs {
				affected[endpointID] = true
			}
		} else {
			next = append(next, config)
			usedAddresses[config.Account.Address.Addr()] = true
		}
	}
	if id != "" && previous.ID == "" {
		return nil, notFound("IPv6 configuration not found")
	}

	saves := make([]model.IPv6Config, 0, len(inputs))
	responses := make([]IPv6ConfigResponse, 0, len(inputs))
	prefixes := make(map[string]model.IPv6Prefix)
	for _, input := range inputs {
		if input.PrefixID != "" {
			prefix, exists := prefixes[input.PrefixID]
			if !exists {
				prefix, err = s.Engine.GetIPv6Prefix(input.PrefixID)
				if errors.Is(err, state.ErrNotFound) {
					return nil, invalidArg("selected IPv6 prefix no longer exists")
				}
				if err != nil {
					return nil, internal("read IPv6 prefix", err)
				}
				prefixes[input.PrefixID] = prefix
			}
			address, err := netip.ParsePrefix(strings.TrimSpace(input.Address))
			if err != nil || address.Bits() != 64 || !prefix.Prefix.Contains(address.Addr()) {
				return nil, invalidArg("proxy address must belong to the selected IPv6 prefix")
			}
			input.Interface = prefix.Interface
		}
		config, problem := parseIPv6Config(input, previous)
		if problem != nil {
			return nil, problem
		}
		if usedAddresses[config.Account.Address.Addr()] {
			return nil, conflict(fmt.Sprintf("IPv6 address %s is already configured", config.Account.Address.Addr()))
		}
		usedAddresses[config.Account.Address.Addr()] = true
		for _, endpointID := range config.EndpointIDs {
			socks5, exists := available[endpointID]
			if !exists || (!socks5 && !slices.Contains(previous.EndpointIDs, endpointID)) {
				return nil, invalidArg("bindings must reference existing endpoints with SOCKS5 enabled")
			}
			affected[endpointID] = true
		}
		next = append(next, config)
		saves = append(saves, config)
		responses = append(responses, ipv6ConfigResponse(config))
	}

	type endpointChange struct {
		old, next  model.Endpoint
		wasRunning bool
	}
	changes := make([]endpointChange, 0, len(affected))
	for _, endpoint := range endpoints {
		if !affected[endpoint.ID] {
			continue
		}
		updated, err := EndpointWithIPv6Configs(endpoint, next)
		if err != nil {
			return nil, conflict(err.Error())
		}
		if !endpoint.Enabled || s.EndpointRuntime == nil {
			continue
		}
		old, err := EndpointWithIPv6Configs(endpoint, current)
		if err != nil {
			return nil, internal("read previous endpoint IPv6 configuration", err)
		}
		status := s.EndpointRuntime.EndpointStatus(endpoint.ID).State
		changes = append(changes, endpointChange{old: old, next: updated, wasRunning: status == "active" || status == "starting"})
	}
	rollback := func(applied int) error {
		var err error
		for _, change := range slices.Backward(changes[:applied]) {
			if change.wasRunning {
				err = errors.Join(err, s.EndpointRuntime.ApplyEndpoint(change.old))
			} else {
				s.EndpointRuntime.RemoveEndpoint(change.old.ID)
			}
		}
		return err
	}
	for i, change := range changes {
		if err := s.EndpointRuntime.ApplyEndpoint(change.next); err != nil {
			if rollbackErr := rollback(i); rollbackErr != nil {
				return nil, internal("rollback endpoint IPv6 configuration", errors.Join(err, rollbackErr))
			}
			return nil, conflict(fmt.Sprintf("apply IPv6 configuration on port %d: %v", change.next.Port, err))
		}
	}
	deleteID := ""
	if len(inputs) == 0 {
		deleteID = id
	}
	if err := s.Engine.WriteIPv6Configs(saves, deleteID); err != nil {
		return nil, internal("persist IPv6 configuration", errors.Join(err, rollback(len(changes))))
	}
	return responses, nil
}

func parseIPv6Config(input IPv6ConfigInput, previous model.IPv6Config) (model.IPv6Config, *ServiceError) {
	input.Username = strings.TrimSpace(input.Username)
	if len(input.Username) == 0 || len(input.Username) > 255 {
		return model.IPv6Config{}, invalidArg("SOCKS5 username must be 1–255 bytes")
	}
	input.Interface = strings.TrimSpace(input.Interface)
	mapping, err := localipv6.ParseMapping(input.Interface, strings.TrimSpace(input.Address))
	if err != nil {
		return model.IPv6Config{}, invalidArg(err.Error())
	}
	account := previous.Account
	if (input.Password == "" && account.PasswordHash == "") || len(input.Password) > 255 {
		return model.IPv6Config{}, invalidArg("new SOCKS5 passwords must be 1–255 bytes")
	}
	if input.Password != "" {
		if err := account.SetPassword(input.Password); err != nil {
			return model.IPv6Config{}, internal("hash SOCKS5 password", err)
		}
	}
	account.Interface = mapping.Interface
	account.Address = netip.PrefixFrom(mapping.Address, 64)
	id := previous.ID
	if id == "" {
		id = uuid.NewString()
	}
	endpointIDs := append([]string{}, input.EndpointIDs...)
	slices.Sort(endpointIDs)
	return model.IPv6Config{
		ID: id, PrefixID: input.PrefixID, Username: input.Username, Account: account, EndpointIDs: slices.Compact(endpointIDs),
	}, nil
}

// EndpointWithIPv6Configs adds independent bindings to an endpoint's legacy accounts.
func EndpointWithIPv6Configs(endpoint model.Endpoint, configs []model.IPv6Config) (model.Endpoint, error) {
	var merged map[string]model.IPv6Account
	var addresses map[netip.Addr]bool
	for _, config := range configs {
		if !slices.Contains(config.EndpointIDs, endpoint.ID) {
			continue
		}
		if merged == nil {
			merged = make(map[string]model.IPv6Account, len(endpoint.SOCKS5Accounts)+1)
			addresses = make(map[netip.Addr]bool, len(endpoint.SOCKS5Accounts)+1)
			maps.Copy(merged, endpoint.SOCKS5Accounts)
			for _, account := range merged {
				addresses[account.Address.Addr()] = true
			}
		}
		if _, exists := merged[config.Username]; exists {
			return endpoint, fmt.Errorf("SOCKS5 username %q is already configured on port %d", config.Username, endpoint.Port)
		}
		if address := config.Account.Address.Addr(); addresses[address] {
			return endpoint, fmt.Errorf("IPv6 address %s is already configured on port %d", address, endpoint.Port)
		}
		merged[config.Username] = config.Account
		addresses[config.Account.Address.Addr()] = true
	}
	if merged != nil {
		endpoint.SOCKS5Accounts = merged
	}
	return endpointForSOCKS5(endpoint), nil
}

func endpointForSOCKS5(endpoint model.Endpoint) model.Endpoint {
	if !endpoint.AllowSOCKS5 {
		endpoint.SOCKS5IPv6Interface = ""
		endpoint.SOCKS5Accounts = nil
	}
	return endpoint
}
