package service

import "net/http"

// @sequence authorize
// @action create
// @resource course
// @id new
//
// @sequence post
// @model Course.Create
// @param UserID currentUser
// @param Title request
// @param Description request
// @param Category request
// @param Level request
// @param Price request
// @result course Course
//
// @sequence response json
// @var course
func CreateCourse(w http.ResponseWriter, r *http.Request) {}
