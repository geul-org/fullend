package service

import "net/http"

// @sequence get
// @model Enrollment.FindByCourseAndUser
// @param CourseID request
// @param UserID currentUser
// @result enrollment Enrollment
//
// @sequence guard nil enrollment
// @message "수강 중인 강의만 리뷰할 수 있습니다"
//
// @sequence get
// @model Review.FindByCourseAndUser
// @param CourseID request
// @param UserID currentUser
// @result existing Review
//
// @sequence guard exists existing
// @message "이미 리뷰를 작성했습니다"
//
// @sequence post
// @model Review.Create
// @param UserID currentUser
// @param CourseID request
// @param Rating request
// @param Comment request
// @result review Review
//
// @sequence response json
// @var review
func CreateReview(w http.ResponseWriter, r *http.Request) {}
