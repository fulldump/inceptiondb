# Delete - fullscan

Curl example:

```sh
curl -X POST "https://example.com/v1/collections/my-collection:remove" \
-d '{
    "filter": {
        "name": "Alfonso"
    },
    "limit": 10
}'
```


HTTP request/response example:

```http
POST /v1/collections/my-collection:remove HTTP/1.1
Host: example.com

{
    "filter": {
        "name": "Alfonso"
    },
    "limit": 10
}

HTTP/1.1 200 OK
Content-Length: 56
Content-Type: text/plain; charset=utf-8
Date: Mon, 15 Aug 2022 02:08:13 GMT

{"id":"1","name":"Alfonso"}
{"id":"3","name":"Alfonso"}

```


