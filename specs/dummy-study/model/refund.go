package model

// Refund는 환불 계산 결과다.
type Refund struct {
	Amount int
	Reason string
}

// calculateRefund는 예약 취소 시 환불 금액을 계산한다.
// SSaC에서 @func calculateRefund로 참조된다.
func calculateRefund(reservation Reservation) (Refund, error) {
	// 구현은 개발자가 직접 작성
	return Refund{}, nil
}
