package service

import "net/http"

// @sequence get
// @model Reservation.FindByID
// @param ReservationID request
// @result reservation Reservation

// @sequence guard nil reservation
// @message "예약을 찾을 수 없습니다"

// @sequence response json
// @var reservation
func GetReservation(w http.ResponseWriter, r *http.Request) {}
