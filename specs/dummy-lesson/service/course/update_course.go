package service

import "net/http"

// @sequence authorize
// @action update
// @resource course
// @id CourseID
//
// @sequence get
// @model Course.FindByID
// @param CourseID request
// @result course Course
//
// @sequence guard nil course
// @message "강의를 찾을 수 없습니다"
//
// @sequence put
// @model Course.Update
// @param CourseID request
// @param Title request
// @param Description request
// @param Category request
// @param Level request
// @param Price request
//
// @sequence response json
func UpdateCourse(w http.ResponseWriter, r *http.Request) {}
