// Package domain holds the core data types shared across the service.
package domain

// Role identifies the author of a chat message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

// Article is the content of a paper fetched from the Article Service.
type Article struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Authors string `json:"authors"`
	Content string `json:"content"`
}

// Message is a single chat message stored in the Chat Service.
type Message struct {
	ID        string `json:"id"`
	Role      Role   `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
}
