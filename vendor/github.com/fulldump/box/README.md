# Box
<img src="logo.png">

<p align="center">
<a href="https://app.travis-ci.com/fulldump/box" rel="nofollow"><img src="https://app.travis-ci.com/fulldump/box.svg?branch=master" alt="Build Status"></a>
<a href="https://goreportcard.com/report/github.com/fulldump/box"><img src="https://goreportcard.com/badge/github.com/fulldump/box"></a>
<a href="https://godoc.org/github.com/fulldump/box"><img src="https://godoc.org/github.com/fulldump/box?status.svg" alt="GoDoc"></a>
<img alt="GitHub release (latest SemVer)" src="https://img.shields.io/github/v/release/fulldump/box?sort=semver">
</p>

Box is an HTTP router to speed up development. Box supports URL parameters, interceptors, magic handlers
and introspection documentation.

<!-- TOC -->
* [Box](#box)
  * [Getting started](#getting-started)
  * [Sending JSON](#sending-json)
  * [URL parameters](#url-parameters)
  * [Receiving and sending JSON](#receiving-and-sending-json)
  * [Use interceptors](#use-interceptors)
  * [Error handling](#error-handling)
  * [Groups](#groups)
  * [Custom interceptors](#custom-interceptors)
  * [Parametrized interceptors](#parametrized-interceptors)
<!-- TOC -->


## Getting started

```go
package main

import (
	"github.com/fulldump/box"
)

func main() {

    b := box.NewBox()

    b.HandleFunc("GET", "/hello", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("World!"))
    })

    b.ListenAndServe() // listening at http://localhost:8080

}
```

## Sending JSON

```go
b := box.NewBox()

type MyResponse struct {
    Name string
    Age  int
}

b.Handle("GET", "/hello", func(w http.ResponseWriter, r *http.Request) MyResponse {
    return MyResponse{
        Name: "Fulanez",
        Age:  33,
    }
})
```

## URL parameters

```go
b := box.NewBox()

b.Handle("GET", "/articles/{article-id}", func(w http.ResponseWriter, r *http.Request) string {
    articleID := box.Param(r, "article-id")
    return "ArticleID is " + articleID
})
```

## Receiving and sending JSON

```go
type CreateArticleRequest struct {
    Title string
    Text  string
}

type Article struct {
    Id      string    `json:"id"`
    Title   string    `json:"title"`
    Text    string    `json:"text"`
    Created time.Time `json:"created"`
}

b := box.NewBox()
b.Handle("POST", "/articles", func(input CreateArticleRequest) Article {
    fmt.Println("Persist new article...", input)
    return Article{
        Id:      "my-new-id",
        Title:   input.Title,
        Text:    input.Text,
        Created: time.Unix(1674762079, 0),
    }
})
```

## Use interceptors

Interceptors, also known as middlewares, are pieces of code that are executed
in order before the handler to provide common functionality:

* Do things before and/or after the handler execution
* Cut the execution and stop executing the rest of interceptors and handler
* Inject items into the context

```go
func ListArticles()   { /* ... */ }
func CreateArticles() { /* ... */ }
func GetArticle()     { /* ... */ }
func DeleteArticle()  { /* ... */ }

func main() {
    b := box.NewBox()

    b.Use(box.AccessLog)   // use middlewares to print logs
    b.Use(box.PrettyError) // use middlewares return pretty errors

    b.Handle("GET", "/articles", ListArticles)
    b.Handle("POST", "/articles", CreateArticles)
    b.Handle("GET", "/articles/{article-id}", GetArticle)
    b.Handle("DELETE", "/articles/{article-id}", DeleteArticle)
}
```

## Error handling

```go
b := box.NewBox()
b.Use(box.PrettyError)
b.Handle("GET", "/articles", func() (*Article, error) {
    return nil, errors.New("could not connect to the database")
})
go b.ListenAndServe()

resp, _ := http.Get(s.URL + "/articles")
io.Copy(os.Stdout, resp.Body) // could not connect to the database
```

## Groups

Groups are a neat way to organize and compose big APIs and also to limit the scope
of interceptors.

```go
b := box.NewBox()

v0 := b.Group("/v0")
v0.Use(box.SetResponseHeader("Content-Type", "application/json"))

v0.Handle("GET", "/articles", ListArticles)
v0.Handle("POST", "/articles", CreateArticle)
```

## Custom interceptors

Interceptors are very useful to reuse logic in a very convenient and modular way.

Here is a sample interceptor that does nothing:

```go
func MyCustomInterceptor(next box.H) box.H {
	return func(ctx context.Context) {
        // do something before the handler
		next(ctx) // continue the flow
		// do something after the handler
	}
}
```

The following interceptor returns a `Server` header:

```go
func MyCustomInterceptor(next box.H) box.H {
	return func(ctx context.Context) {
		w := box.GetResponse(ctx)
		w.Header().Set("Server", "MyServer")
		next(ctx) // continue the flow
	}
}

func main() {

	b := box.NewBox()
	b.Use(MyCustomInterceptor)

}
```

## Parametrized interceptors

Sometimes interceptors can be generalized to cover a wider set of use cases. For
example, the following interceptor can set any response header and can be used
multiple times.

```go
func SetResponseHeader(key, value string) box.I {
	return func(next box.H) box.H {
		return func(ctx context.Context) {
			box.GetResponse(ctx).Header().Set(key, value)
			next(ctx)
		}
	}
}

func main() {
    b := box.NewBox()
    b.Use(
        box.SetResponseHeader("Server", "My server name"),
        box.SetResponseHeader("Version", "v3.2.1"),
    )
}
```
