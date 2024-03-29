// Copyright (c) Alex Ellis 2017. All rights reserved.
// Copyright 2020 OpenFaaS Author(s)
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package handlers

import (
	"io/ioutil"
	"net/http"

	types "github.com/openfaas/faas-provider/types"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	v1 "k8s.io/client-go/listers/apps/v1"

	"github.com/openfaas/faas-netes/pkg/k8s"
)

// MakeFunctionReader handler for reading functions deployed in the cluster as deployments.
func MakeFunctionReader(defaultNamespace string, deploymentLister v1.DeploymentLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, _ := http.Get("http://openfaas-hypervisor-service:8080/system/functions")
		body, _ := ioutil.ReadAll(resp.Body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}
}

func getServiceList(functionNamespace string, deploymentLister v1.DeploymentLister) ([]types.FunctionStatus, error) {
	functions := []types.FunctionStatus{}

	sel := labels.NewSelector()
	req, err := labels.NewRequirement("faas_function", selection.Exists, []string{})
	if err != nil {
		return functions, err
	}
	onlyFunctions := sel.Add(*req)

	res, err := deploymentLister.Deployments(functionNamespace).List(onlyFunctions)

	if err != nil {
		return nil, err
	}

	for _, item := range res {
		if item != nil {
			function := k8s.AsFunctionStatus(*item)
			if function != nil {
				functions = append(functions, *function)
			}
		}
	}

	return functions, nil
}
