package intromt

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Work 94, the case that opened this: two real sentences under a staff roll and
// a SPEC table, both of which live in structured columns already.
const work94Intro = `同人サークルNEKO WORKsの看板娘ショコラとバニラが豪華スタッフで同人ゲーム化しました。
E-moteシステムでキャラが動いたり、Hシーンもアニメーションしちゃいます!
タイトル:ネコぱら vol.1 ソレイユ開店しました!
ジャンル:ちょっとHでハートフルなネココメディ
原画:さより
シナリオ:雪仁
楽曲制作:水城新人
主題歌:nao/霜月はるか
デザイン:KOMEWORKS
SPEC
対応OS:Microsoft Windows Vista/7/8/8.1
解像度:1280×720ドット以上
CPU:Pentium4 1.8GHz 以上(推奨 Intel Core2Duo 2.0GHz 以上)
メモリ:1GB(推奨 2GB以上)
DirectX 9.0 に完全対応したビデオカード
VRAM:128MB以上 (推奨 256MB以上)
サウンド:DirectSound に正式対応したサウンドカード`

func TestStripsTheStoreListingFurniture(t *testing.T) {
	got := stripStructuredNoise(work94Intro)

	assert.Contains(t, got, "看板娘ショコラとバニラ", "the two real sentences survive")
	assert.Contains(t, got, "E-moteシステムで")
	for _, gone := range []string{"対応OS", "VRAM", "サウンド:", "原画:", "シナリオ:", "楽曲制作:", "主題歌:"} {
		assert.NotContains(t, got, gone)
	}
	assert.NotContains(t, got, "SPEC", "a section header whose whole body went must go with it")
	assert.Contains(t, got, "ジャンル:", "genre is a fact about the product, not a credit")
}

func TestKeepsTheCharacterBlockItLooksLike(t *testing.T) {
	// Work 481's shape. CV: and the stat lines read like labelled furniture and
	// are not: they sit inside the character block extract-char-intros reads.
	in := `CHARACTER
すみれ
CV:野中みかん
身長:158cm
体重:42kg
スリーサイズ:86(D)/59/87
忍の里の頭領の一人娘。忍者としての資質・実力・知識も十分だが、天然且つドジで失敗が多い。
父から修行を命じられ山奥にある忍者の里から街に出てきた。`

	got := stripStructuredNoise(in)
	assert.Equal(t, in, got, "nothing in a character block is furniture")
}

func TestDropsBareURLLinesButNotProseThatMentionsOne(t *testing.T) {
	in := `本編の続きが配信中です。主人公は異世界に転生し、そこで出会った少女たちとの日々を過ごすことになります。
https://www.dlsite.com/maniax/work/=/product_id/RJ01024197.html

詳しくは https://example.com/info をご覧ください。`

	got := stripStructuredNoise(in)
	assert.NotContains(t, got, "RJ01024197", "a line that is only a URL is a link, not prose")
	assert.Contains(t, got, "詳しくは https://example.com/info をご覧ください。",
		"a URL inside a sentence stays — removing it would break the sentence")
}

func TestAnIntroThatIsOnlyFurnitureIsLeftAlone(t *testing.T) {
	in := `SPEC
対応OS:Windows 10
CPU:Core i3
メモリ:4GB`

	assert.Equal(t, in, stripStructuredNoise(in),
		"stripping to nothing is worse than shipping the spec table")
}

func TestSanitizeRunsTheStripperAfterTheMarkup(t *testing.T) {
	in := `本作の紹介ページです。ここでは物語のあらすじと登場人物について説明しています。
[url=https://example.com/product]https://example.com/product[/url]
物語は主人公が異世界に転生するところから始まります。彼は世界最強のスキルを手に入れた。`

	got := sanitizeSource(in)
	assert.NotContains(t, got, "example.com",
		"the bbcode unwrap leaves a bare URL line, which the stripper must then see")
	assert.Contains(t, got, "異世界に転生する")
}

func TestStrippingChangesTheHashSoTheWorkRetranslatesOnItsOwn(t *testing.T) {
	clean := strings.Repeat("物語は静かに始まる。", 10)
	dirty := clean + "\n原画:さより\n対応OS:Windows 10"

	require.NotEqual(t, hashSource(sanitizeSource(dirty)), hashSource(dirty),
		"a stripped work must not keep hashing as the text it used to send")
	assert.Equal(t, hashSource(clean), hashSource(sanitizeSource(dirty)),
		"and it must land on the hash of what actually gets sent")
}
