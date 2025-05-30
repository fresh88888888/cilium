// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package cmd

import (
	"context"
	"net/http"

	"github.com/cilium/hive/cell"
	"github.com/go-openapi/runtime/middleware"

	"github.com/cilium/cilium/api/v1/server/restapi/daemon"
	"github.com/cilium/cilium/api/v1/server/restapi/endpoint"
	"github.com/cilium/cilium/api/v1/server/restapi/policy"
	"github.com/cilium/cilium/pkg/api"
	"github.com/cilium/cilium/pkg/option"
	"github.com/cilium/cilium/pkg/promise"
)

type handlersOut struct {
	cell.Out

	DaemonGetDebuginfoHandler daemon.GetDebuginfoHandler
	DaemonGetHealthzHandler   daemon.GetHealthzHandler

	EndpointDeleteEndpointHandler        endpoint.DeleteEndpointHandler
	EndpointDeleteEndpointIDHandler      endpoint.DeleteEndpointIDHandler
	EndpointGetEndpointHandler           endpoint.GetEndpointHandler
	EndpointGetEndpointIDConfigHandler   endpoint.GetEndpointIDConfigHandler
	EndpointGetEndpointIDHandler         endpoint.GetEndpointIDHandler
	EndpointGetEndpointIDHealthzHandler  endpoint.GetEndpointIDHealthzHandler
	EndpointGetEndpointIDLabelsHandler   endpoint.GetEndpointIDLabelsHandler
	EndpointGetEndpointIDLogHandler      endpoint.GetEndpointIDLogHandler
	EndpointPatchEndpointIDConfigHandler endpoint.PatchEndpointIDConfigHandler
	EndpointPatchEndpointIDHandler       endpoint.PatchEndpointIDHandler
	EndpointPatchEndpointIDLabelsHandler endpoint.PatchEndpointIDLabelsHandler
	EndpointPutEndpointIDHandler         endpoint.PutEndpointIDHandler

	PolicyGetIdentityEndpointsHandler policy.GetIdentityEndpointsHandler
	PolicyGetIdentityHandler          policy.GetIdentityHandler
	PolicyGetIdentityIDHandler        policy.GetIdentityIDHandler
	PolicyGetIPHandler                policy.GetIPHandler
}

// apiHandler implements Handle() for the given parameter type.
// It allows expressing the API handlers requiring *Daemon as simply
// as a function of form `func(d *Daemon, p ParamType) middleware.Responder`.
// This wrapper takes care of Await'ing for *Daemon.
type apiHandler[Params any] struct {
	dp      promise.Promise[*Daemon]
	handler func(d *Daemon, p Params) middleware.Responder
}

func (a *apiHandler[Params]) Handle(p Params) middleware.Responder {
	// Wait for *Daemon to be ready. While 'p' would have a context, it's hard to get it
	// since it's a struct. Could use reflection, but since we'll stop the agent anyway
	// if daemon initialization fails it doesn't really matter that much here what context
	// to use.
	d, err := a.dp.Await(context.Background())
	if err != nil {
		return api.Error(http.StatusServiceUnavailable, err)
	}
	return a.handler(d, p)
}

func wrapAPIHandler[Params any](dp promise.Promise[*Daemon], handler func(d *Daemon, p Params) middleware.Responder) *apiHandler[Params] {
	return &apiHandler[Params]{dp: dp, handler: handler}
}

// apiHandlers bridges the API handlers still implemented inside Daemon into a set of
// individual handlers. Since NewDaemon() is side-effectful, we can only get a promise for
// *Daemon, and thus the handlers will need to Await() for it to be ready.
//
// This method depends on [deletionQueue] to make sure the deletion lock file is created and locked
// before the API server starts.
//
// This is meant to be a temporary measure until handlers have been moved out from *Daemon
// to daemon/restapi or feature-specific packages. At that point the dependency on *deletionQueue
// should be moved to the cell in daemon/restapi.
func ciliumAPIHandlers(dp promise.Promise[*Daemon], cfg *option.DaemonConfig, _ *deletionQueue) (out handlersOut) {
	// /healthz/
	out.DaemonGetHealthzHandler = wrapAPIHandler(dp, getHealthzHandler)

	// /endpoint/
	out.EndpointDeleteEndpointHandler = wrapAPIHandler(dp, deleteEndpointHandler)
	out.EndpointGetEndpointHandler = wrapAPIHandler(dp, getEndpointHandler)

	// /endpoint/{id}
	out.EndpointGetEndpointIDHandler = wrapAPIHandler(dp, getEndpointIDHandler)
	out.EndpointPutEndpointIDHandler = wrapAPIHandler(dp, putEndpointIDHandler)
	out.EndpointPatchEndpointIDHandler = wrapAPIHandler(dp, patchEndpointIDHandler)
	out.EndpointDeleteEndpointIDHandler = wrapAPIHandler(dp, deleteEndpointIDHandler)

	// /endpoint/{id}config/
	out.EndpointGetEndpointIDConfigHandler = wrapAPIHandler(dp, getEndpointIDConfigHandler)
	out.EndpointPatchEndpointIDConfigHandler = wrapAPIHandler(dp, patchEndpointIDConfigHandler)

	// /endpoint/{id}/labels/
	out.EndpointGetEndpointIDLabelsHandler = wrapAPIHandler(dp, getEndpointIDLabelsHandler)
	out.EndpointPatchEndpointIDLabelsHandler = wrapAPIHandler(dp, putEndpointIDLabelsHandler)

	// /endpoint/{id}/log/
	out.EndpointGetEndpointIDLogHandler = wrapAPIHandler(dp, getEndpointIDLogHandler)

	// /endpoint/{id}/healthz
	out.EndpointGetEndpointIDHealthzHandler = wrapAPIHandler(dp, getEndpointIDHealthzHandler)

	// /identity/
	out.PolicyGetIdentityHandler = wrapAPIHandler(dp, getIdentityHandler)
	out.PolicyGetIdentityIDHandler = wrapAPIHandler(dp, getIdentityIDHandler)

	// /identity/endpoints
	out.PolicyGetIdentityEndpointsHandler = wrapAPIHandler(dp, getIdentityEndpointsHandler)

	// /debuginfo
	out.DaemonGetDebuginfoHandler = wrapAPIHandler(dp, getDebugInfoHandler)

	// /ip/
	out.PolicyGetIPHandler = wrapAPIHandler(dp, getIPHandler)

	return
}
