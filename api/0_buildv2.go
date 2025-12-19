package api

import (
	"net/http"

	"github.com/fulldump/box"
	"github.com/fulldump/box/boxopenapi"

	"github.com/fulldump/inceptiondb/api/apicollectionv1"
	"github.com/fulldump/inceptiondb/service"
)

func BuildV2(s service.Servicer, staticsDir, version string, apiKey, apiSecret string) *box.B { // TODO: remove datadir

	b := box.NewBox()

	v2 := b.Resource("/v2")
	v2.WithInterceptors(
		box.SetResponseHeader("Content-Type", "application/json"),
		Authenticate(apiKey, apiSecret),
	)

	apicollectionv1.BuildV1Collection(v2, s).
		WithInterceptors(
			injectServicer(s),
		)

	b.Resource("/v2/*").
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

	// // Mount statics
	// b.Resource("/*").
	// 	WithActions(
	// 		box.Get(statics.ServeStatics(staticsDir)).WithName("serveStatics"),
	// 	)

	return b
}
