# Insert many

Curl example:

```sh
curl -X POST "https://example.com/v1/collections/my-collection:insert" \
-d '{"id":"1","name":"Alfonso"}
{"id":"2","name":"Gerardo"}
{"id":"3","name":"Alfonso"}
'
```


HTTP request/response example:

```http
POST /v1/collections/my-collection:insert HTTP/1.1
Host: example.com

{"id":"1","name":"Alfonso"}
{"id":"2","name":"Gerardo"}
{"id":"3","name":"Alfonso"}


HTTP/1.1 201 Created
Content-Length: 0
Date: Mon, 15 Aug 2022 02:08:13 GMT


```


