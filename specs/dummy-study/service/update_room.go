package service

import "net/http"

// @sequence authorize
// @action update
// @resource room
// @id RoomID

// @sequence get
// @model Room.FindByID
// @param RoomID request
// @result room Room

// @sequence guard nil room
// @message "스터디룸이 존재하지 않습니다"

// @sequence put
// @model Room.Update
// @param RoomID request
// @param Name request
// @param Capacity request
// @param Location request

// @sequence response json
// @var room
func UpdateRoom(w http.ResponseWriter, r *http.Request) {}
