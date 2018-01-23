package builder

import (
	"errors"
	"os"
	"strings"

	"github.com/namely/k8s-pipeliner/pipeline/builder/types"

	"k8s.io/api/apps/v1beta2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var (
	// ErrUnsupportedManifest is returned when a given kubernetes manifest file
	// is not supported
	ErrUnsupportedManifest = errors.New("builder: manifest type is not supported")
)

const (
	// SpinnakerImageDescriptionAccountAnnotation is used for injecting in the docker registry
	// that should be used when generating the imageDescription struct on a container
	// field. This should match a docker registry account you've added to spinnaker
	SpinnakerImageDescriptionAccountAnnotation = "namely.com/spinnaker-image-description-account"

	// SpinnakerImageDescriptionImageIDAnnotation represents the whole repository
	// Example: registry.namely.com/namely/namely:latest
	SpinnakerImageDescriptionImageIDAnnotation = "namely.com/spinnaker-image-description-imageid"

	// SpinnakerImageDescriptionRegistryAnnotation is the registry host
	// Example: registry.namely.com
	SpinnakerImageDescriptionRegistryAnnotation = "namely.com/spinnaker-image-description-registry"

	// SpinnakerImageDescriptionRepositoryAnnotation is the user / repository name
	// Example: "namely/namely"
	SpinnakerImageDescriptionRepositoryAnnotation = "namely.com/spinnaker-image-description-repository"

	// SpinnakerImageDescriptionTagAnnotation is the tag portion of the image ID
	// Example: "latest"
	SpinnakerImageDescriptionTagAnnotation = "namely.com/spinnaker-image-description-tag"

	// SpinnakerLoadBalancersAnnotations is a comma separated list of load balancers
	// defined in Spinnaker that should be attached to a cluster
	// Example: "catalog,catalog-public"
	SpinnakerLoadBalancersAnnotations = "namely.com/spinnaker-load-balancers"
)

// ManifestGroup keeps a collection of containers from a deployment
// and metadata associated with them
type ManifestGroup struct {
	Namespace   string
	Annotations map[string]string
	Containers  []*types.Container
}

// ContainersFromManifest loads a kubernetes manifest file and generates
// spinnaker pipeline containers config from it.
//
// NOTE: If your manifest file declares multiple types, only the first will be taken
// to generate the config.
func ContainersFromManifest(file string) (*ManifestGroup, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}

	d := yaml.NewYAMLOrJSONDecoder(f, 4096)

	ext := runtime.RawExtension{}
	if dErr := d.Decode(&ext); dErr != nil {
		return nil, dErr
	}

	versions := &runtime.VersionedObjects{}
	_, gvk, err := unstructured.UnstructuredJSONScheme.Decode(ext.Raw, nil, versions)
	if err != nil {
		return nil, err
	}

	// seek back to the beginning of the file so we can unmarshal into the correct
	// type once we've determined it
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}
	var mg ManifestGroup

	switch gvk.Kind {
	case "Deployment":
		dep := &v1beta2.Deployment{}
		if err := d.Decode(dep); err != nil {
			return nil, err
		}

		mg.Containers = deploymentContainers(dep)
		mg.Annotations = dep.Annotations
		mg.Namespace = dep.Namespace
	default:
		return nil, ErrUnsupportedManifest
	}

	return &mg, nil
}

func deploymentContainers(dep *v1beta2.Deployment) []*types.Container {
	var c []*types.Container
	for _, container := range dep.Spec.Template.Spec.Containers {
		spinContainer := &types.Container{}

		// add the image description first off using the annotations on the container
		spinContainer.ImageDescription = types.ImageDescription{
			Account:      dep.Annotations[SpinnakerImageDescriptionAccountAnnotation],
			ImageID:      dep.Annotations[SpinnakerImageDescriptionImageIDAnnotation],
			Tag:          dep.Annotations[SpinnakerImageDescriptionTagAnnotation],
			Repository:   dep.Annotations[SpinnakerImageDescriptionRepositoryAnnotation],
			Registry:     dep.Annotations[SpinnakerImageDescriptionRegistryAnnotation],
			Organization: "namely",
		}

		args := []string{}
		if container.Args != nil {
			args = container.Args
		}

		spinContainer.Name = container.Name
		spinContainer.Args = args
		spinContainer.Command = container.Command
		spinContainer.ImagePullPolicy = strings.ToUpper(string(container.ImagePullPolicy))
		spinContainer.Requests.CPU = container.Resources.Requests.Cpu().String()
		spinContainer.Requests.Memory = container.Resources.Requests.Memory().String()
		spinContainer.Limits.CPU = container.Resources.Limits.Cpu().String()
		spinContainer.Limits.Memory = container.Resources.Limits.Memory().String()

		// appends all of the ports on the deployment type into the spinnaker definition
		for _, port := range container.Ports {
			spinContainer.Ports = append(spinContainer.Ports, types.Port{
				ContainerPort: port.ContainerPort,
				Name:          port.Name,
				Protocol:      string(port.Protocol),
			})
		}

		// appends all of the environment variables on the deployment type into the spinnaker definition
		for _, env := range container.Env {
			var e types.EnvVar
			e.Name = env.Name
			e.Value = env.Value

			if vf := env.ValueFrom; vf != nil {
				if vf.ConfigMapKeyRef != nil {
					e.EnvSource = &types.EnvSource{
						ConfigMapSource: &types.ConfigMapSource{
							ConfigMapName: vf.ConfigMapKeyRef.Name,
							Key:           vf.ConfigMapKeyRef.Key,
						},
					}
				}

				if vf.SecretKeyRef != nil {
					e.EnvSource = &types.EnvSource{
						SecretSource: &types.SecretSource{
							Key:        vf.SecretKeyRef.Key,
							SecretName: vf.SecretKeyRef.Name,
						},
					}
				}
			}

			spinContainer.EnvVars = append(spinContainer.EnvVars, e)
		}

		c = append(c, spinContainer)
	}

	return c
}