<img src="logo.png">

<p align="center">
<a href="https://travis-ci.org/fulldump/biff"><img src="https://travis-ci.org/fulldump/biff.svg?branch=master"></a>
<a href="https://cover.run/go?tag=golang-1.10&repo=github.com%2Ffulldump%2Fbiff"><img src="https://cover.run/go/github.com/fulldump/biff.svg?style=flat&tag=golang-1.10" alt="coverage badge"></a>
<a href="https://goreportcard.com/report/fulldump/biff"><img src="http://goreportcard.com/badge/fulldump/biff"></a>
<a href="https://godoc.org/github.com/fulldump/biff"><img src="https://godoc.org/github.com/fulldump/biff?status.svg" alt="GoDoc"></a>
<a href="https://codeclimate.com/github/fulldump/biff/maintainability"><img src="https://api.codeclimate.com/v1/badges/1b34e1fe8a1ab355b044/maintainability" /></a>
</p>


Biff stands for BIFurcation Framework based on nesting cases or alternatives. You can take advantage of variable scoping to make your tests simpler and easier to read. Good choice for acceptance and use cases testing, it provides a BBD style exit.


<!-- MarkdownTOC autolink=true bracket=round -->

- [Getting started](#getting-started)
- [Get into the buggy line](#get-into-the-buggy-line)
- [Isolated use cases](#isolated-use-cases)
- [BDD on the fly](#bdd-on-the-fly)
- [Take advantage of go function scope](#take-advantage-of-go-function-scope)
- [Supported assertions](#supported-assertions)
- [Contribute](#contribute)
- [Testing](#testing)
- [Example project](#example-project)

<!-- /MarkdownTOC -->




## Getting started

```go
biff.Alternative("Instance service", func(a *biff.A) {

	s := NewMyService()

	a.Alternative("Register user", func(a *biff.A) {

		john := s.RegisterUser("john@email.com", "john-123")
		a.AssertNotNil(john)

		a.Alternative("Bad credentials", func(a *biff.A) {
			user := s.Login(john.Email, "bad-password")
			a.AssertNil(user)

		}).Alternative("Login", func(a *biff.A) {
			user := s.Login(john.Email, john.Password)
			a.AssertEqual(user, john)
		})

	})
})
```

Output:

```
=== RUN   TestExample
Case: Instance service
Case: Register user
    john is &example.User{Email:"john@email.com", Password:"john-123"}
Case: Bad credentials
    user is <nil>
-------------------------------
Case: Instance service
Case: Register user
    john is &example.User{Email:"john@email.com", Password:"john-123"}
Case: Login
    user is &example.User{Email:"john@email.com", Password:"john-123"}
-------------------------------
Case: Instance service
Case: Register user
    john is &example.User{Email:"john@email.com", Password:"john-123"}
-------------------------------
Case: Instance service
-------------------------------
--- PASS: TestExample (0.00s)
PASS
```

## Get into the buggy line

In case of error, Biff will print something like this:

```
Case: Instance service
Case: Register user
    john is &example.User{Email:"john@email.com", Password:"john-123"}
Case: Login
    Expected: &example.User{Email:"maria@email.com", Password:"1234"}
    Obtained: &example.User{Email:"john@email.com", Password:"john-123"}
    at biff/example.TestExample.func1.1.2(0xc420096ac0
    /home/fulldump/workspace/my-project/src/example/users_test.go:84 +0x12
```

Navigating directly to the line where the fail was produced.


## Isolated use cases

All possible bifurcations are tested in an isolated way.


## BDD on the fly

You do not need to translate your tests behaviour to natural language. Biff will navigate through the execution stack and will parse portions of your testing code to pretty print your assertions.

This testing code:

```go
a.AssertEqual(user, john)
```

will be printed as:

```
    user is &example.User{Email:"john@email.com", Password:"john-123"}
```


## Take advantage of go function scope

Avoid testing helpers and auxiliar methods to maintain the status between tests,
take advantage of language varialbe scope itself to write powerful tests easy to write
and easy to read.


## Supported assertions

Most commonly used assertions are implemented:

* `AssertEqual`
* `AssertEqualJson`
* `AssertNil`
* `AssertNotNil`
* `AssertNotEqual`
* `AssertInArray`
* `AssertTrue`
* `AssertFalse`


## Contribute

Feel free to fork, make changes and pull-request to master branch.

If you prefer, [create a new issue](https://github.com/fulldump/biff/issues/new) or email me for new features, issues or whatever.


## Testing

Who will test the tester? ha ha

There are no tests for the moment but there will be, sooner than later.


## Example project

This project includes an example project with some naive business logic plus some Biff tests.

