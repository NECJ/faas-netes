// Copyright 2020 OpenFaaS Author(s)
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package handlers

import (
	"net/http"

	"github.com/openfaas/faas-netes/pkg/k8s"
	"k8s.io/client-go/kubernetes"
)

// MakeSecretHandler makes a handler for Create/List/Delete/Update of
// secrets in the Kubernetes API
func MakeSecretHandler(defaultNamespace string, kube kubernetes.Interface) http.HandlerFunc {
	handler := SecretsHandler{
		LookupNamespace: NewNamespaceResolver(defaultNamespace, kube),
		Secrets:         k8s.NewSecretsClient(kube),
	}
	return handler.ServeHTTP
}

// SecretsHandler enabling to create openfaas secrets across namespaces
type SecretsHandler struct {
	Secrets         k8s.SecretsClient
	LookupNamespace NamespaceResolver
}

func (h SecretsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
}
