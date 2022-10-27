# Find - fullscan with limit 10

Curl example:

```sh
curl -X POST "https://example.com/v1/collections/my-collection:find" \
-d '{
    "limit": 10
}'
```


HTTP request/response example:

```http
POST /v1/collections/my-collection:find HTTP/1.1
Host: example.com

{
    "limit": 10
}

HTTP/1.1 200 OK
Content-Length: 82
Content-Type: text/plain; charset=utf-8
Date: Mon, 15 Aug 2022 02:08:13 GMT

{"id":"1","name":"Alfonso"}
{"id":"2","name":"Gerardo"}
{"id":"3","name":"Pedro"}

```


