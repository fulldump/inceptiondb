package api

import (
	"context"
	"encoding/json"
	"fmt"
	"inceptiondb/database"

	"github.com/fulldump/box"
)

func getBoxContext(ctx context.Context) *box.C {

	v := ctx.Value("box_context")
	if c, ok := v.(*box.C); ok {
		return c
	}

	return nil
}

func getParam(ctx context.Context, name string) (value string) {
	return getBoxContext(ctx).Parameters[name]
}

func interceptorPrintError(next box.H) box.H {
	return func(ctx context.Context) {
		next(ctx)
		err := box.GetError(ctx)
		if nil != err {
			json.NewEncoder(box.GetResponse(ctx)).Encode(map[string]interface{}{
				"error": err.Error(),
			})
		}
	}
}

func interceptorUnavailable(db *database.Database) box.I {
	return func(next box.H) box.H {
		return func(ctx context.Context) {

			status := db.GetStatus()
			if status == database.StatusOpening {
				box.SetError(ctx, fmt.Errorf("temporary unavailable: opening"))
				return
			}
			if status == database.StatusClosing {
				box.SetError(ctx, fmt.Errorf("temporary unavailable: closing"))
				return
			}
			next(ctx)
		}
	}
}
