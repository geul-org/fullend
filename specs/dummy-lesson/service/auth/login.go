package service

import "net/http"

// @sequence get
// @model User.FindByEmail
// @param Email request
// @result user User
//
// @sequence guard nil user
// @message "사용자를 찾을 수 없습니다"
//
// @sequence password
// @param user.PasswordHash
// @param Password request
//
// @sequence call
// @func issueToken
// @param user
// @result token Token
//
// @sequence response json
// @var token
func Login(w http.ResponseWriter, r *http.Request) {}
