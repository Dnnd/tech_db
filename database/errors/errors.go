package errors

import (
	"github.com/lib/pq"
)

const (
	UniqueConstraintViolation = pq.ErrorCode("23505")
	ForeginKeyViolation       = pq.ErrorCode("23503")
)

func CheckErrorCode(code pq.ErrorCode, err error) bool {
	if driverError, ok := err.(*pq.Error); ok {
		if driverError.Code == code {
			return true
		}
	}
	return false
}

func CheckUniqueViolation(err error) bool {

	return CheckErrorCode(UniqueConstraintViolation, err)
}

func CheckForeginKeyViolation(err error) bool {

	return CheckErrorCode(ForeginKeyViolation, err)
}
