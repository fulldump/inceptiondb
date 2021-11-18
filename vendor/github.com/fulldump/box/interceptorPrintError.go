package box

import "context"

func InterceptorPrintError(next H) H {
	return func(ctx context.Context) {
		next(ctx)
		err := GetError(ctx)
		if nil != err {
			GetResponse(ctx).Write([]byte(err.Error()))
		}
	}
}
