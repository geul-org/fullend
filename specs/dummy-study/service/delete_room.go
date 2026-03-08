package service

import "net/http"

// @sequence authorize
// @action delete
// @resource room
// @id RoomID

// @sequence get
// @model Room.FindByID
// @param RoomID request
// @result room Room

// @sequence guard nil room
// @message "스터디룸이 존재하지 않습니다"

// @sequence get
// @model Reservation.CountByRoomID
// @param RoomID request
// @result reservationCount int

// @sequence guard exists reservationCount
// @message "예약이 존재하여 삭제할 수 없습니다"

// @sequence delete
// @model Room.Delete
// @param RoomID request

// @sequence response json
func DeleteRoom(w http.ResponseWriter, r *http.Request) {}
