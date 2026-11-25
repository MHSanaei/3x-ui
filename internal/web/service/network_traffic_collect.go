package service

import (
	"fmt"
	stdnet "net"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/shirou/gopsutil/v4/net"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

type NetworkInterfaceOption struct {
	Name string `json:"name" example:"eth0"`
	Up   bool   `json:"up" example:"true"`
}

type NetworkInterfacesResponse struct {
	Interfaces     []NetworkInterfaceOption `json:"interfaces"`
	Automatic      string                   `json:"automatic" example:"eth0"`
	AutomaticError string                   `json:"automaticError"`
}

type NetworkTrafficSelection struct {
	Mode      string `json:"mode" example:"manual"`
	Name      string `json:"name" example:"eth0"`
	Available bool   `json:"available" example:"true"`
	Error     string `json:"error"`
}

type networkTrafficReader struct {
	interfaces func() ([]stdnet.Interface, error)
	counters   func() ([]net.IOCountersStat, error)
	routes     func([]stdnet.Interface) ([]networkDefaultRoute, error)
}

func (s *ServerService) trafficReader() networkTrafficReader {
	if s.networkTrafficReader != nil {
		return *s.networkTrafficReader
	}
	return networkTrafficReader{stdnet.Interfaces, func() ([]net.IOCountersStat, error) { return net.IOCounters(true) }, readNetworkDefaultRoutes}
}

func (s *SettingService) getNetworkTrafficConfig() (networkTrafficConfig, error) {
	config := networkTrafficConfig{mode: defaultValueMap["networkTrafficMode"]}
	var rows []model.Setting
	if err := database.GetDB().Where("key IN ?", []string{"networkTrafficMode", "networkTrafficInterface"}).Find(&rows).Error; err != nil {
		return config, err
	}
	for _, row := range rows {
		switch row.Key {
		case "networkTrafficMode":
			if row.Value != "" {
				config.mode = row.Value
			}
		case "networkTrafficInterface":
			config.iface = row.Value
		}
	}
	return config, nil
}

func validateNetworkTrafficSettings(settings *entity.AllSetting) error {
	if settings.NetworkTrafficMode == "" {
		if settings.NetworkTrafficInterface != "" {
			return fmt.Errorf("network traffic mode is required when selecting an interface")
		}
		return nil
	}
	if !slices.Contains([]string{"legacy", "auto", "manual"}, settings.NetworkTrafficMode) {
		return fmt.Errorf("network traffic mode must be legacy, auto, or manual")
	}
	name := settings.NetworkTrafficInterface
	if name != "" && (utf8.RuneCountInString(name) > 255 || !utf8.ValidString(name) || strings.ContainsFunc(name, unicode.IsControl)) {
		return fmt.Errorf("network traffic interface name is invalid")
	}
	if settings.NetworkTrafficMode == "manual" && name == "" {
		return fmt.Errorf("network traffic interface is required in manual mode")
	}
	return nil
}

func validateNetworkTrafficInterfaceChange(settings *entity.AllSetting, previous networkTrafficConfig) error {
	if settings.NetworkTrafficMode != "manual" || (previous.mode == "manual" && previous.iface == settings.NetworkTrafficInterface) {
		return nil
	}
	if _, err := stdnet.InterfaceByName(settings.NetworkTrafficInterface); err != nil {
		return fmt.Errorf("network traffic interface %q is unavailable: %w", settings.NetworkTrafficInterface, err)
	}
	return nil
}

func (s *ServerService) GetNetworkInterfaces() (*NetworkInterfacesResponse, error) {
	reader := s.trafficReader()
	interfaces, err := reader.interfaces()
	if err != nil {
		return nil, err
	}
	response := &NetworkInterfacesResponse{Interfaces: make([]NetworkInterfaceOption, 0, len(interfaces))}
	for _, iface := range interfaces {
		response.Interfaces = append(response.Interfaces, NetworkInterfaceOption{iface.Name, iface.Flags&stdnet.FlagUp != 0})
	}
	slices.SortFunc(response.Interfaces, func(a, b NetworkInterfaceOption) int { return strings.Compare(a.Name, b.Name) })
	routes, err := reader.routes(interfaces)
	if err == nil {
		response.Automatic, err = chooseNetworkDefaultRoute(routes, interfaces)
	}
	if err != nil {
		response.AutomaticError = err.Error()
	}
	return response, nil
}

func (s *ServerService) collectNetworkTraffic(status, previous *Status) {
	config, err := s.settingService.getNetworkTrafficConfig()
	selection := &NetworkTrafficSelection{Mode: config.mode}
	status.NetworkInterface = selection
	if config.mode == "manual" {
		selection.Name = config.iface
	}
	reader := s.trafficReader()
	var sample networkTrafficSample
	if err == nil {
		var interfaces []stdnet.Interface
		interfaces, err = reader.interfaces()
		if err == nil && config.mode == "auto" {
			var routes []networkDefaultRoute
			routes, err = reader.routes(interfaces)
			if err == nil {
				selection.Name, err = chooseNetworkDefaultRoute(routes, interfaces)
			}
		}
		if err == nil {
			var counters []net.IOCountersStat
			counters, err = reader.counters()
			if err == nil {
				sample, err = selectNetworkTraffic(config, counters, interfaces, selection.Name)
			}
		}
	}
	if err != nil {
		selection.Error = err.Error()
		if previous == nil || previous.NetworkInterface == nil || previous.NetworkInterface.Error != selection.Error {
			logger.Warning("network traffic statistics unavailable: ", err)
		}
		status.UnavailableMetrics = append(status.UnavailableMetrics, "netTraffic", "netUp", "netDown", "pktUp", "pktDown")
		return
	}
	selection.Available = true
	status.networkTrafficSample = &sample
	status.NetTraffic.Sent, status.NetTraffic.Recv = sample.sent, sample.recv
	status.NetTraffic.PktSent, status.NetTraffic.PktRecv = sample.pktSent, sample.pktRecv
	if previous != nil {
		rates := networkTrafficRates(previous.networkTrafficSample, &sample, status.T.Sub(previous.T))
		status.NetIO.Up, status.NetIO.Down, status.NetIO.PktUp, status.NetIO.PktDown = rates[0], rates[1], rates[2], rates[3]
	}
}
