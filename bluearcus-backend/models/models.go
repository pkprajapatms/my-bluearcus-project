package models

type (
	InsertData struct {
		X         int64  `json:"x"`
		Y         int64  `json:"y"`
		Timestamp string `json:"timestamp"`
		Type      string `json:"type"`
	}
	DataPoint struct {
		X int64 `json:"x"`
		Y int64 `json:"y"`
	}
)