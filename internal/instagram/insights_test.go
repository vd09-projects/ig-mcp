package instagram

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vikrant/instagram-mcp/internal/graphapi"
)

// mediaServer serves pages of fake media. Each page is keyed by its "after"
// cursor ("" for the first page). A page with an empty next cursor is the last.
type mediaPage struct {
	ids  []string
	next string
}

func newMediaServer(t *testing.T, pages map[string]mediaPage, calls *[]string) *graphClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/acct/media" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		after := r.URL.Query().Get("after")
		*calls = append(*calls, after+"|"+r.URL.Query().Get("limit"))

		page, ok := pages[after]
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":{"message":"bad cursor","type":"OAuthException","code":100}}`)
			return
		}

		items := make([]string, len(page.ids))
		for i, id := range page.ids {
			items[i] = fmt.Sprintf(`{"id":%q,"media_product_type":"REELS","like_count":1}`, id)
		}
		paging := `{"cursors":{"after":""}}`
		if page.next != "" {
			paging = fmt.Sprintf(`{"cursors":{"after":%q},"next":"https://example/next"}`, page.next)
		}
		fmt.Fprintf(w, `{"data":[%s],"paging":%s}`, strings.Join(items, ","), paging)
	}))
	t.Cleanup(srv.Close)

	return &graphClient{api: graphapi.NewClient(srv.URL, "token"), accountID: "acct"}
}

func ids(items []MediaItem) string {
	out := make([]string, len(items))
	for i, m := range items {
		out[i] = m.ID
	}
	return strings.Join(out, ",")
}

func TestListRecentMediaFollowsPagination(t *testing.T) {
	var calls []string
	c := newMediaServer(t, map[string]mediaPage{
		"":   {ids: []string{"a", "b"}, next: "c1"},
		"c1": {ids: []string{"c", "d"}, next: "c2"},
		"c2": {ids: []string{"e"}},
	}, &calls)

	items, err := c.ListRecentMedia(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(items); got != "a,b,c,d,e" {
		t.Errorf("expected a,b,c,d,e, got %s", got)
	}
	if !items[0].IsReel() || items[0].LikeCount != 1 {
		t.Errorf("expected parsed reel fields, got %+v", items[0])
	}
	if len(calls) != 3 {
		t.Errorf("expected 3 calls (stop on last page), got %v", calls)
	}
}

func TestListRecentMediaTruncatesToLimit(t *testing.T) {
	var calls []string
	c := newMediaServer(t, map[string]mediaPage{
		"":   {ids: []string{"a", "b"}, next: "c1"},
		"c1": {ids: []string{"c", "d"}, next: "c2"},
	}, &calls)

	items, err := c.ListRecentMedia(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(items); got != "a,b,c" {
		t.Errorf("expected a,b,c, got %s", got)
	}
	// Second request asks only for the remaining item.
	if len(calls) != 2 || calls[0] != "|3" || calls[1] != "c1|1" {
		t.Errorf("unexpected calls %v", calls)
	}
}

func TestListRecentMediaPageError(t *testing.T) {
	var calls []string
	c := newMediaServer(t, map[string]mediaPage{
		"": {ids: []string{"a"}, next: "missing"},
	}, &calls)

	if _, err := c.ListRecentMedia(context.Background(), 10); err == nil || !strings.Contains(err.Error(), "bad cursor") {
		t.Fatalf("expected page error, got %v", err)
	}
}
