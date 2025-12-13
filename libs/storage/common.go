package storage

type TodoData struct {
	Lists map[string][]TodoItem `json:"lists"`
}

type TodoItem struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
	DueDate     string `json:"dueDate"`
	Priority    string `json:"priority"`
}
