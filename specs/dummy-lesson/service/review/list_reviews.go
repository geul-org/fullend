package service

import "net/http"

// @sequence get
// @model Review.ListByCourse
// @param CourseID request
// @result reviews []Review
//
// @sequence response json
// @var reviews
func ListReviews(w http.ResponseWriter, r *http.Request) {}
