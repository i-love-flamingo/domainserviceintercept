# Domain Service Intercept

Helper to intercept calls, check for patches, and apply them if necessary.

Supposed to be used in dev, not prod.

example:
```go
package bla

import (
	"flamingo.me/dingo"
	"flamingo.me/domainserviceintercept"
)

type serviceMock struct{}

func (*serviceMock) Configure(injector *dingo.Injector) {
	go dsi.Traceserver()
	injector.BindInterceptor(new(application.Service), dcsIntercepter{})
}

type sInterceptor struct {
	application.Service
}

func (i *sInterceptor) IsDuplicate(ctx context.Context, request *web.Request, order domain.Order) (duplicated bool, err error) {
	dsi.Traceme(ctx, "IsDuplicate", map[string]interface{}{"request": request}, func() { duplicated, err = i.Service.IsDuplicate(ctx, request, order) }, &duplicated, &err)
	return
}
```
