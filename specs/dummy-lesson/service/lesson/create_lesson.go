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
// @sequence post
// @model Lesson.Create
// @param CourseID request
// @param Title request
// @param VideoURL request
// @param SortOrder request
// @result lesson Lesson
//
// @sequence response json
// @var lesson
func CreateLesson(w http.ResponseWriter, r *http.Request) {}
