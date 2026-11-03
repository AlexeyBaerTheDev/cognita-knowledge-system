package search

type Result struct {
	DocumentID string  `json:"documentId"`
	ChunkIndex int     `json:"chunkIndex"`
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
}

type Response struct {
	Results []Result `json:"results"`
}
