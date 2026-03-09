package service

import "net/http"

// @sequence get
// @model Payment.ListByUser
// @param UserID currentUser
// @result payments []Payment
//
// @sequence response json
// @var payments
func ListMyPayments(w http.ResponseWriter, r *http.Request) {}
