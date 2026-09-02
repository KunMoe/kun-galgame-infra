package service

import (
	"context"
	"testing"

	"api/internal/platform/community/model"
	"api/internal/platform/settings"
	"api/internal/platform/settings/keys"
)

func TestNewcomerPostsStraightThrough(t *testing.T) {
	cleanTables(t)
	ts := NewThreadService(testDB, NoopSink{})
	ps := NewPostService(testDB, NoopSink{})
	th := openTopic(t, ts, "letmoe", 100, "b1", "opening")

	const newcomer int64 = 777
	for i, body := range []string{"first ever post", "second ever post"} {
		post, err := ps.Reply(context.Background(), ReplyParams{ThreadID: th.ID, AuthorID: newcomer, BodyRaw: body})
		if err != nil {
			t.Fatalf("reply %d: %v", i+1, err)
		}
		if post.Status != model.PostStatusVisible {
			t.Fatalf("post %d status = %d, want visible — the newcomer hold is retired", i+1, post.Status)
		}
		if item := pendingReviewForPost(t, post.ID); item != nil {
			t.Fatalf("post %d enqueued a review item (%+v); nothing should hold a clean newcomer", i+1, item)
		}
	}

	if r := getTrust(t, newcomer).FirstPostsHeldRemaining; r != 0 {
		t.Fatalf("fresh trust row hold budget = %d, want 0", r)
	}
}

func TestTier0HoldStillEnqueues(t *testing.T) {
	settings.Override(t, keys.TrustCheckEnabled, true)
	cleanTables(t)
	ts := NewThreadService(testDB, NoopSink{})
	ps := NewPostService(testDB, NoopSink{}, WithPostChecker(NewCheckService(&fakeChecker{decision: checkHold})))
	th := openTopic(t, ts, "letmoe", 100, "b1", "opening")

	post, err := ps.Reply(context.Background(), ReplyParams{ThreadID: th.ID, AuthorID: 778, BodyRaw: "suspect words here"})
	if err != nil {
		t.Fatalf("reply: %v", err)
	}
	if post.Status != model.PostStatusVisible {
		t.Fatalf("suspect post status = %d, want visible", post.Status)
	}
	item := pendingReviewForPost(t, post.ID)
	if item == nil || item.Source == nil || *item.Source != model.ReviewSourceSuspectWords {
		t.Fatalf("suspect post should enqueue a suspect_words item, got %+v", item)
	}
}
