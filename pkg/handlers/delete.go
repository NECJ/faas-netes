// Copyright (c) Alex Ellis 2017. All rights reserved.
// Copyright 2020 OpenFaaS Author(s)
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/openfaas/faas-provider/types"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// MakeDeleteHandler delete a function
func MakeDeleteHandler(defaultNamespace string, clientset *kubernetes.Clientset) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Hypervisor only supports a single function currently
		// TODO: Support deleting functions
		w.WriteHeader(http.StatusAccepted)
	}
}

func isFunction(deployment *appsv1.Deployment) bool {
	if deployment != nil {
		if _, found := deployment.Labels["faas_function"]; found {
			return true
		}
	}
	return false
}

func deleteFunction(functionNamespace string, clientset *kubernetes.Clientset, request types.DeleteFunctionRequest, w http.ResponseWriter) error {
	foregroundPolicy := metav1.DeletePropagationForeground
	opts := &metav1.DeleteOptions{PropagationPolicy: &foregroundPolicy}

	if deployErr := clientset.AppsV1().Deployments(functionNamespace).
		Delete(context.TODO(), request.FunctionName, *opts); deployErr != nil {

		if errors.IsNotFound(deployErr) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Write([]byte(deployErr.Error()))
		return fmt.Errorf("error deleting function's deployment")
	}

	if svcErr := clientset.CoreV1().
		Services(functionNamespace).
		Delete(context.TODO(), request.FunctionName, *opts); svcErr != nil {

		if errors.IsNotFound(svcErr) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}

		w.Write([]byte(svcErr.Error()))
		return fmt.Errorf("error deleting function's service")
	}
	return nil
}
