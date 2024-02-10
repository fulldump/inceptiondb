# Set defaults


The `SetDefaults` function is designed to automatically assign predefined default values to specific
fields in a document when a new entry is added to a database collection. This ensures consistency and 
completeness of data, especially for fields that require a default state or value.

## Overview

When you insert a new document into the collection, `SetDefaults` intervenes by checking for any fields 
that have not been explicitly provided in the input document. For such fields, if default values have 
been predefined using SetDefaults, those values are automatically added to the document before it is 
inserted into the collection. This process is seamless and ensures that every new document adheres to 
a defined structure and contains all necessary information.

## Example usage

Consider a scenario where you are adding a new user record to a collection but only provide the user's
name. If `SetDefaults` has been configured for the collection, it will automatically fill in any missing
fields that have default values defined.

### Input Document

When you attempt to insert a document with just the user's name:

```json
{
  "name": "Fulanez"
}
```

### Predefined Defaults

Assume the following default values have been set for the collection:

```json
{
	"id": "uuid()",      // A function generating a unique identifier
	"verified": false    // A boolean flag set to false by default
}
```

### Resulting Document

With `SetDefaults` applied, the document that gets inserted into the collection will include the missing
fields with their default values:

```json
{
  "id": "3bb5afae-c7b7-11ee-86b0-4f000ceb9a36", // Generated unique ID
  "name": "Fulanez",                             // Provided by the user
  "verified": false                              // Default value
}
```

## Benefits

* **Consistency**: Ensures that all documents in the collection follow a consistent structure, even when
some data points are not provided during insertion.
* **Completeness**: Guarantees that essential fields are always populated, either by the user or through
default values, ensuring data integrity.
* **Efficiency**: Saves time and effort by automating the assignment of common default values, reducing 
the need for manual data entry or post-insertion updates.

## Configuration

To utilize `SetDefaults`, you must first define the default values for the desired fields in your 
collection's configuration. This typically involves specifying a field name and its corresponding 
default value or function (e.g., uuid() for generating unique identifiers).

It's important to note that `SetDefaults` only applies to new documents being inserted into the 
collection. It does not affect documents that are already present in the collection or those being 
updated.

## Generative Functions in `SetDefaults`

`SetDefaults` supports a variety of generative functions to automatically assign dynamic values to 
fields in new documents. These functions are executed at the time of document insertion, ensuring that 
each entry receives a unique or contextually appropriate value based on the specified function. Below is
a list of supported generative functions:

### 1. `uuid()`

**Description**: Generates a Universally Unique Identifier (UUID) for the document. This is particularly
useful for assigning a unique identifier to each entry, ensuring that each document can be distinctly 
identified within the collection.

**Example Usage**: Ideal for fields requiring a unique ID, such as user identifiers, transaction IDs, etc.

**Output Example**: `"id": "3bb5afae-c7b7-11ee-86b0-4f000ceb9a36"`

### 2. `unixnano()`
**Description**: Produces a numerical value representing the current time in Unix nanoseconds. This 
function is handy for timestamping documents at the exact time of their creation, providing 
high-resolution time tracking.

**Example Usage**: Suitable for fields that need to record the precise time of document insertion, 
like creation timestamps, log entries, etc.

**Output Example**: `"created_at": 16180339887467395` (represents the number of nanoseconds since 
January 1, 1970, 00:00:00 UTC)

### 3. `auto()`
**Description**: Implements an automatic row counter that increments with each insert, starting from 
the first insertion. This function is beneficial for maintaining a sequential order or count of the
documents added to the collection.

**Example Usage**: Useful for auto-increment fields, such as a serial number, order number, or any
scenario where a simple, incrementing counter is needed.

**Output Example**: `"serial_number": 1023` (where 1023 is the current count of documents inserted 
since the first one)

### Implementation Considerations

When integrating generative functions with `SetDefaults`, consider the following:

**Uniqueness**: Functions like uuid() guarantee uniqueness, making them ideal for identifiers.

**Temporal Precision**: unixnano() provides high-precision timestamps, useful for time-sensitive data.

**Sequential Integrity**: auto() ensures a consistent, incremental sequence, beneficial for ordering or 
numbering entries.

Ensure that the chosen generative function aligns with the field's purpose and the overall data model's 
requirements. Proper configuration of `SetDefaults` with these functions enhances data integrity, 
consistency, and utility within your application.

			
Curl example:

```sh
curl -X POST "https://example.com/v1/collections/my-collection:setDefaults" \
-d '{
    "created_on": "unixnano()",
    "name": "",
    "street": "",
    "verified": false
}'
```


HTTP request/response example:

```http
POST /v1/collections/my-collection:setDefaults HTTP/1.1
Host: example.com

{
    "created_on": "unixnano()",
    "name": "",
    "street": "",
    "verified": false
}

HTTP/1.1 200 OK
Content-Length: 81
Content-Type: text/plain; charset=utf-8
Date: Mon, 15 Aug 2022 02:08:13 GMT

{
    "created_on": "unixnano()",
    "id": "uuid()",
    "name": "",
    "street": "",
    "verified": false
}
```


