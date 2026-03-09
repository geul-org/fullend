package service

import "net/http"

// @sequence get
// @model Review.FindByID
// @param ReviewID request
// @result review Review
//
// @sequence guard nil review
// @message "리뷰를 찾을 수 없습니다"
//
// @sequence authorize
// @action delete
// @resource review
// @id ReviewID
//
// @sequence delete
// @model Review.Delete
// @param ReviewID request
//
// @sequence response json
func DeleteReview(w http.ResponseWriter, r *http.Request) {}
