package api

import (
	"context"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/api/apicollectionv1"
	"github.com/fulldump/inceptiondb/database"
	"github.com/fulldump/inceptiondb/service"
	"github.com/fulldump/inceptiondb/statics"
)

func Build(db *database.Database, staticsDir string) *box.B { // TODO: remove datadir

	b := box.NewBox()

	v1 := b.Resource("/v1")
	s := service.NewService(db)
	apicollectionv1.BuildV1Collection(v1, s).
		WithInterceptors(
			injectServicer(s),
		)

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
