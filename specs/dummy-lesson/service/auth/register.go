package service

import "net/http"

// @sequence get
// @model User.FindByEmail
// @param Email request
// @result existing User
//
// @sequence guard exists existing
// @message "이미 가입된 이메일입니다"
//
// @sequence call
// @func hashPassword
// @param Password request
// @result hashedPassword string
//
// @sequence post
// @model User.Create
// @param Email request
// @param hashedPassword
// @param Name request
// @param "student"
// @result user User
//
// @sequence response json
// @var user
func Register(w http.ResponseWriter, r *http.Request) {}
