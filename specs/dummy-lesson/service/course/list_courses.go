package service

import "net/http"

// @sequence get
// @model Course.List
// @result courses []Course
//
// @sequence response json
// @var courses
func ListCourses(w http.ResponseWriter, r *http.Request) {}
