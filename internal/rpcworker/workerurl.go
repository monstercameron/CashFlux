// SPDX-License-Identifier: MIT

package rpcworker

import (
	"errors"
	"net/url"
)

func workerURLFromBase(raw string) (string, error) {
	base, err := url.Parse(raw)
	if err != nil || base == nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return "", errors.New("rpc worker: document base URL is invalid")
	}
	return base.ResolveReference(&url.URL{Path: "services-worker.js"}).String(), nil
}
