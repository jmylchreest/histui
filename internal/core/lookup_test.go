package core

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jmylchreest/histui/internal/model"
)

func TestLookupByID(t *testing.T) {
	notifications := []model.Notification{
		{HistuiID: "abc123", AppName: "firefox"},
		{HistuiID: "def456", AppName: "slack"},
		{HistuiID: "ghi789", AppName: "discord"},
	}

	t.Run("found", func(t *testing.T) {
		result := LookupByID(notifications, "def456")
		assert.NotNil(t, result)
		assert.Equal(t, "slack", result.AppName)
	})

	t.Run("not found", func(t *testing.T) {
		result := LookupByID(notifications, "notexist")
		assert.Nil(t, result)
	})

	t.Run("empty slice", func(t *testing.T) {
		result := LookupByID(nil, "abc123")
		assert.Nil(t, result)
	})
}

func TestLookupByIndex(t *testing.T) {
	notifications := []model.Notification{
		{HistuiID: "1", AppName: "firefox"},
		{HistuiID: "2", AppName: "slack"},
		{HistuiID: "3", AppName: "discord"},
	}

	t.Run("valid index 1", func(t *testing.T) {
		result := LookupByIndex(notifications, 1)
		assert.NotNil(t, result)
		assert.Equal(t, "firefox", result.AppName)
	})

	t.Run("valid index 3", func(t *testing.T) {
		result := LookupByIndex(notifications, 3)
		assert.NotNil(t, result)
		assert.Equal(t, "discord", result.AppName)
	})

	t.Run("index 0 out of bounds", func(t *testing.T) {
		result := LookupByIndex(notifications, 0)
		assert.Nil(t, result)
	})

	t.Run("negative index", func(t *testing.T) {
		result := LookupByIndex(notifications, -1)
		assert.Nil(t, result)
	})

	t.Run("index too high", func(t *testing.T) {
		result := LookupByIndex(notifications, 10)
		assert.Nil(t, result)
	})

	t.Run("empty slice", func(t *testing.T) {
		result := LookupByIndex(nil, 1)
		assert.Nil(t, result)
	})
}

func TestSearch(t *testing.T) {
	notifications := []model.Notification{
		{HistuiID: "1", Summary: "Download Complete", Body: "file.zip finished"},
		{HistuiID: "2", Summary: "New Message", Body: "Hello from John"},
		{HistuiID: "3", Summary: "Update Available", Body: "Firefox has updates"},
	}

	t.Run("match in summary", func(t *testing.T) {
		result := Search(notifications, "download")
		assert.Len(t, result, 1)
		assert.Equal(t, "1", result[0].HistuiID)
	})

	t.Run("match in body", func(t *testing.T) {
		result := Search(notifications, "john")
		assert.Len(t, result, 1)
		assert.Equal(t, "2", result[0].HistuiID)
	})

	t.Run("case insensitive", func(t *testing.T) {
		result := Search(notifications, "FIREFOX")
		assert.Len(t, result, 1)
		assert.Equal(t, "3", result[0].HistuiID)
	})

	t.Run("multiple matches", func(t *testing.T) {
		// "e" appears in multiple notifications
		result := Search(notifications, "e")
		assert.Len(t, result, 3)
	})

	t.Run("no matches", func(t *testing.T) {
		result := Search(notifications, "xyz123")
		assert.Len(t, result, 0)
	})

	t.Run("empty search term returns all", func(t *testing.T) {
		result := Search(notifications, "")
		assert.Len(t, result, 3)
	})
}

func TestUniqueApps(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		notifications := []model.Notification{
			{AppName: "Firefox"},
			{AppName: "Slack"},
			{AppName: "Firefox"},
			{AppName: "Discord"},
		}

		apps := UniqueApps(notifications)
		assert.Len(t, apps, 3)
		assert.Equal(t, "Discord", apps[0])
		assert.Equal(t, "Firefox", apps[1])
		assert.Equal(t, "Slack", apps[2])
	})

	t.Run("empty app names excluded", func(t *testing.T) {
		notifications := []model.Notification{
			{AppName: "Firefox"},
			{AppName: ""},
			{AppName: "Slack"},
		}

		apps := UniqueApps(notifications)
		assert.Len(t, apps, 2)
	})

	t.Run("empty slice", func(t *testing.T) {
		apps := UniqueApps(nil)
		assert.Len(t, apps, 0)
	})
}
