package models

type MatchRequest struct {
	UserID1 string `json:"user_id1"`
	UserID2 string `json:"user_id2"`
}

type Match struct {
	MatchID string `json:"match_id"`
	UserID1 string `json:"user_id1"`
	UserID2 string `json:"user_id2"`
}
