package telegramfax

type Update struct {
	UpdateID                int64                    `json:"update_id"`
	Message                 *Message                 `json:"message,omitempty"`
	BusinessConnection      *BusinessConnection      `json:"business_connection,omitempty"`
	BusinessMessage         *Message                 `json:"business_message,omitempty"`
	EditedBusinessMessage   *Message                 `json:"edited_business_message,omitempty"`
	DeletedBusinessMessages *DeletedBusinessMessages `json:"deleted_business_messages,omitempty"`
}

type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

type Message struct {
	MessageID            int64  `json:"message_id"`
	Date                 int64  `json:"date"`
	BusinessConnectionID string `json:"business_connection_id,omitempty"`
	From                 *User  `json:"from,omitempty"`
	Text                 string `json:"text,omitempty"`
}

type BusinessConnection struct {
	ID   string `json:"id"`
	User User   `json:"user"`
}

type DeletedBusinessMessages struct {
	BusinessConnectionID string  `json:"business_connection_id"`
	MessageIDs           []int64 `json:"message_ids"`
}

type GetUpdatesRequest struct {
	Offset         int64    `json:"offset,omitempty"`
	Timeout        int      `json:"timeout,omitempty"`
	AllowedUpdates []string `json:"allowed_updates,omitempty"`
}
