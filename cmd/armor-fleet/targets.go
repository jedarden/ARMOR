// Package main provides target configuration loading for armor-fleet.
package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Target defines an ARMOR instance to monitor.
type Target struct {
	Name      string `yaml:"name"`
	Cluster   string `yaml:"cluster"`
	Namespace string `yaml:"namespace"`
	Service   string `yaml:"service"`
	AdminPort int    `yaml:"admin_port"`
}

// LoadTargets loads target definitions from a YAML file.
func LoadTargets(path string) ([]Target, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var targets []Target
	if err := yaml.Unmarshal(data, &targets); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}

	// Validate targets
	for i, t := range targets {
		if t.Name == "" {
			return nil, fmt.Errorf("target %d: name is required", i)
		}
		if t.Cluster == "" {
			return nil, fmt.Errorf("target %s: cluster is required", t.Name)
		}
		if t.Namespace == "" {
			return nil, fmt.Errorf("target %s: namespace is required", t.Name)
		}
		if t.Service == "" {
			return nil, fmt.Errorf("target %s: service is required", t.Name)
		}
		if t.AdminPort == 0 {
			return nil, fmt.Errorf("target %s: admin_port is required", t.Name)
		}
	}

	return targets, nil
}
