package deployer

import (
	"context"
	"maps"

	kitmaps "github.com/k8s-manifest-kit/pkg/util/maps"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/option"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResolverFunc retrieves a service's chart-facing status. It is responsible
// for excluding conditions, phase, observedGeneration, and other unwanted data.
type ResolverFunc func(context.Context, client.Client) (map[string]any, error)

// Options configures a deployer. It can itself be passed to For as an Option.
type Options struct {
	ChartsPath  string
	Namespace   string
	Resolvers   map[string]ResolverFunc
	Labels      map[string]string
	Annotations map[string]string
	FieldOwner  string
}

// Option configures For.
type Option = option.Option[Options]

// ApplyTo merges this struct into target. Empty scalar fields are ignored.
func (o Options) ApplyTo(target *Options) {
	if o.ChartsPath != "" {
		target.ChartsPath = o.ChartsPath
	}
	if o.Namespace != "" {
		target.Namespace = o.Namespace
	}
	mergeResolvers(&target.Resolvers, o.Resolvers)
	mergeStrings(&target.Labels, o.Labels)
	mergeStrings(&target.Annotations, o.Annotations)
	if o.FieldOwner != "" {
		target.FieldOwner = o.FieldOwner
	}
}

// WithChartsPath sets the root for local module charts.
func WithChartsPath(path string) Option {
	return option.FunctionalOption[Options](func(o *Options) { o.ChartsPath = path })
}

// WithNamespace sets the Helm release namespace.
func WithNamespace(namespace string) Option {
	return option.FunctionalOption[Options](func(o *Options) { o.Namespace = namespace })
}

// Resolver registers a service resolver at construction time.
func Resolver(name string, resolve ResolverFunc) Option {
	return option.FunctionalOption[Options](func(o *Options) {
		if o.Resolvers == nil {
			o.Resolvers = make(map[string]ResolverFunc)
		}
		o.Resolvers[name] = resolve
	})
}

// WithLabels merges labels into constructor defaults.
func WithLabels(values map[string]string) Option {
	return option.FunctionalOption[Options](func(o *Options) { mergeStrings(&o.Labels, values) })
}

// WithAnnotations merges annotations into constructor defaults.
func WithAnnotations(values map[string]string) Option {
	return option.FunctionalOption[Options](func(o *Options) { mergeStrings(&o.Annotations, values) })
}

// WithFieldOwner sets the default server-side apply field manager.
func WithFieldOwner(value string) Option {
	return option.FunctionalOption[Options](func(o *Options) { o.FieldOwner = value })
}

// RunOptions configures one Run invocation. The current deploy action requires Owner.
type RunOptions struct {
	Owner client.Object
	// Values are merged over Helm source defaults; resolved service fields win.
	Values      map[string]any
	Labels      map[string]string
	Annotations map[string]string
	FieldOwner  string
}

// RunOption configures Run.
type RunOption = option.Option[RunOptions]

// ApplyTo merges this struct into target. Empty scalar fields are ignored.
func (o RunOptions) ApplyTo(target *RunOptions) {
	if o.Owner != nil {
		target.Owner = o.Owner
	}
	mergeValues(&target.Values, o.Values)
	mergeStrings(&target.Labels, o.Labels)
	mergeStrings(&target.Annotations, o.Annotations)
	if o.FieldOwner != "" {
		target.FieldOwner = o.FieldOwner
	}
}

// RunWithOwner sets the owner for one Run invocation.
func RunWithOwner(owner client.Object) RunOption {
	return option.FunctionalOption[RunOptions](func(o *RunOptions) { o.Owner = owner })
}

// RunWithValues merges additional Helm values for one Run invocation.
func RunWithValues(values map[string]any) RunOption {
	return option.FunctionalOption[RunOptions](func(o *RunOptions) { mergeValues(&o.Values, values) })
}

// RunWithLabels merges labels for one Run invocation.
func RunWithLabels(values map[string]string) RunOption {
	return option.FunctionalOption[RunOptions](func(o *RunOptions) { mergeStrings(&o.Labels, values) })
}

// RunWithAnnotations merges annotations for one Run invocation.
func RunWithAnnotations(values map[string]string) RunOption {
	return option.FunctionalOption[RunOptions](func(o *RunOptions) { mergeStrings(&o.Annotations, values) })
}

// RunWithFieldOwner overrides the field manager for one Run invocation.
func RunWithFieldOwner(value string) RunOption {
	return option.FunctionalOption[RunOptions](func(o *RunOptions) { o.FieldOwner = value })
}

func mergeStrings(target *map[string]string, source map[string]string) {
	if len(source) == 0 {
		return
	}
	if *target == nil {
		*target = make(map[string]string, len(source))
	}
	maps.Copy(*target, source)
}

func mergeValues(target *map[string]any, source map[string]any) {
	if source != nil {
		*target = kitmaps.DeepMerge(*target, source)
	}
}

func mergeResolvers(target *map[string]ResolverFunc, source map[string]ResolverFunc) {
	if len(source) == 0 {
		return
	}
	if *target == nil {
		*target = make(map[string]ResolverFunc, len(source))
	}
	maps.Copy(*target, source)
}
