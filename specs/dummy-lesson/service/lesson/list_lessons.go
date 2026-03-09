package service

import "net/http"

// @sequence get
// @model Lesson.ListByCourse
// @param CourseID request
// @result lessons []Lesson
//
// @sequence response json
// @var lessons
func ListLessons(w http.ResponseWriter, r *http.Request) {}
