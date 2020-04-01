// Copyright (c) 2019 The Jaeger Authors.
// Copyright (c) 2017 Uber Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package frontend

import (
	"context"
	"encoding/json"
	"net/http"
	"path"
	"text/template"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"go.uber.org/zap"

	"github.com/signalfx/hotrod_rum/pkg/httperr"
	"github.com/signalfx/hotrod_rum/pkg/log"
	"github.com/signalfx/hotrod_rum/pkg/tracing"
)

// Server implements jaeger-demo-frontend service
type Server struct {
	hostPort string
	tracer   opentracing.Tracer
	logger   log.Factory
	bestETA  *bestETA
	assetFS  http.FileSystem
	basepath string
}

// ConfigOptions used to make sure service clients
// can find correct server ports
type ConfigOptions struct {
	FrontendHostPort string
	DriverHostPort   string
	CustomerHostPort string
	RouteHostPort    string
	Basepath         string
}

// NewServer creates a new frontend.Server
func NewServer(options ConfigOptions, tracer opentracing.Tracer, logger log.Factory) *Server {
	assetFS := FS(false)
	return &Server{
		hostPort: options.FrontendHostPort,
		tracer:   tracer,
		logger:   logger,
		bestETA:  newBestETA(tracer, logger, options),
		assetFS:  assetFS,
		basepath: options.Basepath,
	}
}

// Run starts the frontend server
func (s *Server) Run() error {
	mux := s.createServeMux()
	s.logger.Bg().Info("Starting", zap.String("address", "http://"+path.Join(s.hostPort, s.basepath)))
	return http.ListenAndServe(s.hostPort, mux)
}

func (s *Server) createServeMux() http.Handler {
	mux := tracing.NewServeMux(s.tracer)
	p := path.Join("/", s.basepath)
	mux.Handle(path.Join(p, "/"), http.HandlerFunc(s.index))
	mux.Handle(path.Join(p, "/dispatch"), http.HandlerFunc(s.dispatch))
	return mux
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tmpl, err := s.getTemplate(ctx, "/index.html")

	tmplCtx := map[string]string{}
	span := opentracing.SpanFromContext(ctx)

	if spanCtx, ok := span.Context().(jaeger.SpanContext); ok {
		tmplCtx["traceID"] = spanCtx.TraceID().String()
	}

	err = tmpl.Execute(w, tmplCtx)
	if err != nil {
		s.logger.For(ctx).Error("could not execute template", zap.Error(err))
		return
	}
}

func (s *Server) getTemplate(ctx context.Context, name string) (*template.Template, error) {
	name, err := FSString(false, name)
	if err != nil {
		s.logger.For(ctx).Error("could not find template", zap.Error(err))
		return nil, err
	}

	t, err := template.New("").Parse(name)
	if err != nil {
		s.logger.For(ctx).Error("could not parse template", zap.Error(err))
		return nil, err
	}
	return t, nil
}

func (s *Server) dispatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s.logger.For(ctx).Info("HTTP request received", zap.String("method", r.Method), zap.Stringer("url", r.URL))
	if err := r.ParseForm(); httperr.HandleError(w, err, http.StatusBadRequest) {
		s.logger.For(ctx).Error("bad request", zap.Error(err))
		return
	}

	customerID := r.Form.Get("customer")
	if customerID == "" {
		http.Error(w, "Missing required 'customer' parameter", http.StatusBadRequest)
		return
	}

	// TODO distinguish between user errors (such as invalid customer ID) and server failures
	response, err := s.bestETA.Get(ctx, customerID)
	if httperr.HandleError(w, err, http.StatusInternalServerError) {
		s.logger.For(ctx).Error("request failed", zap.Error(err))
		return
	}

	data, err := json.Marshal(response)
	if httperr.HandleError(w, err, http.StatusInternalServerError) {
		s.logger.For(ctx).Error("cannot marshal response", zap.Error(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
