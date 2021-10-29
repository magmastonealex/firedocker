// Package mmds provides utilities to query network and other state from the instance-metadata service.
package mmds

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// This struct isn't defined by Firecracker - it's the format the init & containers will expect to be available over MMDS.
type MMDSRoute struct {
	Gw      string `json:"gw"`
	Network string `json:"network"`
}
type MMDSIPConfig struct {
	IPCIDR       string      `json:"ip_cidr"`
	PrimaryDNS   string      `json:"primary_dns"`
	SecondaryDNS string      `json:"secondary_dns"`
	Routes       []MMDSRoute `json:"routes"`
}

type ContainerRuntimeConfig struct {
	Entrypoint  []string
	Cmd         []string
	Environment []string
	Workdir     string
}

// FetchIPConfig will retrieve the desired IP configuration for this VM from MMDS.
func FetchIPConfig() (*MMDSIPConfig, error) {
	client := &http.Client{
		Timeout: 2 * time.Second,
	}
	resp, err := client.Get("http://169.254.169.254/ipconfig")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ipconfig from mmds: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to fetch ipconfi from mmds (status code %d)", resp.StatusCode)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("could not read body from mmds response: %w", err)
	}
	ipConfig := &MMDSIPConfig{}
	err = json.Unmarshal(bodyBytes, ipConfig)
	if err != nil {
		return nil, fmt.Errorf("could not decode mmds response: %w", err)
	}
	return ipConfig, nil
}

// FetchRuntimeConfig will retrieve the desired exec configuration for this VM from MMDS.
func FetchRuntimeConfig() (*ContainerRuntimeConfig, error) {
	client := &http.Client{
		Timeout: 2 * time.Second,
	}
	resp, err := client.Get("http://169.254.169.254/runtimeConfig")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch runtime config from mmds: %w", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to fetch runtime config from mmds (status code %d)", resp.StatusCode)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("could not read body from mmds response: %w", err)
	}
	rconfig := &ContainerRuntimeConfig{}
	err = json.Unmarshal(bodyBytes, rconfig)
	if err != nil {
		return nil, fmt.Errorf("could not decode mmds response: %w", err)
	}
	return rconfig, nil
}
