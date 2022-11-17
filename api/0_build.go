package api

import (
	"context"
	"net/http"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/api/apicollectionv1"
	"github.com/fulldump/inceptiondb/service"
	"github.com/fulldump/inceptiondb/statics"
)

func Build(s service.Servicer, staticsDir, version string) *box.B { // TODO: remove datadir

	b := box.NewBox()

	v1 := b.Resource("/v1")
	apicollectionv1.BuildV1Collection(v1, s).
		WithInterceptors(
			injectServicer(s),
		)

	b.Resource("/v1/*").
		WithActions(box.AnyMethod(func(w http.ResponseWriter) interface{} {
			w.WriteHeader(http.StatusNotImplemented)
			return PrettyError{
				Message:     "not implemented",
				Description: "this endpoint does not exist, please check the documentation",
			}
		}))

	b.Resource("/release").
		WithActions(box.Get(func() string {
			return version
		}))

	// Mount statics
	b.Resource("/*").
		WithActions(
			box.Get(statics.ServeStatics(staticsDir)).WithName("serveStatics"),
		)

	return b
}

func injectServicer(s service.Servicer) box.I {
	return func(next box.H) box.H {
		return func(ctx context.Context) {
			next(apicollectionv1.SetServicer(ctx, s))
		}
	}
}
