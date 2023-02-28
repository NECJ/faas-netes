package logs

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Requester submits queries the logging system.
// This will be passed to the log handler constructor.
type Requester interface {
	// Query submits a log request to the actual logging system.
	Query(context.Context, Request) (<-chan Message, error)
}

// NewLogHandlerFunc creates an http HandlerFunc from the supplied log Requestor.
func NewLogHandlerFunc(requestor Requester, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		return
	}
}

// parseRequest extracts the logRequest from the GET variables or from the POST body
func parseRequest(r *http.Request) (logRequest Request, err error) {
	query := r.URL.Query()
	logRequest.Name = getValue(query, "name")
	logRequest.Namespace = getValue(query, "namespace")
	logRequest.Instance = getValue(query, "instance")
	tailStr := getValue(query, "tail")
	if tailStr != "" {
		logRequest.Tail, err = strconv.Atoi(tailStr)
		if err != nil {
			return logRequest, err
		}
	}

	// ignore error because it will default to false if we can't parse it
	logRequest.Follow, _ = strconv.ParseBool(getValue(query, "follow"))

	sinceStr := getValue(query, "since")
	if sinceStr != "" {
		since, err := time.Parse(time.RFC3339, sinceStr)
		logRequest.Since = &since
		if err != nil {
			return logRequest, err
		}
	}

	return logRequest, nil
}

// getValue returns the value for the given key. If the key has more than one value, it returns the
// last value. if the value does not exist, it returns the empty string.
func getValue(queryValues url.Values, name string) string {
	values := queryValues[name]
	if len(values) == 0 {
		return ""
	}

	return values[len(values)-1]
}
