package models

// Customer represents a customer in the system
type Customer struct {
	ID               int64  `json:"id"`
	Phone            string `json:"phone"`
	FirstName        string `json:"first_name"`
	LastName         string `json:"last_name"`
	Location         string `json:"location"`
	PreferredProduct string `json:"preferred_product"`
}

// CustomerFilter holds filtering options for listing customers
type CustomerFilter struct {
	Phone    string
	Location string
	Page     int
	PageSize int
}

// Validate performs basic validation on customer data
func (c *Customer) Validate() error {
	if c.Phone == "" {
		return ErrInvalidInput("phone is required")
	}
	return nil
}
