package authz

# @ownership reservation: reservations.user_id

import rego.v1

default allow := false

# Anyone authenticated can create a reservation
allow if {
	input.action == "create"
	input.resource == "reservation"
}

# Only the owner can cancel their reservation
allow if {
	input.action == "cancel"
	input.resource == "reservation"
	input.user.id == input.resource_owner
}

# Only admin can update a room
allow if {
	input.action == "update"
	input.resource == "room"
	input.user.role == "admin"
}

# Only admin can delete a room
allow if {
	input.action == "delete"
	input.resource == "room"
	input.user.role == "admin"
}
