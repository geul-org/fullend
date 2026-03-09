package service

import "net/http"

// @sequence get
// @model Course.FindByID
// @param CourseID request
// @result course Course
//
// @sequence guard nil course
// @message "강의를 찾을 수 없습니다"
//
// @sequence get
// @model Lesson.ListByCourse
// @param CourseID request
// @result lessons []Lesson
//
// @sequence response json
// @var course
// @var lessons
func GetCourse(w http.ResponseWriter, r *http.Request) {}
