package deployer

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestLoadConfig(t *testing.T) {
	g := NewWithT(t)
	config, err := LoadConfig(strings.NewReader(`apiVersion: deployer.opendatahub.io/v1alpha1
kind: PlatformModuleConfig
metadata:
  name: example
spec:
  moduleRef:
    apiVersion: example.io/v1
    kind: ExampleModule
    name: cluster
  config:
    nested:
      enabled: true
  services:
    - gatewayconfig
`))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(config.Metadata.Name).To(Equal("example"))
	g.Expect(config.Spec.Config["nested"]).To(Equal(map[string]any{"enabled": true}))

	_, err = LoadConfig(strings.NewReader(`apiVersion: deployer.opendatahub.io/v1alpha1
kind: PlatformModuleConfig
metadata:
  name: example
spec:
  moduleRef:
    apiVersion: example.io/v1
    kind: ExampleModule
    name: cluster
  unknown: true
`))
	g.Expect(err).To(HaveOccurred())
}

func TestConfigValidation(t *testing.T) {
	g := NewWithT(t)
	config := testConfig()
	g.Expect(config.Validate()).To(Succeed())
	config.Spec.Services = []string{"gatewayconfig", "gatewayconfig"}
	g.Expect(config.Validate()).To(MatchError(ContainSubstring("duplicate")))
	config = testConfig()
	config.Spec.Runlevel = -1
	g.Expect(config.Validate()).To(MatchError(ContainSubstring("runlevel")))
}

func TestPlatformModuleConfigClone(t *testing.T) {
	g := NewWithT(t)
	config := testConfig()
	config.Spec.Config["nested"] = map[string]any{
		"items": []any{map[string]any{"name": "original"}},
	}

	clone := config.Clone()
	clone.Spec.RelatedImages[0] = "RELATED_IMAGE_OTHER"
	clone.Spec.Services[0] = "auth"
	clone.Spec.Config["key"] = "changed"
	clone.Spec.Config["nested"].(map[string]any)["items"].([]any)[0].(map[string]any)["name"] = "changed"

	g.Expect(config.Spec.RelatedImages).To(Equal([]string{"RELATED_IMAGE_FOO"}))
	g.Expect(config.Spec.Services).To(Equal([]string{"gatewayconfig"}))
	g.Expect(config.Spec.Config["key"]).To(Equal("value"))
	g.Expect(config.Spec.Config["nested"]).To(Equal(map[string]any{
		"items": []any{map[string]any{"name": "original"}},
	}))
	g.Expect(PlatformModuleConfig{}.Clone()).To(Equal(PlatformModuleConfig{}))
}
