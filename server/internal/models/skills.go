package models

type CreateNewSkill struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Content     string `json:"content"`
}
