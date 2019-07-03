package uadmin

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// TestMainHandler is a unit testing function for mainHandler() function
func TestMainHandler(t *testing.T) {
	allowed := AllowedIPs
	blocked := BlockedIPs

	s1 := &Session{
		UserID: 1,
		Active: true,
	}
	s1.GenerateKey()
	s1.Save()

	u1 := &User{
		Username: "u1",
		Password: "u1",
		Active:   true,
	}
	u1.Save()

	s2 := &Session{
		UserID: u1.ID,
		Active: true,
	}
	s2.GenerateKey()
	s2.Save()

	examples := []struct {
		r       *http.Request
		ip      string
		allowed string
		blocked string
		session *Session
		code    int
		title   string
		err_msg string
	}{
		{httptest.NewRequest("GET", "http://0.0.0.0:5000/", nil), "", "", "", nil, 200, "uAdmin - Login", ""},
		{httptest.NewRequest("GET", "http://0.0.0.0:5000/", nil), "10.0.0.1", "10.0.0.0/24", "", nil, 200, "uAdmin - Login", ""},
		{httptest.NewRequest("GET", "http://0.0.0.0:5000/", nil), "10.0.0.1", "10.0.1.0/24", "", nil, 404, "uAdmin - 404", ""},
		{httptest.NewRequest("GET", "http://0.0.0.0:5000/resetpassword", nil), "", "", "", nil, 404, "uAdmin - 404", ""},
		{httptest.NewRequest("GET", "http://0.0.0.0:5000/", nil), "1.1.1.1", "", "", s2, 404, "uAdmin - 404", "Remote Access Denied"},
		{httptest.NewRequest("GET", "http://0.0.0.0:5000/", nil), "10.0.0.1", "", "", s2, 200, "uAdmin - Dashboard", ""},
	}

	for i, e := range examples {
		w := httptest.NewRecorder()
		if e.session != nil {
			e.r.AddCookie(&http.Cookie{Name: "session", Value: e.session.Key})
		}
		if e.ip != "" {
			e.r.RemoteAddr = e.ip + ":1234"
		}
		if e.allowed == "" {
			AllowedIPs = allowed
			BlockedIPs = blocked
		} else {
			AllowedIPs = e.allowed
			BlockedIPs = e.blocked
		}
		mainHandler(w, e.r)

		if w.Code != e.code {
			t.Errorf("mainHandler returned invalid code on example %d. Requesting %s. got %d, expected %d", i, e.r.URL.Path, w.Code, e.code)
			buf, _ := ioutil.ReadAll(w.Body)
			Trail(DEBUG, string(buf))
		}

		doc, _ := parseHTML(w.Result().Body, t)
		_, content, _ := tagSearch(doc, "title", "", 0)

		//title, _, _ := getHTMLTag(w.Result().Body, "title")
		if content[0] != e.title {
			t.Errorf("mainHandler returned invalid title on example %d. Requesting %s. got %s, expected %s", i, e.r.URL.Path, content[0], e.title)
		}

		if e.err_msg != "" {
			_, content, _ = tagSearch(doc, "h3", "", 0)
			//err_msg, _, _ := getHTMLTag(w.Result().Body, "h3")
			if content[0] != e.err_msg {
				t.Errorf("mainHandler returned invalid error message on example %d. Requesting %s. got %s, expected %s", i, e.r.URL.Path, content[0], e.err_msg)
			}
		}
	}

	// Clean up
	AllowedIPs = allowed
	BlockedIPs = blocked

	Delete(s1)
	Delete(s2)
	Delete(u1)
}

func traverse(n *html.Node, tag string) (string, map[string]string, bool) {
	if isTagElement(n, tag) {
		tempMap := map[string]string{}
		for i := range n.Attr {
			tempMap[n.Attr[i].Key] = n.Attr[i].Val
		}
		if n.FirstChild == nil {
			return "", tempMap, true
		}
		return strings.TrimSpace(n.FirstChild.Data), tempMap, true
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		result, attr, ok := traverse(c, tag)
		if ok {
			return result, attr, ok
		}
	}

	return "", map[string]string{}, false
}

func getHTMLTag(r io.Reader, tag string) (string, map[string]string, bool) {
	doc, err := html.Parse(r)
	if err != nil {
		Trail(ERROR, "Fail to parse html")
		return "", map[string]string{}, false
	}

	return traverse(doc, tag)
}

func isTagElement(n *html.Node, tag string) bool {
	return n.Type == html.ElementNode && n.Data == tag
}

func tagSearch(n *html.Node, tag string, path string, index int) ([]string, []string, []map[string]string) {
	paths := []string{}
	content := []string{}
	attr := []map[string]string{}

	if path == "" {
		if n.Data != "" {
			path = fmt.Sprintf("%s[%d]", n.Data, index)
		}
	} else {
		path = path + "/" + fmt.Sprintf("%s[%d]", n.Data, index)
	}

	if isTagElement(n, tag) {
		if n.FirstChild == nil {
			content = append(content, "")
		} else {
			content = append(content, strings.TrimSpace(n.FirstChild.Data))
		}
		paths = append(paths, path)
		tempMap := map[string]string{}
		for i := range n.Attr {
			tempMap[n.Attr[i].Key] = n.Attr[i].Val
		}
		attr = append(attr, tempMap)
	}

	index = 0
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		childPaths, childContent, childAttr := tagSearch(c, tag, path, index)
		paths = append(paths, childPaths...)
		content = append(content, childContent...)
		attr = append(attr, childAttr...)
		if c.Type == html.ElementNode {
			index++
		}
	}
	return paths, content, attr
}

func getHTMLTagList(r io.Reader, tag string) (paths []string, content []string, attr []map[string]string) {
	doc, err := html.Parse(r)
	if err != nil {
		Trail(ERROR, "Failed to parse html")
		return
	}
	return tagSearch(doc, tag, "", 0)
}

func parseHTML(r io.Reader, t *testing.T) (*html.Node, error) {
	doc, err := html.Parse(r)
	if err != nil {
		t.Errorf("Unable to parse html stream")
	}
	return doc, err
}
