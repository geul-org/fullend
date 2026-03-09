package service

import "net/http"

// @sequence authorize
// @action delete
// @resource lesson
// @id LessonID
//
// @sequence get
// @model Lesson.FindByID
// @param LessonID request
// @result lesson Lesson
//
// @sequence guard nil lesson
// @message "차시를 찾을 수 없습니다"
//
// @sequence delete
// @model Lesson.Delete
// @param LessonID request
//
// @sequence response json
func DeleteLesson(w http.ResponseWriter, r *http.Request) {}
