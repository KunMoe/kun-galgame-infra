package main

import (
	"context"
	"fmt"

	"api/internal/platform/catalog/editspec"

	"gorm.io/gorm"
)

// Panel bucket (adjudicated refs/proj/178: narrow (c)): the roster is every
// character whose elected zh-Hans intro is a TRANSLATED machine row. A source
// row (provenance 0) excludes the character — machine output never competes
// with source text. An existing derived row means the panel already ran.
var candidatePanelWorksSQL = `
	WITH zhi AS (
		SELECT DISTINCT ON (work_id) work_id, intro, updated_at
		FROM catalog_work_intro
		WHERE lang = 'zh-Hans' AND length(intro) >= 200
		ORDER BY work_id, source_id
	), inc AS (
		SELECT DISTINCT ON (character_id) character_id, intro
		FROM catalog_character_intro
		WHERE lang = 'zh-Hans' AND provenance = 1 AND source_id <> 18
		ORDER BY character_id, source_id
	)
	SELECT w.id AS work_id, zhi.intro,
	       wc.character_id, c.display_name,
	       COALESCE((
	         SELECT a.name FROM catalog_character_alias a
	         WHERE a.character_id = c.id AND a.lang IN ('zh-Hans','zh','zh-Hant') AND a.kind IN (0,1)
	         ORDER BY (NOT a.is_primary_for_locale), (a.lang <> 'zh-Hans'), a.id LIMIT 1
	       ), '') AS zh_name,
	       inc.intro AS incumbent
	FROM catalog_work w
	JOIN zhi ON zhi.work_id = w.id
	JOIN catalog_work_character wc ON wc.work_id = w.id
	JOIN catalog_character c ON c.id = wc.character_id AND c.deleted_at IS NULL
	JOIN inc ON inc.character_id = wc.character_id
	WHERE w.deleted_at IS NULL
	  AND ` + editspec.NotSuppressedRosterSQL("wc") + `
	  AND NOT EXISTS (
	    SELECT 1 FROM catalog_character_intro s
	    WHERE s.character_id = wc.character_id AND s.lang = 'zh-Hans' AND s.provenance = 0)
	  AND NOT EXISTS (
	    SELECT 1 FROM catalog_character_intro d
	    WHERE d.character_id = wc.character_id AND d.lang = 'zh-Hans' AND d.source_id = 18)`

func loadPanelCandidateWorks(ctx context.Context, db *gorm.DB, o candidateOpts) ([]candidateWork, error) {
	sql := candidatePanelWorksSQL + sinceClause(o.Since) + `
	ORDER BY w.id, wc.character_id`
	var rows []struct {
		WorkID      int64  `gorm:"column:work_id"`
		Intro       string `gorm:"column:intro"`
		CharacterID int64  `gorm:"column:character_id"`
		DisplayName string `gorm:"column:display_name"`
		ZhName      string `gorm:"column:zh_name"`
		Incumbent   string `gorm:"column:incumbent"`
	}
	if err := db.WithContext(ctx).Raw(sql, sinceArgs(o.Since)...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load panel candidate works: %w", err)
	}
	var out []candidateWork
	for _, r := range rows {
		if len(out) == 0 || out[len(out)-1].WorkID != r.WorkID {
			out = append(out, candidateWork{WorkID: r.WorkID, Intro: r.Intro})
		}
		cur := &out[len(out)-1]
		cur.Roster = append(cur.Roster, rosterChar{
			CharacterID: r.CharacterID, Name: r.DisplayName, ZhName: r.ZhName, Incumbent: r.Incumbent,
		})
	}
	return window(out, o), nil
}

type panelVote int

const (
	voteIncumbent panelVote = iota
	voteChallenger
	voteEqual
)

// comparison is one vote to cast: which passage serves better as this
// character's introduction. ChallengerFirst decides which one is shown as A,
// so a position bias cannot decide the verdict.
type comparison struct {
	Name            string
	Incumbent       string
	Challenger      string
	ChallengerFirst bool
}

type comparisonResult struct {
	Vote panelVote
	Err  error
}

type panelJudge interface {
	// CompareBatch answers one vote per comparison, in order.
	CompareBatch(ctx context.Context, batch []comparison) []comparisonResult
}

const panelVotes = 3

// panelVerdict folds three votes by direction: equal votes are not opposing
// votes, but adoption needs a unanimous direction with at least two positive
// votes — any vote for the incumbent keeps it.
func panelVerdict(votes []panelVote) bool {
	var challenger, incumbent int
	for _, v := range votes {
		switch v {
		case voteChallenger:
			challenger++
		case voteIncumbent:
			incumbent++
		}
	}
	return incumbent == 0 && challenger >= 2
}

const judgeSystemPrompt = `你在为视觉小说(galgame)数据库挑选更好的一段中文角色介绍。每个条目给出角色名和两段候选文本 A、B。
判据(按重要性):
1. 介绍性:说明角色的身份、性格、外貌或与他人的关系。纯剧情叙述、只在事件中顺带提到角色的文本不合格。
2. 信息量:具体、有内容,不是空话。
3. 通顺自然的中文;机翻腔、辞不达意、残缺句是硬伤。
两段都合格且相当时输出 equal。每项的 winner 取值只能是 "A"、"B"、"equal" 三者之一。`

// voteOf maps the judge's answer back onto the two passages. Anything but
// A/B/equal lands on equal, and the batch lane turns that into an error.
func voteOf(winner string, challengerFirst bool) panelVote {
	switch winner {
	case "A":
		if challengerFirst {
			return voteChallenger
		}
		return voteIncumbent
	case "B":
		if challengerFirst {
			return voteIncumbent
		}
		return voteChallenger
	}
	return voteEqual
}
