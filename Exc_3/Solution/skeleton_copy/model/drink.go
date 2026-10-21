package model

type Drink struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	Price       uint16 `json:"price"`
	Description string `json:"Description"`
	// todo Add fields: Name, Price, Description with suitable types
	// todo json attributes need to be snakecase, i.e. name, created_at, my_variable, ..
}
