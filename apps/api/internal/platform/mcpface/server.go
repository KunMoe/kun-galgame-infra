package mcpface

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName    = "nextmoe-catalog"
	serverVersion = "0.1.0-m1"
)

const instructions = "NextMoe catalog v2: read-only tools generated from the public OpenAPI document. " +
	"Each tool is one GET on /v2. Tool names are OpenAPI operationIds. Parameters match the HTTP " +
	"query and path parameters, including view= and fields=. " +
	"Send `Authorization: Bearer nmk_live_…` on the MCP endpoint for catalog reads that require a key; " +
	"mint a v2 key at " + devPortalURL + " (internal-tier apps during preview). " +
	"v1 nm_live_ keys are not accepted. News, problems, vocabularies, stats and schemas need no key. " +
	"R18 content is hidden by default: pass nsfw=true to include it. Any key may do so. " +
	"This surface is preview: paths and fields may still change."

type tools struct {
	up *Upstream
}

func NewServer(up *Upstream, spec []byte) (*mcp.Server, error) {
	s := mcp.NewServer(&mcp.Implementation{Name: serverName, Version: serverVersion},
		&mcp.ServerOptions{Instructions: instructions})
	if err := registerSpecTools(s, up, spec); err != nil {
		return nil, err
	}
	return s, nil
}

func (t *tools) run(ctx context.Context, req *mcp.CallToolRequest, tool, path string, query url.Values) (*mcp.CallToolResult, any, error) {
	var header http.Header
	if req != nil && req.Extra != nil {
		header = req.Extra.Header
	}
	token, ok := bearerToken(header)
	if !ok {
		return authError(), nil, nil
	}
	return (&toolsRunner{up: t.up}).run(ctx, req, tool, path, query, "Bearer "+token)
}

func newQuery() url.Values { return url.Values{} }

func setStr(q url.Values, key, val string) {
	if val != "" {
		q.Set(key, val)
	}
}

func setInt(q url.Values, key string, val int) {
	if val != 0 {
		q.Set(key, strconv.Itoa(val))
	}
}

func setBool(q url.Values, key string, val bool) {
	if val {
		q.Set(key, "1")
	}
}

func pathID(prefix string, id int) string {
	return prefix + "/" + strconv.Itoa(id)
}
