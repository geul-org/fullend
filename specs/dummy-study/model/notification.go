package model

// Notification은 알림 발송 component다.
// SSaC에서 @component notification으로 참조된다.
type Notification interface {
	Execute(reservation Reservation, message string) error
}
