// Handling search: show page, deal with JSON requests.
// Also: provide a more clean API.
package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	kSearchPage         = "/search"
	kApiSearchFormatted = "/api/search-formatted"
	kApiSearch          = "/api/search"
)

type SearchHandler struct {
	store    StuffStore
	template *TemplateRenderer
	imgPath  string
}

func AddSearchHandler(store StuffStore, template *TemplateRenderer, imgPath string) {
	handler := &SearchHandler{
		store:    store,
		template: template,
		imgPath:  imgPath,
	}
	http.Handle(kSearchPage, handler)
	http.Handle("/", handler)
	http.Handle(kApiSearchFormatted, handler)
	http.Handle(kApiSearch, handler)
}

func (h *SearchHandler) ServeHTTP(out http.ResponseWriter, req *http.Request) {
	switch {
	case strings.HasPrefix(req.URL.Path, kApiSearchFormatted):
		h.apiSearchPageItem(out, req)
	case strings.HasPrefix(req.URL.Path, kApiSearch):
		h.apiSearch(out, req)
	default:
		h.showSearchPage(out, req)
	}
}

func (h *SearchHandler) showSearchPage(out http.ResponseWriter, r *http.Request) {
	out.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Just static html. Maybe serve from /static ?
	content, _ := ioutil.ReadFile(h.template.baseDir + "/search-result.html")
	out.Write(content)
}

type JsonComponent struct {
	Component
	Image string `json:"img"`
}
type JsonApiSearchResult struct {
	Directlink string          `json:"link"`
	Items      []JsonComponent `json:"components"`
}

func encodeUriComponent(str string) string {
	u, err := url.Parse(str)
	if err != nil {
		return ""
	}
	return u.String()
}
func (h *SearchHandler) apiSearch(out http.ResponseWriter, r *http.Request) {
	// Allow very brief caching, so that editing the query does not
	// necessarily has to trigger a new server roundtrip.
	out.Header().Set("Cache-Control", "max-age=10")
	out.Header().Set("Content-Type", "application/json")
	defaultOutLen := 20
	maxOutLen := 100 // Limit max output
	query := r.FormValue("q")
	limit, _ := strconv.Atoi(r.FormValue("count"))
	if limit <= 0 {
		limit = defaultOutLen
	}
	if limit > maxOutLen {
		limit = maxOutLen
	}
	var searchResults []*Component
	if query != "" {
		searchResults = h.store.Search(query)
	}
	outlen := limit
	if len(searchResults) < limit {
		outlen = len(searchResults)
	}
	jsonResult := &JsonApiSearchResult{
		Directlink: encodeUriComponent("/search#" + query),
		Items:      make([]JsonComponent, outlen),
	}

	for i := 0; i < outlen; i++ {
		var c = searchResults[i]
		jsonResult.Items[i].Component = *c
		jsonResult.Items[i].Image = fmt.Sprintf("/img/%d", c.Id)
	}

	json, _ := json.MarshalIndent(jsonResult, "", "  ")
	out.Write(json)
}

// Pre-formatted search for quick div replacements.
type JsonHtmlSearchResultRecord struct {
	Id    int    `json:"id"`
	Label string `json:"txt"`
}

type JsonHtmlSearchResult struct {
	Count      int                          `json:"count"`
	QueryInfo  string                       `json:"queryinfo"`
	ResultInfo string                       `json:"resultinfo"`
	Items      []JsonHtmlSearchResultRecord `json:"items"`
}

func (h *SearchHandler) apiSearchPageItem(out http.ResponseWriter, r *http.Request) {
	defer ElapsedPrint("Query", time.Now())
	// Allow very brief caching, so that editing the query does not
	// necessarily has to trigger a new server roundtrip.
	out.Header().Set("Cache-Control", "max-age=10")
	query := r.FormValue("q")
	if query == "" {
		out.Write([]byte(`{"count":0, "queryinfo":"", "resultinfo":"", "items":[]}`))
		return
	}
	start := time.Now()
	searchResults := h.store.Search(query)
	elapsed := time.Now().Sub(start)
	elapsed = time.Microsecond * ((elapsed + time.Microsecond/2) / time.Microsecond)

	// We only want to output a query info if it actually has been
	// rewritten.
	queryInfo := ""
	rewrittenQuery := queryRewrite(query)
	if rewrittenQuery != query {
		queryInfo = rewrittenQuery
	}

	outlen := 24 // Limit max output
	if len(searchResults) < outlen {
		outlen = len(searchResults)
	}
	jsonResult := &JsonHtmlSearchResult{
		Count:      len(searchResults),
		ResultInfo: fmt.Sprintf("%d results (%s)", len(searchResults), elapsed),
		QueryInfo:  queryInfo,
		Items:      make([]JsonHtmlSearchResultRecord, outlen),
	}

	for i := 0; i < outlen; i++ {
		var c = searchResults[i]
		jsonResult.Items[i].Id = c.Id
		jsonResult.Items[i].Label = "<b>" + html.EscapeString(c.Value) + "</b> " +
			html.EscapeString(c.Description) +
			fmt.Sprintf(" <span class='idtxt'>(ID:%d)</span>", c.Id)
	}
	json, _ := json.Marshal(jsonResult)
	out.Write(json)
}
