# Common error types

* Bad Request (400): The request is malformed, incomplete, or contains invalid data. This usually occurs when required fields are missing, incorrect values are provided, or the data is not formatted properly.
* Unauthorized (401): The request lacks valid authentication credentials or the provided API key is invalid. Ensure that the API key is correct and passed in the request header.
* Forbidden (403): The authenticated user does not have the required permissions to perform the requested action. Verify that the user has the necessary permissions to access the requested resource.
* Not Found (404): The requested resource, such as a collection or an item, could not be found. Ensure that the provided identifiers are correct and the resource exists.
* Method Not Allowed (405): The request method (GET, POST, PATCH, DELETE) is not supported for the specified endpoint. Check the API documentation for the allowed methods for the endpoint.
* Internal Server Error (500): An unexpected error occurred on the server while processing the request. This typically indicates an issue with the API itself or its infrastructure.
