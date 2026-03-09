package service

import "net/http"

// @sequence authorize
// @action delete
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
// @sequence delete
// @model Course.Delete
// @param CourseID request
//
// @sequence response json
func DeleteCourse(w http.ResponseWriter, r *http.Request) {}
