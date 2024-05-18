package api

import (
	"context"
	"net/http"

	"github.com/fulldump/box"
	"github.com/fulldump/box/boxopenapi"

	"github.com/fulldump/inceptiondb/api/apicollectionv1"
	"github.com/fulldump/inceptiondb/service"
	"github.com/fulldump/inceptiondb/statics"
)

func Build(s service.Servicer, staticsDir, version string) *box.B { // TODO: remove datadir

	b := box.NewBox()

	v1 := b.Resource("/v1")
	v1.WithInterceptors(box.SetResponseHeader("Content-Type", "application/json"))

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

	spec := boxopenapi.Spec(b)
	spec.Info.Title = "InceptionDB"
	spec.Info.Description = "A durable in-memory database to store JSON documents."
	spec.Info.Contact = &boxopenapi.Contact{
		Url: "https://github.com/fulldump/inceptiondb/issues/new",
	}
	b.Handle("GET", "/openapi.json", func(r *http.Request) any {

		spec.Servers = []boxopenapi.Server{
			{
				Url: "https://" + r.Host,
			},
			{
				Url: "http://" + r.Host,
			},
		}

		return spec
	})

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
