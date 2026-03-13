package model

// CurrentUser is the authenticated user extracted by JWT middleware.
type CurrentUser struct {
	Email string
	ID int64
	Role string
}

// Authorizer checks permissions.
type Authorizer interface {
	Check(user *CurrentUser, action, resource string, input interface{}) error
}
