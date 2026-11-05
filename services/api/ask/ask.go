package ask

type Source struct {
	DocumentID string  `json:"documentId"`
	ChunkIndex int     `json:"chunkIndex"`
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
}

type Response struct {
	Answer  string   `json:"answer"`
	Sources []Source `json:"sources"`
}
