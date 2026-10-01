package deployer

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	manifesttypes "github.com/k8s-manifest-kit/engine/pkg/types"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
)

// Module config type identifiers.
const (
	ConfigAPIVersion = "deployer.opendatahub.io/v1alpha1"
	ConfigKind       = "PlatformModuleConfig"
)

// PlatformModuleConfig is a local, CRD-shaped description of one module.
// It is not installed as a Kubernetes custom resource.
type PlatformModuleConfig struct {
	APIVersion string         `yaml:"apiVersion"`
	Kind       string         `yaml:"kind"`
	Metadata   ModuleMetadata `yaml:"metadata"`
	Spec       ModuleSpec     `yaml:"spec"`
}

// Clone returns an independent copy of the config's YAML-compatible values.
func (config PlatformModuleConfig) Clone() PlatformModuleConfig {
	clone := config
	clone.Spec.Config = manifesttypes.Values(config.Spec.Config).DeepClone()
	clone.Spec.RelatedImages = slices.Clone(config.Spec.RelatedImages)
	clone.Spec.Services = slices.Clone(config.Spec.Services)
	return clone
}

// ModuleMetadata identifies the module and Helm release.
type ModuleMetadata struct {
	Name string `yaml:"name"`
}

// ModuleSpec describes the module chart and its values sources.
type ModuleSpec struct {
	ModuleRef     ModuleRef      `yaml:"moduleRef"`
	Chart         ChartSpec      `yaml:"chart,omitempty"`
	Runlevel      int            `yaml:"runlevel,omitempty"`
	RelatedImages []string       `yaml:"relatedImages,omitempty"`
	Config        map[string]any `yaml:"config,omitempty"`
	Services      []string       `yaml:"services,omitempty"`
}

// ModuleRef identifies the module CR that must not be deployed by this package.
type ModuleRef struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Name       string `yaml:"name"`
}

// ChartSpec selects a local chart directory and optional Helm version constraint.
type ChartSpec struct {
	Name    string `yaml:"name,omitempty"`
	Path    string `yaml:"path,omitempty"`
	Version string `yaml:"version,omitempty"`
}

// LoadConfigFile reads one module config document from a local YAML file.
func LoadConfigFile(path string) (PlatformModuleConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return PlatformModuleConfig{}, fmt.Errorf("open module config: %w", err)
	}
	defer func() { _ = file.Close() }()

	return LoadConfig(file)
}

// LoadConfig decodes exactly one YAML document and validates its basic shape.
func LoadConfig(reader io.Reader) (PlatformModuleConfig, error) {
	decoder := yaml.NewDecoder(reader)
	decoder.KnownFields(true)
	var config PlatformModuleConfig
	if err := decoder.Decode(&config); err != nil {
		return PlatformModuleConfig{}, fmt.Errorf("decode module config: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return PlatformModuleConfig{}, fmt.Errorf("module config must contain exactly one YAML document")
		}
		return PlatformModuleConfig{}, fmt.Errorf("decode trailing YAML: %w", err)
	}
	if err := config.Validate(); err != nil {
		return PlatformModuleConfig{}, err
	}
	return config, nil
}

// Validate checks the module identity and static configuration fields.
func (config PlatformModuleConfig) Validate() error {
	if config.APIVersion != ConfigAPIVersion || config.Kind != ConfigKind {
		return fmt.Errorf("expected %s %s, got %s %s", ConfigAPIVersion, ConfigKind, config.APIVersion, config.Kind)
	}
	if errs := validation.IsDNS1123Subdomain(config.Metadata.Name); len(errs) > 0 {
		return fmt.Errorf("invalid metadata.name %q: %s", config.Metadata.Name, strings.Join(errs, ", "))
	}
	if _, err := schema.ParseGroupVersion(config.Spec.ModuleRef.APIVersion); err != nil {
		return fmt.Errorf("invalid spec.moduleRef.apiVersion: %w", err)
	}
	if config.Spec.ModuleRef.Kind == "" {
		return fmt.Errorf("spec.moduleRef.kind is required")
	}
	if errs := validation.IsDNS1123Subdomain(config.Spec.ModuleRef.Name); len(errs) > 0 {
		return fmt.Errorf("invalid spec.moduleRef.name %q: %s", config.Spec.ModuleRef.Name, strings.Join(errs, ", "))
	}
	if config.Spec.Runlevel < 0 {
		return fmt.Errorf("spec.runlevel must be non-negative")
	}
	if err := uniqueNonEmpty(config.Spec.RelatedImages, "spec.relatedImages"); err != nil {
		return err
	}
	return uniqueNonEmpty(config.Spec.Services, "spec.services")
}

func uniqueNonEmpty(values []string, field string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s contains an empty value", field)
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("%s contains duplicate %q", field, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}
