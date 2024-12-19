package box

import "context"

// An H stands for Handler
type H = func(ctx context.Context)
type Handler = H // Just alias to make it more readable
