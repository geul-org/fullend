package service

import "github.com/geul-org/fullend/pkg/auth"

// @get User user = User.FindByEmail({Email: request.Email})
// @empty user "User not found"
// @call auth.VerifyPassword({PasswordHash: user.PasswordHash, Password: request.Password})
// @call string accessToken = auth.IssueToken({UserID: user.ID, Email: user.Email, Role: user.Role})
// @response {
//   accessToken: accessToken
// }
func Login() {}
