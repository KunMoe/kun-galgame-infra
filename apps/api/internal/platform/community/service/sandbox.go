package service

import (
	"api/internal/platform/community/model"
	"api/internal/platform/community/repository"
	"api/internal/platform/community/sanitize"
	"api/internal/platform/settings/keys"
)

func isSandboxed(level int16) bool { return level < model.TrustLevelBasic }

func checkContentSandbox(level int16, cooked sanitize.Cooked) error {
	if !isSandboxed(level) {
		return nil
	}
	switch {
	case cooked.Links > int(keys.CommunitySandboxMaxLinks.Get()):
		return &SandboxError{Reason: "too many links"}
	case cooked.Images > int(keys.CommunitySandboxMaxImages.Get()):
		return &SandboxError{Reason: "too many images"}
	case cooked.Mentions > int(keys.CommunitySandboxMaxMentions.Get()):
		return &SandboxError{Reason: "too many mentions"}
	}
	return nil
}

func trustLevel(trusts *repository.TrustRepository, userID int64) (int16, error) {
	t, err := trusts.GetTrust(userID)
	if err != nil {
		return 0, err
	}
	if t == nil {
		return model.TrustLevelNew, nil
	}
	return t.Level, nil
}
