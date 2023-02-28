// Copyright (c) Alex Ellis 2017. All rights reserved.
// Copyright 2020 OpenFaaS Author(s)
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/openfaas/faas-netes/pkg/k8s"
	types "github.com/openfaas/faas-provider/types"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/client-go/listers/apps/v1"
	glog "k8s.io/klog"
)

// MakeReplicaReader reads the amount of replicas for a deployment
func MakeReplicaReader(defaultNamespace string, lister v1.DeploymentLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		function := types.FunctionStatus{
			Name:              "calc-pi",
			Replicas:          1,
			Image:             "None",
			AvailableReplicas: 1,
			InvocationCount:   0,
			Labels:            &(map[string]string{}),
			Annotations:       &(map[string]string{}),
			Namespace:         "openfaas",
			Secrets:           []string{},
			CreatedAt:         time.Now(),
		}

		functionBytes, err := json.Marshal(function)
		if err != nil {
			glog.Errorf("Failed to marshal function: %s", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Failed to marshal function"))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(functionBytes)
	}
}

// getService returns a function/service or nil if not found
func getService(functionNamespace string, functionName string, lister v1.DeploymentLister) (*types.FunctionStatus, error) {

	item, err := lister.Deployments(functionNamespace).
		Get(functionName)

	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	if item != nil {
		function := k8s.AsFunctionStatus(*item)
		if function != nil {
			return function, nil
		}
	}

	return nil, fmt.Errorf("function: %s not found", functionName)
}
