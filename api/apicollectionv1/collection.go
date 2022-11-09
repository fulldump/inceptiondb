package apicollectionv1

type CollectionResponse struct {
	Name    string `json:"name"`
	Total   int    `json:"total"`
	Indexes int    `json:"indexes"`
}
