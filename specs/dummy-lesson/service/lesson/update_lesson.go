package service

import "net/http"

// @sequence authorize
// @action update
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
// @sequence put
// @model Lesson.Update
// @param LessonID request
// @param Title request
// @param VideoURL request
// @param SortOrder request
//
// @sequence response json
func UpdateLesson(w http.ResponseWriter, r *http.Request) {}
