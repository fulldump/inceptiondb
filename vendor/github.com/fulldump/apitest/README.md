# Apitest

Easy way to test HTTP APIs with more readable and less verbose code.

<table>
	<tr>
		<th>With standard library</th>
		<th>With ApiTest</th>
	</tr>
	<tr>
		<td>
			<pre lang="go">
post_body := map[string]interface{}{
&nbsp;&nbsp;&nbsp;&nbsp;"hello": "World!",
}
<br>
post_json, _ := json.Marshal(post_body)
// TODO: handle post_json error
<br>
post_bytes := bytes.NewBuffer(post_json)
<br>
request, _ := http.NewRequest("POST",
&nbsp;&nbsp;&nbsp;&nbsp;"https://httpbin.org/post", post_bytes)
request.Header.Set("X-My-Header", "hello")
// TODO: handle request error
<br>
response, _ := http.DefaultClient.Do(request)
// TODO: handle response error
<br>
response_body := map[string]interface{}{}
<br>
_ = json.NewDecoder(response.Body).
&nbsp;&nbsp;&nbsp;&nbsp;Decode(&response_body)
// TODO: handle response error
<br>
fmt.Println("Check response:", response_body)
			</pre>
		</td>
		<td>
			<pre lang="go">
a := apitest.NewWithBase("https://httpbin.org")
<br>
r := a.Request("POST", "/post").
&nbsp;&nbsp;&nbsp;&nbsp;WithHeader("X-My-Header", "hello").
&nbsp;&nbsp;&nbsp;&nbsp;WithBodyJson(map[string]interface{}{
&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;"hello": "World!",
&nbsp;&nbsp;&nbsp;&nbsp;}).Do()
<br>
response_body := r.BodyJson()
<br>
fmt.Println("Check response:", response_body)
			</pre>
		</td>
	</tr>
</table>



## Getting started

```go
my_api := golax.NewApi()

// build `my_api`...

testserver := apitest.New(my_api)

r := testserver.Request("POST", "/users/23/items").
    WithHeader("Content-Type", "application/json").
    WithCookie("sess_id", "123123213213213"),
    WithBodyString(`
        {
            "name": "pencil",
            "description": "Blah blah..."
        }
    `).
    Do()

r.StatusCode // Check this
r.BodyString() // Check this
```

## Sending body JSON

```go
r := testserver.Request("POST", "/users/23/items").
    WithBodyJson(map[string]interface{}{
        "name": "pencil",
        "description": "Blah blah",
    }).
    Do()
```

## Reading body JSON

```go
r := testserver.Request("GET", "/users/23").
    Do()
    
body := r.BodyJson()
```

## Asynchronous request

```go
func Test_Example(t *testing.T) {

	a := golax.NewApi()

	a.Root.Node("users").Method("GET", func(c *golax.Context) {
		fmt.Fprint(c.Response, "John")
	})

	s := apitest.New(a)

	w := &sync.WaitGroup{}

	for i := 0; i < 10; i++ {
		w.Add(1)
		n := i
		go s.Request("GET", "/users").DoAsync(func(r *apitest.Response) {

			if http.StatusOK != r.StatusCode {
				t.Error("Expected status code is 200")
			}

			fmt.Println(r.BodyString(), n)

			w.Done()
		})
	}

	w.Wait()
}
```

