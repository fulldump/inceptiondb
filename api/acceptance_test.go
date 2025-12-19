package api

import (
	"testing"

	"github.com/fulldump/apitest"
	"github.com/fulldump/biff"

	"github.com/fulldump/inceptiondb/database"
	"github.com/fulldump/inceptiondb/service"
)

func TestAcceptance(t *testing.T) {

	biff.Alternative("Setup", func(a *biff.A) {

		db := database.NewDatabase(&database.Config{
			Dir: t.TempDir(),
		})

		biff.AssertNil(db.Load())
		biff.AssertEqual(db.GetStatus(), database.StatusOperating)

		s := service.NewService(db)

		b := Build(s, "", "test", "", "", false)
		b.WithInterceptors(
			InterceptorUnavailable(db),
			RecoverFromPanic,
			PrettyErrorInterceptor,
		)

		api := apitest.NewWithHandler(b)

		service.Acceptance(a, func(method, path string) *apitest.Request {
			return api.Request(method, "/v1"+path)
		})

	})
}
