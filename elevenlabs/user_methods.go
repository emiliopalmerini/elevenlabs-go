package elevenlabs

import (
	"context"
)

// Get gets information about the authenticated user.
func (c *UserService) Get(ctx context.Context) (*User, error) {
	return c.GetUser(ctx)
}

// GetUser gets information about the authenticated user.
func (c *UserService) GetUser(ctx context.Context) (*User, error) {
	resp, err := c.GetUserWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// GetWithResponse gets information about the authenticated user and returns
// HTTP response metadata.
func (c *UserService) GetWithResponse(ctx context.Context) (*Response[*User], error) {
	return c.GetUserWithResponse(ctx)
}

// GetUserWithResponse gets information about the authenticated user and returns
// HTTP response metadata.
func (c *UserService) GetUserWithResponse(ctx context.Context) (*Response[*User], error) {
	core, err := c.apiClient()
	if err != nil {
		return nil, err
	}

	var out User
	raw, err := core.GetJSON(ctx, "/v1/user", &out)
	if err != nil {
		return nil, err
	}

	return &Response[*User]{
		Data:        &out,
		RawResponse: raw,
	}, nil
}

// GetSubscription gets extended information about the authenticated user's
// subscription.
func (c *UserService) GetSubscription(ctx context.Context) (*Subscription, error) {
	resp, err := c.GetSubscriptionWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// GetSubscriptionWithResponse gets extended information about the authenticated
// user's subscription and returns HTTP response metadata.
func (c *UserService) GetSubscriptionWithResponse(ctx context.Context) (*Response[*Subscription], error) {
	core, err := c.apiClient()
	if err != nil {
		return nil, err
	}

	var out Subscription
	raw, err := core.GetJSON(ctx, "/v1/user/subscription", &out)
	if err != nil {
		return nil, err
	}

	return &Response[*Subscription]{
		Data:        &out,
		RawResponse: raw,
	}, nil
}
