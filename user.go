package elevenlabs

import (
	"context"
	"net/http"
)

// UserService provides user API methods.
type UserService struct {
	client *Client
}

type UserResponse struct {
	UserID    string `json:"user_id"`
	SeatType  string `json:"seat_type,omitempty"`
	CreatedAt int64  `json:"created_at,omitempty"`
}

// Get fetches the current user.
func (s *UserService) Get(ctx context.Context) (*UserResponse, error) {
	var out UserResponse
	path := "/v1/user"
	err := s.client.doJSON(ctx, http.MethodGet, path, nil, true, func(ctx context.Context) (*http.Request, error) {
		return s.client.newRequest(ctx, http.MethodGet, path, nil, nil)
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
