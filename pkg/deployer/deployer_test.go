package deployer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/gomega"
	platformannotations "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata/annotations"
	platformlabels "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata/labels"
)

const chartTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: rendered
  namespace: {{ .Release.Namespace }}
data:
  image: {{ .Values.images.RELATED_IMAGE_FOO | quote }}
  config: {{ .Values.config.key | quote }}
  ready: {{ .Values.services.gatewayconfig.ready | quote }}
`

func testConfig() PlatformModuleConfig {
	return PlatformModuleConfig{
		APIVersion: ConfigAPIVersion,
		Kind:       ConfigKind,
		Metadata:   ModuleMetadata{Name: "example"},
		Spec: ModuleSpec{
			ModuleRef:     ModuleRef{APIVersion: "example.io/v1", Kind: "ExampleModule", Name: "cluster"},
			RelatedImages: []string{"RELATED_IMAGE_FOO"},
			Config:        map[string]any{"key": "value"},
			Services:      []string{"gatewayconfig"},
		},
	}
}

func writeTestChart(t *testing.T, template string) string {
	t.Helper()
	root := t.TempDir()
	chart := filepath.Join(root, "example")
	if err := os.MkdirAll(filepath.Join(chart, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chart, "Chart.yaml"), []byte("apiVersion: v2\nname: example\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chart, "templates", "resource.yaml"), []byte(template), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func testResolver(_ context.Context, _ client.Client) (map[string]any, error) {
	return map[string]any{"ready": "yes"}, nil
}

func testClient(t *testing.T) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := rbacv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).Build()
}

func TestForInitializesMetadataMaps(t *testing.T) {
	g := NewWithT(t)
	root := writeTestChart(t, chartTemplate)
	t.Setenv("RELATED_IMAGE_FOO", "registry/image:1")
	d, err := For(testConfig(), Options{
		ChartsPath: root,
		Namespace:  "module-system",
		Resolvers:  map[string]ResolverFunc{"gatewayconfig": testResolver},
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(d.options.Labels).NotTo(BeNil())
	g.Expect(d.options.Annotations).NotTo(BeNil())
	g.Expect(d.options.Labels).To(BeEmpty())
	g.Expect(d.options.Annotations).To(BeEmpty())
}

func TestMergeMetadata(t *testing.T) {
	g := NewWithT(t)
	base := map[string]string{"shared": "base", "base": "keep"}
	overrides := map[string]string{"shared": "override", "override": "add"}
	merged := mergeMetadata(base, overrides)
	g.Expect(merged).To(Equal(map[string]string{
		"shared":   "override",
		"base":     "keep",
		"override": "add",
	}))
	merged["base"] = "changed"
	g.Expect(base["base"]).To(Equal("keep"))
	g.Expect(mergeMetadata(nil, nil)).NotTo(BeNil())
}

func TestForCapturesImagesAndRendererCachesChart(t *testing.T) {
	g := NewWithT(t)
	root := writeTestChart(t, chartTemplate)
	t.Setenv("RELATED_IMAGE_FOO", "registry/first:1")
	config := testConfig()
	ready := "yes"
	d, err := For(config, Options{
		ChartsPath: root,
		Namespace:  "module-system",
		Resolvers: map[string]ResolverFunc{"gatewayconfig": func(context.Context, client.Client) (map[string]any, error) {
			return map[string]any{"ready": ready}, nil
		}},
	})
	g.Expect(err).NotTo(HaveOccurred())
	config.Spec.Config["key"] = "changed by caller"
	t.Setenv("RELATED_IMAGE_FOO", "registry/second:2")
	cli := testClient(t)
	objects, err := d.Run(t.Context(), cli, RunOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(objects).To(HaveLen(1))
	g.Expect(objects[0].GetName()).To(Equal("rendered"))
	g.Expect(objects[0].Object["data"]).To(HaveKeyWithValue("image", "registry/first:1"))
	g.Expect(objects[0].Object["data"]).To(HaveKeyWithValue("config", "value"))
	g.Expect(objects[0].Object["data"]).To(HaveKeyWithValue("ready", "yes"))

	g.Expect(os.WriteFile(filepath.Join(root, "example", "templates", "resource.yaml"), []byte(strings.ReplaceAll(chartTemplate, "rendered", "changed")), 0o644)).To(Succeed())
	ready = "no"
	objects, err = d.Run(t.Context(), cli, RunOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(objects[0].GetName()).To(Equal("rendered"))
	g.Expect(objects[0].Object["data"]).To(HaveKeyWithValue("ready", "no"))
}

func TestRunAdditionalValues(t *testing.T) {
	g := NewWithT(t)
	template := strings.Replace(chartTemplate, "  ready:", "  extra: {{ .Values.extra | quote }}\n  note: {{ .Values.services.gatewayconfig.note | quote }}\n  ready:", 1)
	root := writeTestChart(t, template)
	t.Setenv("RELATED_IMAGE_FOO", "registry/image:1")
	d, err := For(testConfig(), Options{ChartsPath: root, Namespace: "module-system", Resolvers: map[string]ResolverFunc{"gatewayconfig": testResolver}})
	g.Expect(err).NotTo(HaveOccurred())
	values := map[string]any{
		"config":   map[string]any{"key": "run override"},
		"extra":    "struct",
		"services": map[string]any{"gatewayconfig": map[string]any{"note": "added", "ready": "wrong"}},
	}
	objects, err := d.Run(t.Context(), testClient(t), RunOptions{Values: values}, RunWithValues(map[string]any{"extra": "functional"}))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(objects).To(HaveLen(1))
	g.Expect(objects[0].Object["data"]).To(HaveKeyWithValue("config", "run override"))
	g.Expect(objects[0].Object["data"]).To(HaveKeyWithValue("image", "registry/image:1"))
	g.Expect(objects[0].Object["data"]).To(HaveKeyWithValue("ready", "yes"))
	g.Expect(objects[0].Object["data"]).To(HaveKeyWithValue("note", "added"))
	g.Expect(objects[0].Object["data"]).To(HaveKeyWithValue("extra", "functional"))
	g.Expect(values["services"].(map[string]any)["gatewayconfig"].(map[string]any)["ready"]).To(Equal("wrong"))
}

func TestChartReleaseName(t *testing.T) {
	root := writeTestChart(t, "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: {{ .Release.Name }}\n  namespace: {{ .Release.Namespace }}\n")
	t.Setenv("RELATED_IMAGE_FOO", "registry/image:1")
	for _, test := range []struct {
		name     string
		chart    string
		expected string
	}{
		{name: "defaults to module name", expected: "example"},
		{name: "uses chart name", chart: "custom-release", expected: "custom-release"},
	} {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			config := testConfig()
			config.Spec.Chart.Name = test.chart
			d, err := For(config, Options{ChartsPath: root, Namespace: "module-system", Resolvers: map[string]ResolverFunc{"gatewayconfig": testResolver}})
			g.Expect(err).NotTo(HaveOccurred())
			objects, err := d.Run(t.Context(), testClient(t))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(objects[0].GetName()).To(Equal(test.expected))
		})
	}
}

func TestForValidatesInputs(t *testing.T) {
	g := NewWithT(t)
	root := writeTestChart(t, chartTemplate)
	t.Setenv("RELATED_IMAGE_FOO", "registry/image:1")
	_, err := For(testConfig(), Options{ChartsPath: root, Namespace: "module-system"})
	g.Expect(err).To(MatchError(ContainSubstring("resolver")))

	config := testConfig()
	config.Spec.Chart.Path = "../outside"
	_, err = For(config, Options{ChartsPath: root, Namespace: "module-system", Resolvers: map[string]ResolverFunc{"gatewayconfig": testResolver}})
	g.Expect(err).To(MatchError(ContainSubstring("local and relative")))

	t.Setenv("RELATED_IMAGE_FOO", "")
	_, err = For(testConfig(), Options{ChartsPath: root, Namespace: "module-system", Resolvers: map[string]ResolverFunc{"gatewayconfig": testResolver}})
	g.Expect(err).To(MatchError(ContainSubstring("RELATED_IMAGE_FOO")))
}

func TestChartLoadIsManagedByRenderer(t *testing.T) {
	g := NewWithT(t)
	root := writeTestChart(t, chartTemplate)
	g.Expect(os.WriteFile(filepath.Join(root, "example", "Chart.yaml"), []byte("not a helm chart\n"), 0o644)).To(Succeed())
	t.Setenv("RELATED_IMAGE_FOO", "registry/image:1")
	d, err := For(testConfig(), Options{ChartsPath: root, Namespace: "module-system", Resolvers: map[string]ResolverFunc{"gatewayconfig": testResolver}})
	g.Expect(err).NotTo(HaveOccurred())
	_, err = d.Run(t.Context(), testClient(t))
	g.Expect(err).To(HaveOccurred())
}

func TestOptionsComposeAndRunMetadataWins(t *testing.T) {
	g := NewWithT(t)
	root := writeTestChart(t, chartTemplate)
	t.Setenv("RELATED_IMAGE_FOO", "registry/image:1")
	d, err := For(testConfig(),
		Options{ChartsPath: root, Namespace: "module-system", Labels: map[string]string{"shared": "constructor", "other": "keep"}},
		Resolver("gatewayconfig", testResolver),
		WithLabels(map[string]string{"constructor": "yes"}),
	)
	g.Expect(err).NotTo(HaveOccurred())
	objects, err := d.Run(t.Context(), testClient(t),
		RunOptions{Labels: map[string]string{"shared": "run"}},
		RunWithAnnotations(map[string]string{"run": "yes"}),
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(objects[0].GetLabels()).To(HaveKeyWithValue("shared", "run"))
	g.Expect(objects[0].GetLabels()).To(HaveKeyWithValue("other", "keep"))
	g.Expect(objects[0].GetLabels()).To(HaveKeyWithValue("constructor", "yes"))
	g.Expect(objects[0].GetAnnotations()).To(HaveKeyWithValue("run", "yes"))
	g.Expect(d.options.Labels).To(HaveKeyWithValue("shared", "constructor"))
}

func TestRunOwnedAndUnowned(t *testing.T) {
	g := NewWithT(t)
	root := writeTestChart(t, chartTemplate)
	t.Setenv("RELATED_IMAGE_FOO", "registry/image:1")
	d, err := For(testConfig(), Options{
		ChartsPath:  root,
		Namespace:   "module-system",
		Resolvers:   map[string]ResolverFunc{"gatewayconfig": testResolver},
		Labels:      map[string]string{"shared": "constructor", "constructor": "yes"},
		Annotations: map[string]string{"shared": "constructor"},
	})
	g.Expect(err).NotTo(HaveOccurred())
	owner := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "owner", UID: "owner-uid"}}
	owner.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Namespace"))
	cli := testClient(t)
	_, err = d.Run(t.Context(), cli, RunOptions{
		Owner:       owner,
		Labels:      map[string]string{"shared": "run"},
		Annotations: map[string]string{"shared": "run"},
	})
	g.Expect(err).NotTo(HaveOccurred())
	live := &corev1.ConfigMap{}
	g.Expect(cli.Get(t.Context(), client.ObjectKey{Name: "rendered", Namespace: "module-system"}, live)).To(Succeed())
	g.Expect(live.GetOwnerReferences()).To(HaveLen(1))
	g.Expect(live.GetLabels()).To(HaveKey(platformlabels.PlatformPartOf))
	g.Expect(live.GetLabels()).To(HaveKeyWithValue("shared", "run"))
	g.Expect(live.GetLabels()).To(HaveKeyWithValue("constructor", "yes"))
	g.Expect(live.GetAnnotations()).To(HaveKeyWithValue("shared", "run"))
	_, err = d.Run(t.Context(), cli, RunOptions{Owner: owner})
	g.Expect(err).NotTo(HaveOccurred())

	objects, err := d.Run(t.Context(), cli, RunOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(objects).To(HaveLen(1))
	g.Expect(objects[0].GetOwnerReferences()).To(BeEmpty())
	g.Expect(cli.Get(t.Context(), client.ObjectKey{Name: "rendered", Namespace: "module-system"}, live)).To(Succeed())
	g.Expect(live.GetOwnerReferences()).To(BeEmpty())
	g.Expect(live.GetLabels()).NotTo(HaveKey(platformlabels.PlatformPartOf))
	g.Expect(live.GetLabels()).To(HaveKeyWithValue("shared", "constructor"))
	g.Expect(live.GetAnnotations()).To(HaveKeyWithValue("shared", "constructor"))
	g.Expect(live.GetAnnotations()).NotTo(HaveKey(platformannotations.InstanceUID))
}

func TestClusterScopedOwnerOwnsClusterScopedObjects(t *testing.T) {
	g := NewWithT(t)
	root := writeTestChart(t, chartTemplate+`---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: rendered-role
rules: []
`)
	t.Setenv("RELATED_IMAGE_FOO", "registry/image:1")
	d, err := For(testConfig(), Options{ChartsPath: root, Namespace: "module-system", Resolvers: map[string]ResolverFunc{"gatewayconfig": testResolver}})
	g.Expect(err).NotTo(HaveOccurred())
	owner := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "owner", UID: "owner-uid"}}
	owner.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Namespace"))
	cli := testClient(t)
	objects, err := d.Run(t.Context(), cli, RunOptions{Owner: owner})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(objects).To(HaveLen(2))
	role := &rbacv1.ClusterRole{}
	g.Expect(cli.Get(t.Context(), client.ObjectKey{Name: "rendered-role"}, role)).To(Succeed())
	g.Expect(role.GetOwnerReferences()).To(HaveLen(1))
}

func TestRunDeploysModuleCRDButNotModuleCR(t *testing.T) {
	g := NewWithT(t)
	root := writeTestChart(t, `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: examplemodules.example.io
spec:
  group: example.io
  names:
    kind: ExampleModule
    plural: examplemodules
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
`)
	t.Setenv("RELATED_IMAGE_FOO", "registry/image:1")
	d, err := For(testConfig(), Options{ChartsPath: root, Namespace: "module-system", Resolvers: map[string]ResolverFunc{"gatewayconfig": testResolver}})
	g.Expect(err).NotTo(HaveOccurred())
	cli := testClient(t)
	objects, err := d.Run(t.Context(), cli)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(objects).To(HaveLen(1))
	g.Expect(objects[0].GetKind()).To(Equal("CustomResourceDefinition"))
	live := &apiextensionsv1.CustomResourceDefinition{}
	g.Expect(cli.Get(t.Context(), client.ObjectKey{Name: "examplemodules.example.io"}, live)).To(Succeed())
}

func TestRunFiltersModuleCRGVK(t *testing.T) {
	g := NewWithT(t)
	root := writeTestChart(t, chartTemplate+`---
apiVersion: example.io/v1
kind: ExampleModule
metadata:
  name: cluster
---
apiVersion: example.io/v1
kind: ExampleModule
metadata:
  name: another
`)
	t.Setenv("RELATED_IMAGE_FOO", "registry/image:1")
	d, err := For(testConfig(), Options{ChartsPath: root, Namespace: "module-system", Resolvers: map[string]ResolverFunc{"gatewayconfig": testResolver}})
	g.Expect(err).NotTo(HaveOccurred())
	cli := testClient(t)
	objects, err := d.Run(t.Context(), cli)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(objects).To(HaveLen(1))
	g.Expect(objects[0].GetKind()).To(Equal("ConfigMap"))
	live := &corev1.ConfigMap{}
	g.Expect(cli.Get(t.Context(), client.ObjectKey{Name: "rendered", Namespace: "module-system"}, live)).To(Succeed())
}

func TestRunStopsBeforeApplyOnResolverError(t *testing.T) {
	g := NewWithT(t)
	root := writeTestChart(t, chartTemplate)
	t.Setenv("RELATED_IMAGE_FOO", "registry/image:1")
	d, err := For(testConfig(), Options{
		ChartsPath: root,
		Namespace:  "module-system",
		Resolvers: map[string]ResolverFunc{
			"gatewayconfig": func(context.Context, client.Client) (map[string]any, error) {
				return nil, errors.New("service unavailable")
			},
		},
	})
	g.Expect(err).NotTo(HaveOccurred())
	cli := testClient(t)
	objects, err := d.Run(t.Context(), cli)
	g.Expect(err).To(MatchError(ContainSubstring("service unavailable")))
	g.Expect(objects).To(BeNil())
	live := &corev1.ConfigMap{}
	err = cli.Get(t.Context(), client.ObjectKey{Name: "rendered", Namespace: "module-system"}, live)
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
}

func TestRunRequiresClient(t *testing.T) {
	g := NewWithT(t)
	root := writeTestChart(t, chartTemplate)
	t.Setenv("RELATED_IMAGE_FOO", "registry/image:1")
	d, err := For(testConfig(), Options{ChartsPath: root, Namespace: "module-system", Resolvers: map[string]ResolverFunc{"gatewayconfig": testResolver}})
	g.Expect(err).NotTo(HaveOccurred())
	_, err = d.Run(t.Context(), nil)
	g.Expect(err).To(MatchError(ContainSubstring("client is required")))
}

func TestForRejectsEscapingChartSymlink(t *testing.T) {
	g := NewWithT(t)
	root := t.TempDir()
	outside := writeTestChart(t, chartTemplate)
	g.Expect(os.Symlink(filepath.Join(outside, "example"), filepath.Join(root, "example"))).To(Succeed())
	t.Setenv("RELATED_IMAGE_FOO", "registry/image:1")
	_, err := For(testConfig(), Options{ChartsPath: root, Namespace: "module-system", Resolvers: map[string]ResolverFunc{"gatewayconfig": testResolver}})
	g.Expect(err).To(MatchError(ContainSubstring("escapes charts path")))
}
