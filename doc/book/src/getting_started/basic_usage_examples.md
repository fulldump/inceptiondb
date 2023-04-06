# Basic Usage Examples

Here are some basic usage examples to help you get started with the client library.

## Creating a Collection

cURL example:

```shell
curl -X POST "https://example.com/v1/collections" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-collection"
  }'
```

## Inserting Data

cURL example:

```shell
curl -X POST "https://example.com/v1/collections/my-collection:insert" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "1",
    "name": "John Doe"
  }'
```

## Querying Data

cURL example:

```shell
curl "https://example.com/v1/collections/my-collection/find?index=my-index&value=1" \
  -H "Authorization: Bearer $API_KEY"
```

## Updating Data

cURL example:

```shell
curl -X PATCH "https://example.com/v1/collections/my-collection" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "1",
    "fields": {
      "name": "Jane Doe"
    }
  }'
```

## Deleting Data

cURL example:

```shell
curl -X DELETE "https://example.com/v1/collections/my-collection/1" \
  -H "Authorization: Bearer $API_KEY"
```
