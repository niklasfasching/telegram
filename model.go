package telegram

type User struct {
	ID        int    `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
	IsBot     bool   `json:"is_bot"`
}

type Message struct {
	ID   int    `json:"message_id"`
	From User   `json:"from"`
	Date int    `json:"date"`
	Text string `json:"text"`
	Chat struct {
		ID        int    `json:"id"`
		FirstName string `json:"first_name"`
		Type      string `json:"type"`
		Username  string `json:"username"`
	} `json:"chat"`
}
