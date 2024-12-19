package box

// An I stands for Interceptor
type I = func(next H) H
type Interceptor = I // Just alias to make it more readable
