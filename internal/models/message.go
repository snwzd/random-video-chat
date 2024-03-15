package models

type Message struct {
	Event string    `json:"event"`
	Data  *Exchange `json:"data"`
}

type Exchange struct {
	Username  string `json:"username"`
	Initiator bool   `json:"initiator"`
}
