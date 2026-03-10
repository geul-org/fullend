package service

import "github.com/geul-org/fullend/pkg/auth"

// @call string hashedPassword = auth.HashPassword({Password: request.Password})
// @post User user = User.Create({Email: request.Email, PasswordHash: hashedPassword.HashedPassword, Role: request.Role, Name: request.Name})
// @response {
//   user: user
// }
func Register() {}
