// Copyright (c) Alex Ellis 2017. All rights reserved.
// Copyright 2020 OpenFaaS Author(s)
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package handlers

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/openfaas/faas-netes/pkg/k8s"

	types "github.com/openfaas/faas-provider/types"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// initialReplicasCount how many replicas to start of creating for a function
const initialReplicasCount = 1

// MakeDeployHandler creates a handler to create new functions in the cluster
func MakeDeployHandler(functionNamespace string, factory k8s.FunctionFactory) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Hypervisor only supports pre-compiled functions already stored on hypervisor
		// TODO: Support creating functions
		w.WriteHeader(http.StatusAccepted)
	}
}

func makeDeploymentSpec(request types.FunctionDeployment, existingSecrets map[string]*apiv1.Secret, factory k8s.FunctionFactory) (*appsv1.Deployment, error) {
	envVars := buildEnvVars(&request)

	initialReplicas := int32p(initialReplicasCount)
	labels := map[string]string{
		"faas_function": request.Service,
	}

	if request.Labels != nil {
		if min := getMinReplicaCount(*request.Labels); min != nil {
			initialReplicas = min
		}
		for k, v := range *request.Labels {
			labels[k] = v
		}
	}

	nodeSelector := createSelector(request.Constraints)

	resources, resourceErr := createResources(request)

	if resourceErr != nil {
		return nil, resourceErr
	}

	var imagePullPolicy apiv1.PullPolicy
	switch factory.Config.ImagePullPolicy {
	case "Never":
		imagePullPolicy = apiv1.PullNever
	case "IfNotPresent":
		imagePullPolicy = apiv1.PullIfNotPresent
	default:
		imagePullPolicy = apiv1.PullAlways
	}

	annotations, err := buildAnnotations(request)
	if err != nil {
		return nil, err
	}

	probes, err := factory.MakeProbes(request)
	if err != nil {
		return nil, err
	}

	deploymentSpec := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        request.Service,
			Annotations: annotations,
			Labels: map[string]string{
				"faas_function": request.Service,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"faas_function": request.Service,
				},
			},
			Replicas: initialReplicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: int32(0),
					},
					MaxSurge: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: int32(1),
					},
				},
			},
			RevisionHistoryLimit: int32p(10),
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:        request.Service,
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: apiv1.PodSpec{
					NodeSelector: nodeSelector,
					Containers: []apiv1.Container{
						{
							Name:  request.Service,
							Image: request.Image,
							Ports: []apiv1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: factory.Config.RuntimeHTTPPort,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env:             envVars,
							Resources:       *resources,
							ImagePullPolicy: imagePullPolicy,
							LivenessProbe:   probes.Liveness,
							ReadinessProbe:  probes.Readiness,
							SecurityContext: &corev1.SecurityContext{
								ReadOnlyRootFilesystem: &request.ReadOnlyRootFilesystem,
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyAlways,
					DNSPolicy:     corev1.DNSClusterFirst,
				},
			},
		},
	}

	factory.ConfigureReadOnlyRootFilesystem(request, deploymentSpec)
	factory.ConfigureContainerUserID(deploymentSpec)

	if err := factory.ConfigureSecrets(request, deploymentSpec, existingSecrets); err != nil {
		return nil, err
	}

	return deploymentSpec, nil
}

func makeServiceSpec(request types.FunctionDeployment, factory k8s.FunctionFactory) (*corev1.Service, error) {
	annotations, err := buildAnnotations(request)
	if err != nil {
		return nil, err
	}

	serviceSpec := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        request.Service,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"faas_function": request.Service,
			},
			Ports: []corev1.ServicePort{
				{
					Name:     "http",
					Protocol: corev1.ProtocolTCP,
					Port:     factory.Config.RuntimeHTTPPort,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: factory.Config.RuntimeHTTPPort,
					},
				},
			},
		},
	}

	return serviceSpec, nil
}

func buildAnnotations(request types.FunctionDeployment) (map[string]string, error) {
	var annotations map[string]string
	if request.Annotations != nil {
		annotations = *request.Annotations
	} else {
		annotations = map[string]string{}
	}

	if v, ok := annotations["topic"]; ok {
		if strings.Contains(v, ",") {
			return nil, fmt.Errorf("the topic annotation may only support one value in the Community Edition")
		}
	}

	for k, _ := range annotations {
		if strings.Contains(k, "amazonaws.com") || strings.Contains(k, "gke.io") {
			return nil, fmt.Errorf("annotation %q is not supported in the Community Edition", k)
		}
	}

	if _, ok := annotations["prometheus.io.scrape"]; !ok {
		annotations["prometheus.io.scrape"] = "false"
	}
	return annotations, nil
}

func buildEnvVars(request *types.FunctionDeployment) []corev1.EnvVar {
	envVars := []corev1.EnvVar{}

	if len(request.EnvProcess) > 0 {
		envVars = append(envVars, corev1.EnvVar{
			Name:  k8s.EnvProcessName,
			Value: request.EnvProcess,
		})
	}

	for k, v := range request.EnvVars {
		envVars = append(envVars, corev1.EnvVar{
			Name:  k,
			Value: v,
		})
	}

	sort.SliceStable(envVars, func(i, j int) bool {
		return strings.Compare(envVars[i].Name, envVars[j].Name) == -1
	})

	return envVars
}

func int32p(i int32) *int32 {
	return &i
}

func createSelector(constraints []string) map[string]string {
	selector := make(map[string]string)

	if len(constraints) > 0 {
		for _, constraint := range constraints {
			parts := strings.Split(constraint, "=")

			if len(parts) == 2 {
				selector[parts[0]] = parts[1]
			}
		}
	}

	return selector
}

func createResources(request types.FunctionDeployment) (*apiv1.ResourceRequirements, error) {
	resources := &apiv1.ResourceRequirements{
		Limits:   apiv1.ResourceList{},
		Requests: apiv1.ResourceList{},
	}

	// Set Memory limits
	if request.Limits != nil && len(request.Limits.Memory) > 0 {
		qty, err := resource.ParseQuantity(request.Limits.Memory)
		if err != nil {
			return resources, err
		}
		resources.Limits[apiv1.ResourceMemory] = qty
	}

	if request.Requests != nil && len(request.Requests.Memory) > 0 {
		qty, err := resource.ParseQuantity(request.Requests.Memory)
		if err != nil {
			return resources, err
		}
		resources.Requests[apiv1.ResourceMemory] = qty
	}

	// Set CPU limits
	if request.Limits != nil && len(request.Limits.CPU) > 0 {
		qty, err := resource.ParseQuantity(request.Limits.CPU)
		if err != nil {
			return resources, err
		}
		resources.Limits[apiv1.ResourceCPU] = qty
	}

	if request.Requests != nil && len(request.Requests.CPU) > 0 {
		qty, err := resource.ParseQuantity(request.Requests.CPU)
		if err != nil {
			return resources, err
		}
		resources.Requests[apiv1.ResourceCPU] = qty
	}

	return resources, nil
}

func getMinReplicaCount(labels map[string]string) *int32 {
	if value, exists := labels["com.openfaas.scale.min"]; exists {
		minReplicas, err := strconv.Atoi(value)
		if err == nil && minReplicas > 0 {
			return int32p(int32(minReplicas))
		}

		log.Println(err)
	}

	return nil
}
