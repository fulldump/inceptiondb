# Find

Retrieve one or more items from a collection by searching with a specified index and value.

## Parameteres

* collectionName: The name of the collection to search in.
* query: An object containing the index and value to search for, along with optional limit and skip parameters.

## Usage Examples

cURL Example:
```sh
curl "https://example.com/v1/collections/my-collection/find?index=my-index&value=John%20Doe&limit=10&skip=0" \
-H "Authorization: Bearer $API_KEY"
```

## Response structure

```json
[
  {
    "id": "1",
    "name": "John Doe",
    "email": "john.doe@example.com"
  },
  {
    "id": "2",
    "name": "John Doe",
    "email": "johndoe@example.org"
  }
]

```