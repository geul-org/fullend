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
// @model Course.Publish
// @param CourseID request
//
// @sequence response json
func PublishCourse(w http.ResponseWriter, r *http.Request) {}
