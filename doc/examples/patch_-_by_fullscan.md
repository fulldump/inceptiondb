# Patch - by fullscan

Curl example:

```sh
curl -X POST "https://example.com/v1/collections/my-collection:patch" \
-d '{
    "filter": {
        "name": "Alfonso"
    },
    "limit": 10,
    "patch": {
        "country": "es"
    }
}'
```


HTTP request/response example:

```http
POST /v1/collections/my-collection:patch HTTP/1.1
Host: example.com

{
    "filter": {
        "name": "Alfonso"
    },
    "limit": 10,
    "patch": {
        "country": "es"
    }
}

HTTP/1.1 200 OK
Content-Length: 86
Content-Type: text/plain; charset=utf-8
Date: Mon, 15 Aug 2022 02:08:13 GMT

{"country":"es","id":"1","name":"Alfonso"}
{"country":"es","id":"3","name":"Alfonso"}

```


