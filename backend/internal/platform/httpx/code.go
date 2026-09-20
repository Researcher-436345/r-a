package httpx

import "net/http"

// ErrorCode writes {"detail": ..., "code": ...}. The frontend ApiError exposes
// code, so clients can branch on it instead of parsing human-readable detail.
func ErrorCode(w http.ResponseWriter, status int, code, detail string) {
	JSON(w, status, map[string]string{"detail": detail, "code": code})
}
