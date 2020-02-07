package dsi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"flamingo.me/dingo"
	"flamingo.me/flamingo/v3/framework/flamingo"
	"go.opencensus.io/trace"
)

// Register DSI for current injector
func Register(injector *dingo.Injector) {
	go Traceserver()
	injector.BindInterceptor(new(flamingo.Logger), PdsiLogger{})
	http.DefaultTransport = &roundtripper{upstream: http.DefaultTransport}
}

type roundtripper struct {
	upstream http.RoundTripper
}

func (r *roundtripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	id := "0000000000000000"
	if ctx == nil {
		ctx = lastctx
	}
	if span := trace.FromContext(ctx); span != nil {
		id = span.SpanContext().TraceID.String()
	}

	b, _ := json.Marshal(req.Header)

	traceMutex.Lock()
	traces = append([]traceEntry{{
		c:        id,
		t:        time.Now(),
		logLevel: "Info",
		logMsg:   fmt.Sprintf("%s %s: %s", req.Method, req.URL.String(), string(b)),
	}}, traces...)
	traceMutex.Unlock()
	return r.upstream.RoundTrip(req)
}
