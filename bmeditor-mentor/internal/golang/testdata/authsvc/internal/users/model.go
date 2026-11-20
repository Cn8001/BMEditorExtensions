// Package users keeps the accounts of the people who use the service.
package users

import "time"

// User is a person's account: what they are called and how to reach
// them. It does not hold how they sign in.
type User struct {
	ID        int64
	Name      string
	Email     string
	CreatedAt time.Time
}

// NewUser makes an account for name, reachable at email.
func NewUser(name, email string) *User {
	return &User{Name: name, Email: email, CreatedAt: time.Now()}
}
