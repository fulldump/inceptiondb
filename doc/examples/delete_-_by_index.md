# Delete - by index

Curl example:

```sh
curl -X POST "https://example.com/v1/collections/my-collection:remove" \
-d '{
    "field": "id",
    "mode": "unique",
    "value": "1"
}'
```


HTTP request/response example:

```http
POST /v1/collections/my-collection:remove HTTP/1.1
Host: example.com

{
    "field": "id",
    "mode": "unique",
    "value": "1"
}

HTTP/1.1 200 OK
Content-Length: 28
Content-Type: text/plain; charset=utf-8
Date: Mon, 15 Aug 2022 02:08:13 GMT

{
    "id": "1",
    "name": "Alfonso"
}
```


