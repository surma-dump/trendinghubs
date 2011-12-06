package trendinghubs

import (
	"appengine"
	"appengine/urlfetch"
	"appengine/datastore"
	"html"
	"http"
	"json"
	"os"
	"strings"
	"template"
	"time"
)

const (
	MAX_AGE = 60 * 60 * 2 // 2 hours
)

func init() {
	http.Handle("/", http.RedirectHandler("http://surma.github.com/trendinghubs", http.StatusMovedPermanently))
	http.HandleFunc("/feed", generateFeed)
}

type Feed struct {
	Timestamp datastore.Time
	List      []Repository
}

type serialized_feed struct {
	Data []byte
}

func getCachedFeed(c appengine.Context) *Feed {

	fs := &serialized_feed{}
	key := datastore.NewKey(c, "Feed", "cache", 0, nil)
	e := datastore.Get(c, key, fs)
	if e != nil {
		return nil
	}

	feed := &Feed{}
	json.Unmarshal(fs.Data, feed)

	if time.Seconds()-feed.Timestamp.Time().Seconds() >= MAX_AGE {
		return nil
	}
	return feed
}

func GetFeed(c appengine.Context) *Feed {
	feed := getCachedFeed(c)
	if feed == nil {
		c.Debugf("GetFeed: Cache miss\n")
		client := urlfetch.Client(c)
		list, e := GetTrendingRepositories(client)
		if e != nil {
			return nil
		}

		feed = &Feed{
			Timestamp: datastore.SecondsToTime(time.Seconds()),
			List:      list,
		}
		data, _ := json.Marshal(feed)

		key := datastore.NewKey(c, "Feed", "cache", 0, nil)
		_, e = datastore.Put(c, key, &serialized_feed{data})
		if e != nil {
			c.Warningf("Could not cache: %s\n", e.String())
		}
	}
	return feed
}

func generateFeed(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	feed := GetFeed(c)
	if feed == nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/rss+xml")
	tpl := template.Must(template.New("feed").Parse(rawtemplate))
	tpl.Execute(w, feed)
}

var (
	rawtemplate = `<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0">
  <channel>
    <title>GitHub - Trending GitHub Repositories</title>
    <link>https://www.github.com</link>
    <description>The daily trending repositories</description>
    <language>en-US</language>
    {{range .List}}
    <item>
      <title>{{.User}} / {{.Name}}</title>
      <link>https://www.github.com/{{.User}}/{{.Name}}</link>
      <author>{{.User}}</author>
    </item>
	{{end}}
  </channel>
</rss>
	`
)

type Repository struct {
	User string
	Name string
}

func GetTrendingRepositories(client *http.Client) ([]Repository, os.Error) {
	resp, e := client.Get("https://www.github.com/explore")
	if e != nil {
		return nil, e
	}
	rootnode, e := html.Parse(resp.Body)
	if e != nil {
		return nil, e
	}

	list := make([]Repository, 0, 5)
	listnode := findNextNodeWithClass(rootnode, "ranked-repositories")
	for _, item := range listnode.Child {
		repo := findNextNodeWithTag(item, "h3")
		if repo != nil && !hasClass("last", item) {
			owner := repo.Child[1].Child[0].Data
			name := repo.Child[3].Child[0].Data
			list = append(list, Repository{
				User: owner,
				Name: name,
			})
		}
	}
	return list, nil

}

type QualFunc func(node *html.Node) bool

func findNextNode(start *html.Node, hasQualifier QualFunc) *html.Node {
	if hasQualifier(start) {
		return start
	}
	for _, child := range start.Child {
		subnode := findNextNode(child, hasQualifier)
		if subnode != nil {
			return subnode
		}
	}
	return nil
}

func findNextNodeWithClass(start *html.Node, classname string) *html.Node {
	return findNextNode(start, func(node *html.Node) bool {
		return hasClass(classname, node)
	})
}

func findNextNodeWithTag(start *html.Node, tagname string) *html.Node {
	return findNextNode(start, func(node *html.Node) bool {
		return node.Data == tagname
	})
}

func hasClass(classname string, node *html.Node) bool {
	for _, attr := range node.Attr {
		if attr.Key == "class" {
			classes := strings.Fields(attr.Val)
			for _, class := range classes {
				if class == classname {
					return true
				}
			}
		}
	}
	return false
}
