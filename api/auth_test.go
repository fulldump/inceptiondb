package api

import (
	"net/http"
	"testing"

	"github.com/fulldump/apitest"
	"github.com/fulldump/biff"

	"github.com/fulldump/inceptiondb/database"
	"github.com/fulldump/inceptiondb/service"
)

func TestAuthentication(t *testing.T) {

	biff.Alternative("Authentication", func(a *biff.A) {

		db := database.NewDatabase(&database.Config{
			Dir: t.TempDir(),
		})

		s := service.NewService(db)

		apiKey := "my-key"
		apiSecret := "my-secret"

		b := Build(s, "", "test", apiKey, apiSecret, false)
		b.WithInterceptors(
			PrettyErrorInterceptor,
		)

		api := apitest.NewWithHandler(b)

		a.Alternative("Missing headers", func(a *biff.A) {
			resp := api.Request("GET", "/v1/collections").Do()
			biff.AssertEqual(resp.StatusCode, http.StatusUnauthorized)
			biff.AssertEqualJson(resp.BodyJson(), map[string]any{
				"error": map[string]any{
					"message":     "unauthorized",
					"description": "user is not authenticated",
				},
			})
		})

		a.Alternative("Wrong Key", func(a *biff.A) {
			resp := api.Request("GET", "/v1/collections").
				WithHeader("X-Api-Key", "wrong-key").
				WithHeader("X-Api-Secret", apiSecret).
				Do()
			biff.AssertEqual(resp.StatusCode, http.StatusUnauthorized)
		})

		a.Alternative("Wrong Secret", func(a *biff.A) {
			resp := api.Request("GET", "/v1/collections").
				WithHeader("X-Api-Key", apiKey).
				WithHeader("X-Api-Secret", "wrong-secret").
				Do()
			biff.AssertEqual(resp.StatusCode, http.StatusUnauthorized)
		})

		a.Alternative("Correct credentials", func(a *biff.A) {
			resp := api.Request("GET", "/v1/collections").
				WithHeader("X-Api-Key", apiKey).
				WithHeader("X-Api-Secret", apiSecret).
				Do()
			biff.AssertEqual(resp.StatusCode, http.StatusOK)
		})

	})
}
