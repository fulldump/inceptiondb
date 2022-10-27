package apicollectionv1

import (
	"context"

	"github.com/fulldump/inceptiondb/service"
)

const ContextServicerKey = "ed0fa170-5593-11ed-9d60-9bdc940af29d"

func SetServicer(ctx context.Context, s service.Servicer) context.Context {
	return context.WithValue(ctx, ContextServicerKey, s)
}

func GetServicer(ctx context.Context) service.Servicer {
	return ctx.Value(ContextServicerKey).(service.Servicer) // TODO: can raise panic :D
}
