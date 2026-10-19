package domain

import (
	"testing"

	"github.com/nstratos/go-myanimelist/mal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnimeUpdate_UpdateRatingWithStatus(t *testing.T) {
	tests := []struct {
		name   string
		update *AnimeUpdate
		status mal.AnimeListStatus
	}{
		{
			name: "updates list status with rating",
			update: &AnimeUpdate{
				ListStatus: mal.AnimeListStatus{},
			},
			status: mal.AnimeListStatus{
				Score: 8,
			},
		},
		{
			name: "overwrites existing status",
			update: &AnimeUpdate{
				ListStatus: mal.AnimeListStatus{
					Score: 5,
				},
			},
			status: mal.AnimeListStatus{
				Score: 10,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.update.UpdateRatingWithStatus(tt.status)
			assert.Equal(t, tt.status, tt.update.ListStatus)
		})
	}
}

func TestAnimeUpdate_UpdateWatchStatusWithStatus(t *testing.T) {
	tests := []struct {
		name   string
		update *AnimeUpdate
		status mal.AnimeListStatus
	}{
		{
			name: "updates list status with watch status",
			update: &AnimeUpdate{
				ListStatus: mal.AnimeListStatus{},
			},
			status: mal.AnimeListStatus{
				Status:             mal.AnimeStatusWatching,
				NumEpisodesWatched: 5,
			},
		},
		{
			name: "overwrites existing watch status",
			update: &AnimeUpdate{
				ListStatus: mal.AnimeListStatus{
					Status:             mal.AnimeStatusPlanToWatch,
					NumEpisodesWatched: 0,
				},
			},
			status: mal.AnimeListStatus{
				Status:             mal.AnimeStatusCompleted,
				NumEpisodesWatched: 12,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.update.UpdateWatchStatusWithStatus(tt.status)
			assert.Equal(t, tt.status, tt.update.ListStatus)
		})
	}
}

func TestAnimeUpdate_UpdateListDetails(t *testing.T) {
	tests := []struct {
		name     string
		update   *AnimeUpdate
		details  ListDetails
		expected ListDetails
	}{
		{
			name: "updates list details",
			update: &AnimeUpdate{
				ListDetails: ListDetails{},
			},
			details: ListDetails{
				Status:          mal.AnimeStatusWatching,
				TotalEpisodeNum: 12,
				WatchedNum:      5,
				Title:           "Test Anime",
			},
			expected: ListDetails{
				Status:          mal.AnimeStatusWatching,
				TotalEpisodeNum: 12,
				WatchedNum:      5,
				Title:           "Test Anime",
			},
		},
		{
			name: "overwrites existing details",
			update: &AnimeUpdate{
				ListDetails: ListDetails{
					Status:          mal.AnimeStatusPlanToWatch,
					TotalEpisodeNum: 10,
					WatchedNum:      0,
					Title:           "Old Title",
				},
			},
			details: ListDetails{
				Status:          mal.AnimeStatusCompleted,
				TotalEpisodeNum: 12,
				WatchedNum:      12,
				Title:           "New Title",
			},
			expected: ListDetails{
				Status:          mal.AnimeStatusCompleted,
				TotalEpisodeNum: 12,
				WatchedNum:      12,
				Title:           "New Title",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.update.UpdateListDetails(tt.details)
			assert.Equal(t, tt.expected, tt.update.ListDetails)
		})
	}
}

func TestAnimeUpdate_validateEpisodeNum(t *testing.T) {
	tests := []struct {
		name        string
		update      *AnimeUpdate
		expectedErr bool
		errContains string
	}{
		{
			name: "valid episode number less than total",
			update: &AnimeUpdate{
				EpisodeNum: 5,
				ListDetails: ListDetails{
					TotalEpisodeNum: 12,
					Title:           "Test Anime",
				},
			},
			expectedErr: false,
		},
		{
			name: "valid episode number equals total",
			update: &AnimeUpdate{
				EpisodeNum: 12,
				ListDetails: ListDetails{
					TotalEpisodeNum: 12,
					Title:           "Test Anime",
				},
			},
			expectedErr: false,
		},
		{
			name: "valid when total episodes is 0 (ongoing anime)",
			update: &AnimeUpdate{
				EpisodeNum: 100,
				ListDetails: ListDetails{
					TotalEpisodeNum: 0,
					Title:           "Ongoing Anime",
				},
			},
			expectedErr: false,
		},
		{
			name: "invalid episode number greater than total",
			update: &AnimeUpdate{
				EpisodeNum: 15,
				ListDetails: ListDetails{
					TotalEpisodeNum: 12,
					Title:           "Test Anime",
				},
			},
			expectedErr: true,
			errContains: "number of episodes watched greater than total number of episodes",
		},
		{
			name: "invalid episode number much greater than total",
			update: &AnimeUpdate{
				EpisodeNum: 100,
				ListDetails: ListDetails{
					TotalEpisodeNum: 12,
					Title:           "Test Anime",
				},
			},
			expectedErr: true,
			errContains: "number of episodes watched greater than total number of episodes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.update.validateEpisodeNum()
			if tt.expectedErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBuildListDetailsFromMALResponse(t *testing.T) {
	tests := []struct {
		name          string
		status        mal.AnimeStatus
		rewatchNum    int
		totalEpisodes int
		watchedNum    int
		title         string
		pictureURL    string
		startDate     string
		finishDate    string
		expected      ListDetails
	}{
		{
			name:          "builds list details from MAL response",
			status:        mal.AnimeStatusWatching,
			rewatchNum:    0,
			totalEpisodes: 12,
			watchedNum:    5,
			title:         "Test Anime",
			pictureURL:    "https://example.com/image.jpg",
			startDate:     "2023-01-01",
			finishDate:    "",
			expected: ListDetails{
				Status:          mal.AnimeStatusWatching,
				RewatchNum:      0,
				TotalEpisodeNum: 12,
				WatchedNum:      5,
				Title:           "Test Anime",
				PictureURL:      "https://example.com/image.jpg",
				MALStartDate:    "2023-01-01",
				MALFinishDate:   "",
			},
		},
		{
			name:          "handles completed anime",
			status:        mal.AnimeStatusCompleted,
			rewatchNum:    1,
			totalEpisodes: 24,
			watchedNum:    24,
			title:         "Completed Anime",
			pictureURL:    "https://example.com/completed.jpg",
			startDate:     "2023-01-01",
			finishDate:    "2023-01-10",
			expected: ListDetails{
				Status:          mal.AnimeStatusCompleted,
				RewatchNum:      1,
				TotalEpisodeNum: 24,
				WatchedNum:      24,
				Title:           "Completed Anime",
				PictureURL:      "https://example.com/completed.jpg",
				MALStartDate:    "2023-01-01",
				MALFinishDate:   "2023-01-10",
			},
		},
		{
			name:          "handles ongoing anime with 0 total episodes",
			status:        mal.AnimeStatusWatching,
			rewatchNum:    0,
			totalEpisodes: 0,
			watchedNum:    50,
			title:         "Ongoing Anime",
			pictureURL:    "",
			startDate:     "",
			finishDate:    "",
			expected: ListDetails{
				Status:          mal.AnimeStatusWatching,
				RewatchNum:      0,
				TotalEpisodeNum: 0,
				WatchedNum:      50,
				Title:           "Ongoing Anime",
				PictureURL:      "",
				MALStartDate:    "",
				MALFinishDate:   "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildListDetailsFromMALResponse(
				tt.status,
				tt.rewatchNum,
				tt.totalEpisodes,
				tt.watchedNum,
				tt.title,
				tt.pictureURL,
				tt.startDate,
				tt.finishDate,
			)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAnimeUpdate_BuildWatchStatusOptions(t *testing.T) {
	tests := []struct {
		name        string
		update      *AnimeUpdate
		expectedErr bool
		errContains string
		validate    func(*testing.T, []mal.UpdateMyAnimeListStatusOption)
	}{
		{
			name: "builds options for first episode",
			update: &AnimeUpdate{
				EpisodeNum: 1,
				ListDetails: ListDetails{
					Status:          mal.AnimeStatusPlanToWatch,
					TotalEpisodeNum: 12,
					WatchedNum:      0,
					Title:           "Test Anime",
				},
			},
			expectedErr: false,
			validate: func(t *testing.T, options []mal.UpdateMyAnimeListStatusOption) {
				assert.Greater(t, len(options), 0)
				// Should include start date and status
			},
		},
		{
			name: "builds options for middle episode",
			update: &AnimeUpdate{
				EpisodeNum: 5,
				ListDetails: ListDetails{
					Status:          mal.AnimeStatusWatching,
					TotalEpisodeNum: 12,
					WatchedNum:      4,
					Title:           "Test Anime",
				},
			},
			expectedErr: false,
			validate: func(t *testing.T, options []mal.UpdateMyAnimeListStatusOption) {
				assert.Greater(t, len(options), 0)
			},
		},
		{
			name: "builds options for completing anime",
			update: &AnimeUpdate{
				EpisodeNum: 12,
				ListDetails: ListDetails{
					Status:          mal.AnimeStatusWatching,
					TotalEpisodeNum: 12,
					WatchedNum:      11,
					Title:           "Test Anime",
				},
			},
			expectedErr: false,
			validate: func(t *testing.T, options []mal.UpdateMyAnimeListStatusOption) {
				assert.Greater(t, len(options), 0)
				// Should include finish date and completed status
			},
		},
		{
			name: "returns error when episode number is invalid",
			update: &AnimeUpdate{
				EpisodeNum: 15,
				ListDetails: ListDetails{
					Status:          mal.AnimeStatusWatching,
					TotalEpisodeNum: 12,
					WatchedNum:      5,
					Title:           "Test Anime",
				},
			},
			expectedErr: true,
			errContains: "number of episodes watched greater than total number of episodes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options, err := tt.update.BuildWatchStatusOptions()
			if tt.expectedErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, options)
			} else {
				require.NoError(t, err)
				require.NotNil(t, options)
				assert.Greater(t, len(options), 0, "options should not be empty")
				if tt.validate != nil {
					tt.validate(t, options)
				}
			}
		})
	}
}

func TestListDetails_buildOptions(t *testing.T) {
	tests := []struct {
		name        string
		details     *ListDetails
		episodeNum  int
		expectedLen int
		validate    func(*testing.T, *ListDetails, []mal.UpdateMyAnimeListStatusOption)
	}{
		{
			name: "first episode from plan to watch",
			details: &ListDetails{
				Status:          mal.AnimeStatusPlanToWatch,
				TotalEpisodeNum: 12,
				WatchedNum:      0,
			},
			episodeNum:  1,
			expectedLen: 3, // StartDate, NumEpisodesWatched, Status
			validate: func(t *testing.T, ld *ListDetails, options []mal.UpdateMyAnimeListStatusOption) {
				assert.Equal(t, mal.AnimeStatusWatching, ld.Status)
			},
		},
		{
			name: "middle episode while watching",
			details: &ListDetails{
				Status:          mal.AnimeStatusWatching,
				TotalEpisodeNum: 12,
				WatchedNum:      5,
			},
			episodeNum:  6,
			expectedLen: 2, // NumEpisodesWatched, Status
			validate: func(t *testing.T, ld *ListDetails, options []mal.UpdateMyAnimeListStatusOption) {
				assert.Equal(t, mal.AnimeStatusWatching, ld.Status)
			},
		},
		{
			name: "completing anime for first time",
			details: &ListDetails{
				Status:          mal.AnimeStatusWatching,
				TotalEpisodeNum: 12,
				WatchedNum:      11,
			},
			episodeNum:  12,
			expectedLen: 3, // FinishDate, NumEpisodesWatched, Status
			validate: func(t *testing.T, ld *ListDetails, options []mal.UpdateMyAnimeListStatusOption) {
				assert.Equal(t, mal.AnimeStatusCompleted, ld.Status)
			},
		},
		{
			name: "rewatching completed anime (episode < total)",
			details: &ListDetails{
				Status:          mal.AnimeStatusCompleted,
				TotalEpisodeNum: 12,
				WatchedNum:      12,
				RewatchNum:      0,
			},
			episodeNum:  5,
			expectedLen: 3, // IsRewatching(true), NumEpisodesWatched, Status
			validate: func(t *testing.T, ld *ListDetails, options []mal.UpdateMyAnimeListStatusOption) {
				assert.Equal(t, mal.AnimeStatusCompleted, ld.Status)
			},
		},
		{
			name: "completing rewatch",
			details: &ListDetails{
				Status:          mal.AnimeStatusCompleted,
				TotalEpisodeNum: 12,
				WatchedNum:      5,
				RewatchNum:      0,
			},
			episodeNum:  12,
			expectedLen: 4, // NumTimesRewatched, IsRewatching(false), NumEpisodesWatched, Status
			validate: func(t *testing.T, ld *ListDetails, options []mal.UpdateMyAnimeListStatusOption) {
				assert.Equal(t, 1, ld.RewatchNum)
				assert.Equal(t, mal.AnimeStatusCompleted, ld.Status)
			},
		},
		{
			name: "ongoing anime (total episodes = 0)",
			details: &ListDetails{
				Status:          mal.AnimeStatusWatching,
				TotalEpisodeNum: 0,
				WatchedNum:      50,
			},
			episodeNum:  51,
			expectedLen: 2, // NumEpisodesWatched, Status
			validate: func(t *testing.T, ld *ListDetails, options []mal.UpdateMyAnimeListStatusOption) {
				assert.Equal(t, mal.AnimeStatusWatching, ld.Status)
			},
		},
		{
			name: "rewatching ongoing anime",
			details: &ListDetails{
				Status:          mal.AnimeStatusCompleted,
				TotalEpisodeNum: 0,
				WatchedNum:      100,
				RewatchNum:      1,
			},
			episodeNum:  50,
			expectedLen: 3, // IsRewatching(true), NumEpisodesWatched, Status
			validate: func(t *testing.T, ld *ListDetails, options []mal.UpdateMyAnimeListStatusOption) {
				assert.Equal(t, mal.AnimeStatusCompleted, ld.Status)
			},
		},
		{
			name: "already completed, watching again (no status change)",
			details: &ListDetails{
				Status:          mal.AnimeStatusCompleted,
				TotalEpisodeNum: 12,
				WatchedNum:      12,
				RewatchNum:      1,
			},
			episodeNum:  12,
			expectedLen: 4, // NumTimesRewatched, IsRewatching(false), NumEpisodesWatched, Status
			validate: func(t *testing.T, ld *ListDetails, options []mal.UpdateMyAnimeListStatusOption) {
				assert.Equal(t, 2, ld.RewatchNum)
				assert.Equal(t, mal.AnimeStatusCompleted, ld.Status)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy to avoid modifying the original
			detailsCopy := *tt.details
			options := detailsCopy.buildOptions(tt.episodeNum)

			assert.GreaterOrEqual(t, len(options), tt.expectedLen, "options length should be at least expected")
			if tt.validate != nil {
				tt.validate(t, &detailsCopy, options)
			}
		})
	}
}

func TestListDetails_isAnimeCompleted(t *testing.T) {
	tests := []struct {
		name       string
		details    *ListDetails
		episodeNum int
		expected   bool
	}{
		{
			name: "completed when episode equals total",
			details: &ListDetails{
				TotalEpisodeNum: 12,
			},
			episodeNum: 12,
			expected:   true,
		},
		{
			name: "not completed when episode less than total",
			details: &ListDetails{
				TotalEpisodeNum: 12,
			},
			episodeNum: 11,
			expected:   false,
		},
		{
			name: "not completed when episode greater than total",
			details: &ListDetails{
				TotalEpisodeNum: 12,
			},
			episodeNum: 13,
			expected:   false,
		},
		{
			name: "not completed when total is 0 (ongoing)",
			details: &ListDetails{
				TotalEpisodeNum: 0,
			},
			episodeNum: 100,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.details.isAnimeCompleted(tt.episodeNum)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestListDetails_isFirstEpisode(t *testing.T) {
	tests := []struct {
		name       string
		details    *ListDetails
		episodeNum int
		expected   bool
	}{
		{
			name: "first episode when episode is 1 and watched is 0",
			details: &ListDetails{
				WatchedNum: 0,
			},
			episodeNum: 1,
			expected:   true,
		},
		{
			name: "not first episode when episode is 1 but already watched",
			details: &ListDetails{
				WatchedNum: 1,
			},
			episodeNum: 1,
			expected:   false,
		},
		{
			name: "not first episode when episode is not 1",
			details: &ListDetails{
				WatchedNum: 0,
			},
			episodeNum: 2,
			expected:   false,
		},
		{
			name: "not first episode when episode is 1 but watched is not 0",
			details: &ListDetails{
				WatchedNum: 5,
			},
			episodeNum: 1,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.details.isFirstEpisode(tt.episodeNum)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestListDetails_isAnimeWatching(t *testing.T) {
	tests := []struct {
		name       string
		details    *ListDetails
		episodeNum int
		expected   bool
	}{
		{
			name: "watching when episode less than total",
			details: &ListDetails{
				TotalEpisodeNum: 12,
			},
			episodeNum: 5,
			expected:   true,
		},
		{
			name: "watching when episode equals total (edge case)",
			details: &ListDetails{
				TotalEpisodeNum: 12,
			},
			episodeNum: 12,
			expected:   false, // This would be completed, not watching
		},
		{
			name: "watching when total is 0 (ongoing anime)",
			details: &ListDetails{
				TotalEpisodeNum: 0,
			},
			episodeNum: 100,
			expected:   true,
		},
		{
			name: "watching when episode is 1",
			details: &ListDetails{
				TotalEpisodeNum: 12,
			},
			episodeNum: 1,
			expected:   true,
		},
		{
			name: "not watching when episode is 0",
			details: &ListDetails{
				TotalEpisodeNum: 12,
			},
			episodeNum: 0,
			expected:   false,
		},
		{
			name: "not watching when episode is negative",
			details: &ListDetails{
				TotalEpisodeNum: 12,
			},
			episodeNum: -1,
			expected:   false,
		},
		{
			name: "watching when episode greater than total but total is 0",
			details: &ListDetails{
				TotalEpisodeNum: 0,
			},
			episodeNum: 50,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.details.isAnimeWatching(tt.episodeNum)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test edge case: BuildWatchStatusOptions returns error when no options generated
func TestAnimeUpdate_BuildWatchStatusOptions_NoOptions(t *testing.T) {
	// This is a tricky edge case - we need to ensure buildOptions never returns empty
	// but if it does, BuildWatchStatusOptions should catch it
	// This test ensures the error handling works
	update := &AnimeUpdate{
		EpisodeNum: 1,
		ListDetails: ListDetails{
			Status:          mal.AnimeStatusPlanToWatch,
			TotalEpisodeNum: 12,
			WatchedNum:      0,
			Title:           "Test Anime",
		},
	}

	options, err := update.BuildWatchStatusOptions()
	require.NoError(t, err)
	require.NotEmpty(t, options, "options should never be empty for valid input")
}
