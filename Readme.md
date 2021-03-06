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
	dsi.Traceme(ctx, "IsDuplicate", dsi.A{"request": request}, func() { duplicated, err = i.Service.IsDuplicate(ctx, request, order) }, dsi.A{"duplicated": &duplicated, "err": &err}, nil)
	return
}
```

Example config:
```yaml
- what: IsDuplicate
  return:
    duplicated: true
  repeat: 2
```
