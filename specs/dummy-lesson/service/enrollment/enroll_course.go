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
// @model Enrollment.FindByCourseAndUser
// @param CourseID request
// @param UserID currentUser
// @result existing Enrollment
//
// @sequence guard exists existing
// @message "이미 수강 중입니다"
//
// @sequence post
// @model Enrollment.Create
// @param UserID currentUser
// @param CourseID request
// @result enrollment Enrollment
//
// @sequence post
// @model Payment.Create
// @param UserID currentUser
// @param enrollment.ID
// @param course.Price
// @param PaymentMethod request
// @param "pending"
// @result payment Payment
//
// @sequence call
// @component notification
// @param enrollment
//
// @sequence response json
// @var enrollment
// @var payment
func EnrollCourse(w http.ResponseWriter, r *http.Request) {}
