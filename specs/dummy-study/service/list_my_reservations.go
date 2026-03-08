package service

import "net/http"

// @sequence get
// @model Reservation.ListByUserID
// @param UserID currentUser
// @result reservations []Reservation

// @sequence response json
// @var reservations
func ListMyReservations(w http.ResponseWriter, r *http.Request) {}
