package service

import "net/http"

// @sequence get
// @model Enrollment.ListByUser
// @param UserID currentUser
// @result enrollments []Enrollment
//
// @sequence response json
// @var enrollments
func ListMyEnrollments(w http.ResponseWriter, r *http.Request) {}
