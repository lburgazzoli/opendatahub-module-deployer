package deployer

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"strings"

	manifestengine "github.com/k8s-manifest-kit/engine/pkg"
	manifestrender "github.com/k8s-manifest-kit/engine/pkg/render"
	manifesttypes "github.com/k8s-manifest-kit/engine/pkg/types"
	kitmaps "github.com/k8s-manifest-kit/pkg/util/maps"
	helm "github.com/k8s-manifest-kit/renderer-helm/pkg"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/deploy"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const defaultFieldOwner = "opendatahub-module-deployer"

// Deployer is prepared for one module and safe for repeated Run calls.
type Deployer struct {
	config   PlatformModuleConfig
	options  Options
	renderer *manifestengine.Engine
	action   *deploy.Action
}

// For prepares a module for deployment. It initializes the Helm engine and
// captures related images; chart loading is an engine implementation detail.
func For(config PlatformModuleConfig, values ...Option) (*Deployer, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	config = config.Clone()
	var opts Options
	for _, value := range values {
		if value != nil {
			value.ApplyTo(&opts)
		}
	}
	if opts.Labels == nil {
		opts.Labels = make(map[string]string)
	}
	if opts.Annotations == nil {
		opts.Annotations = make(map[string]string)
	}
	if opts.ChartsPath == "" {
		return nil, fmt.Errorf("charts path is required")
	}
	if errs := validation.IsDNS1123Label(opts.Namespace); len(errs) > 0 {
		return nil, fmt.Errorf("invalid namespace %q: %s", opts.Namespace, strings.Join(errs, ", "))
	}
	for _, service := range config.Spec.Services {
		if opts.Resolvers[service] == nil {
			return nil, fmt.Errorf("resolver for service %q is required", service)
		}
	}

	chartPath, err := loadChartPath(opts.ChartsPath, cmp.Or(config.Spec.Chart.Path, config.Metadata.Name))
	if err != nil {
		return nil, err
	}
	images, err := resolveRelatedImages(config.Spec.RelatedImages)
	if err != nil {
		return nil, err
	}
	gv, err := schema.ParseGroupVersion(config.Spec.ModuleRef.APIVersion)
	if err != nil {
		return nil, err
	}

	moduleGVK := gv.WithKind(config.Spec.ModuleRef.Kind)
	renderer, err := helm.NewEngine(helm.Source{
		Chart:            chartPath,
		ReleaseName:      cmp.Or(config.Spec.Chart.Name, config.Metadata.Name),
		ReleaseNamespace: opts.Namespace,
		ReleaseVersion:   config.Spec.Chart.Version,
		Values: helm.Values(map[string]any{
			"images": images,
			"config": config.Spec.Config,
		}),
	}, helm.WithFilter(func(_ context.Context, object unstructured.Unstructured) (bool, error) {
		return object.GroupVersionKind() != moduleGVK, nil
	}))
	if err != nil {
		return nil, fmt.Errorf("initialize chart renderer: %w", err)
	}
	action := deploy.New(deploy.WithFieldOwner(cmp.Or(opts.FieldOwner, defaultFieldOwner)))

	return &Deployer{
		config:   config,
		options:  opts,
		renderer: renderer,
		action:   action,
	}, nil
}

// Run renders and deploys the module through the deploy action. The current
// utilities-v2 action rejects a nil owner.
func (d *Deployer) Run(ctx context.Context, cli client.Client, values ...RunOption) ([]unstructured.Unstructured, error) {
	if d == nil {
		return nil, fmt.Errorf("deployer is required")
	}
	if cli == nil {
		return nil, fmt.Errorf("kubernetes client is required")
	}
	var opts RunOptions
	for _, value := range values {
		if value != nil {
			value.ApplyTo(&opts)
		}
	}
	labels := mergeMetadata(d.options.Labels, opts.Labels)
	annotations := mergeMetadata(d.options.Annotations, opts.Annotations)

	services := make(map[string]any, len(d.config.Spec.Services))
	for _, name := range d.config.Spec.Services {
		status, err := d.options.Resolvers[name](ctx, cli)
		if err != nil {
			return nil, fmt.Errorf("resolve service %q: %w", name, err)
		}
		services[name] = status
	}
	renderValues := kitmaps.DeepMerge(opts.Values, map[string]any{"services": services})
	renderOptions := []manifestrender.Option{manifestrender.WithValues(manifesttypes.Values(renderValues)), manifestrender.WithPostRenderer(func(_ context.Context, objects []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
		for i := range objects {
			resources.SetLabels(&objects[i], labels)
			resources.SetAnnotations(&objects[i], annotations)
		}
		return objects, nil
	})}
	objects, err := d.renderer.Render(ctx, renderOptions...)
	if err != nil {
		return nil, fmt.Errorf("render module chart: %w", err)
	}
	collection := resources.New(resources.List(objects))
	_, err = d.action.Run(ctx, deploy.RunOptions{
		Client: cli, Owner: opts.Owner, Resources: collection, FieldOwner: opts.FieldOwner,
	})
	if err != nil {
		return nil, fmt.Errorf("deploy module: %w", err)
	}
	return []unstructured.Unstructured(collection.Get()), nil
}

func mergeMetadata(base map[string]string, overrides map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(overrides))
	maps.Copy(merged, base)
	maps.Copy(merged, overrides)
	return merged
}
