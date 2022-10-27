# Retrieve index

Curl example:

```sh
curl -X POST "https://example.com/v1/collections/my-collection:getIndex" \
-d '{
    "name": "id"
}'
```


HTTP request/response example:

```http
POST /v1/collections/my-collection:getIndex HTTP/1.1
Host: example.com

{
    "name": "id"
}

HTTP/1.1 200 OK
Content-Length: 41
Content-Type: text/plain; charset=utf-8
Date: Mon, 15 Aug 2022 02:08:13 GMT

{
    "field": "id",
    "name": "id",
    "sparse": true
}
```


