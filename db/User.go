package db

import (
	"time"
)

//User is the model for an entity which has access to the API
type User struct {
	ID       int       `db:"id" json:"id"`
	Created  time.Time `db:"created" json:"created"`
	Username string    `db:"username" json:"username" binding:"required"`
	Name     string    `db:"name" json:"name" binding:"required"`
	Email    string    `db:"email" json:"email" binding:"required"`
	Password string    `db:"password" json:"-"` // password hash
	Admin    bool      `db:"admin" json:"admin"`
	External bool      `db:"external" json:"external"`
	Alert    bool      `db:"alert" json:"alert"`
}

// UserWithPwd extends User structure with field for unhashed password received from JSON.
type UserWithPwd struct {
	Pwd string    `db:"-" json:"password"` // unhashed password from JSON
	User
}
