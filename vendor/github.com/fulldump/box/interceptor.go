package box

// An I stands for Interceptor
type I func(next H) H
