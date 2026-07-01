package elevenlabs

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

// VoicesService provides ElevenLabs voice APIs.
type VoicesService struct {
	client *Client
}

// ListShared retrieves a page of shared voices from the voice library.
func (s *VoicesService) ListShared(ctx context.Context, in ListSharedVoicesRequest) (*SharedVoicesResponse, error) {
	resp, err := s.ListSharedWithResponse(ctx, in)
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// ListSharedWithResponse retrieves a page of shared voices from the voice
// library and returns HTTP response metadata.
func (s *VoicesService) ListSharedWithResponse(ctx context.Context, in ListSharedVoicesRequest) (*Response[*SharedVoicesResponse], error) {
	core, err := s.apiClient()
	if err != nil {
		return nil, err
	}

	var out SharedVoicesResponse
	raw, err := core.GetJSON(ctx, sharedVoicesPath(in), &out)
	if err != nil {
		return nil, err
	}

	return &Response[*SharedVoicesResponse]{
		Data:        &out,
		RawResponse: raw,
	}, nil
}

func (s *VoicesService) apiClient() (*Client, error) {
	if s == nil || s.client == nil {
		return nil, errors.New("elevenlabs: nil client")
	}
	return s.client, nil
}

func sharedVoicesPath(in ListSharedVoicesRequest) string {
	path := "/v1/shared-voices"
	values := url.Values{}
	setIntQuery(values, "page_size", in.PageSize)
	setStringQuery(values, "category", in.Category)
	setStringQuery(values, "gender", in.Gender)
	setStringQuery(values, "age", in.Age)
	setStringQuery(values, "accent", in.Accent)
	setStringQuery(values, "language", in.Language)
	setStringQuery(values, "locale", in.Locale)
	setStringQuery(values, "search", in.Search)
	addStringListQuery(values, "use_cases", in.UseCases)
	addStringListQuery(values, "descriptives", in.Descriptives)
	setBoolQuery(values, "featured", in.Featured)
	setIntQuery(values, "min_notice_period_days", in.MinNoticePeriodDays)
	setBoolQuery(values, "include_custom_rates", in.IncludeCustomRates)
	setBoolQuery(values, "include_live_moderated", in.IncludeLiveModerated)
	setBoolQuery(values, "reader_app_enabled", in.ReaderAppEnabled)
	setStringQuery(values, "owner_id", in.OwnerID)
	setStringQuery(values, "sort", in.Sort)
	setIntQuery(values, "page", in.Page)
	if len(values) == 0 {
		return path
	}
	return path + "?" + values.Encode()
}

func addStringListQuery(query url.Values, name string, values []string) {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			query.Add(name, value)
		}
	}
}
