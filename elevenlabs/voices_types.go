package elevenlabs

// ListSharedVoicesCategory filters shared voices by library category.
type ListSharedVoicesCategory string

const (
	ListSharedVoicesCategoryProfessional ListSharedVoicesCategory = "professional"
	ListSharedVoicesCategoryFamous       ListSharedVoicesCategory = "famous"
	ListSharedVoicesCategoryHighQuality  ListSharedVoicesCategory = "high_quality"
)

// ListSharedVoicesSort identifies the sort criteria for shared voices.
type ListSharedVoicesSort string

const (
	ListSharedVoicesSortCreatedDate           ListSharedVoicesSort = "created_date"
	ListSharedVoicesSortUsageCharacterCount1Y ListSharedVoicesSort = "usage_character_count_1y"
	ListSharedVoicesSortTrending              ListSharedVoicesSort = "trending"
	ListSharedVoicesSortClonedByCount         ListSharedVoicesSort = "cloned_by_count"
)

// ListSharedVoicesRequest contains filters and pagination settings for shared
// voices.
type ListSharedVoicesRequest struct {
	Accent               string
	Age                  string
	Category             ListSharedVoicesCategory
	Descriptives         []string
	Featured             *bool
	Gender               string
	IncludeCustomRates   *bool
	IncludeLiveModerated *bool
	Language             string
	Locale               string
	MinNoticePeriodDays  *int
	OwnerID              string
	Page                 *int
	PageSize             *int
	ReaderAppEnabled     *bool
	Search               string
	Sort                 ListSharedVoicesSort
	UseCases             []string
}

// SharedVoiceCategory is the category assigned to a shared voice.
type SharedVoiceCategory string

const (
	SharedVoiceCategoryGenerated    SharedVoiceCategory = "generated"
	SharedVoiceCategoryCloned       SharedVoiceCategory = "cloned"
	SharedVoiceCategoryPremade      SharedVoiceCategory = "premade"
	SharedVoiceCategoryProfessional SharedVoiceCategory = "professional"
	SharedVoiceCategoryFamous       SharedVoiceCategory = "famous"
	SharedVoiceCategoryHighQuality  SharedVoiceCategory = "high_quality"
)

// VerifiedVoiceLanguage contains metadata for a verified language available on
// a shared voice.
type VerifiedVoiceLanguage struct {
	Accent     *string `json:"accent,omitempty"`
	Language   string  `json:"language"`
	Locale     *string `json:"locale,omitempty"`
	ModelID    string  `json:"model_id"`
	PreviewURL *string `json:"preview_url,omitempty"`
}

// SharedVoice contains metadata for a voice returned by the shared voice
// library.
type SharedVoice struct {
	Accent                       string                  `json:"accent"`
	Age                          string                  `json:"age"`
	Category                     SharedVoiceCategory     `json:"category"`
	ClonedByCount                int64                   `json:"cloned_by_count"`
	DateUnix                     int64                   `json:"date_unix"`
	Description                  *string                 `json:"description,omitempty"`
	Descriptive                  string                  `json:"descriptive"`
	Featured                     bool                    `json:"featured"`
	FiatRate                     *float64                `json:"fiat_rate,omitempty"`
	FreeUsersAllowed             bool                    `json:"free_users_allowed"`
	Gender                       string                  `json:"gender"`
	ImageURL                     *string                 `json:"image_url,omitempty"`
	InstagramUsername            *string                 `json:"instagram_username,omitempty"`
	IsAddedByUser                *bool                   `json:"is_added_by_user,omitempty"`
	IsBookmarked                 *bool                   `json:"is_bookmarked,omitempty"`
	Language                     *string                 `json:"language,omitempty"`
	LiveModerationEnabled        bool                    `json:"live_moderation_enabled"`
	Locale                       *string                 `json:"locale,omitempty"`
	Name                         string                  `json:"name"`
	NoticePeriod                 *int                    `json:"notice_period,omitempty"`
	PlayAPIUsageCharacterCount1Y int64                   `json:"play_api_usage_character_count_1y"`
	PreviewURL                   *string                 `json:"preview_url,omitempty"`
	PublicOwnerID                string                  `json:"public_owner_id"`
	Rate                         *float64                `json:"rate,omitempty"`
	TiktokUsername               *string                 `json:"tiktok_username,omitempty"`
	TwitterUsername              *string                 `json:"twitter_username,omitempty"`
	UsageCharacterCount1Y        int64                   `json:"usage_character_count_1y"`
	UsageCharacterCount7D        int64                   `json:"usage_character_count_7d"`
	UseCase                      string                  `json:"use_case"`
	VerifiedLanguages            []VerifiedVoiceLanguage `json:"verified_languages,omitempty"`
	VoiceID                      string                  `json:"voice_id"`
	YoutubeUsername              *string                 `json:"youtube_username,omitempty"`
}

// SharedVoicesResponse is a page of shared voices returned by the voice
// library.
type SharedVoicesResponse struct {
	HasMore    bool          `json:"has_more"`
	LastSortID *string       `json:"last_sort_id,omitempty"`
	TotalCount int64         `json:"total_count,omitempty"`
	Voices     []SharedVoice `json:"voices"`
}
